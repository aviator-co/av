package main

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/treedetector"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/aviator-co/av/internal/utils/uiutils"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/shurcooL/githubv4"
	"github.com/spf13/cobra"
)

var adoptFlags struct {
	Parent           string
	DryRun           bool
	RemoteBranchName string
	Yes              bool
}

var adoptCmd = &cobra.Command{
	Use:   "adopt",
	Short: "Adopt branches that are not managed by av",
	Long: strings.TrimSpace(`
Adopt branches that are not managed by av.

This command will show a list of branches that are not managed by av. You can choose which branches
should be adopted to av.

If you want to adopt the current branch, you can use the --parent flag to specify the parent branch.
For example, "av adopt --parent main" will adopt the current branch with the main branch as
the parent.

If you want to adopt branches on the remote repository, use --remote $BRANCH_NAME to adopt branches.
The command will adopt the stack of pull requests starting from the specified branch name.

By default --remote opens an interactive picker. Pass --yes to skip the picker and adopt what it
selects by default: the named branch and each open, not-yet-adopted ancestor PR found by following
av's PR metadata. Merged or closed ancestors are skipped and their child is reparented onto the next
open ancestor or the trunk. With --yes, the command fails if the named branch's pull request is not
open, or if a pull request in the stack has no av metadata while its base branch has its own open
pull request. After adopting, av switches to the top
adopted branch when the working tree allows it.
`),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		if adoptFlags.Yes && adoptFlags.RemoteBranchName == "" {
			return errors.New("--yes can only be used together with --remote")
		}
		repo, err := getRepo(ctx)
		if err != nil {
			return err
		}

		db, err := getDB(ctx, repo)
		if err != nil {
			return err
		}

		status, err := repo.Status(ctx)
		if err != nil {
			return err
		}

		currentBranch := status.CurrentBranch
		if adoptFlags.Parent != "" {
			return adoptForceAdoption(ctx, repo, db, currentBranch, adoptFlags.Parent)
		}
		if adoptFlags.RemoteBranchName != "" {
			client, err := getGitHubClient(ctx)
			if err != nil {
				return err
			}
			return uiutils.RunBubbleTeaWithContext(ctx, &remoteAdoptViewModel{
				ctx:            func() context.Context { return ctx },
				repo:           repo,
				db:             db,
				ghClient:       client,
				branchName:     adoptFlags.RemoteBranchName,
				nonInteractive: adoptFlags.Yes,
			})
		}
		return uiutils.RunBubbleTeaWithContext(ctx, &adoptViewModel{
			ctx:               func() context.Context { return ctx },
			repo:              repo,
			db:                db,
			currentHEADBranch: plumbing.NewBranchReferenceName(currentBranch),
		})
	},
}

func adoptForceAdoption(
	ctx context.Context,
	repo *git.Repo,
	db meta.DB,
	currentBranch, parent string,
) error {
	if currentBranch == "" {
		return errors.New("the current repository state is at a detached HEAD")
	}

	tx := db.WriteTx()
	branch, exists := tx.Branch(currentBranch)
	if exists {
		return errors.New("branch is already adopted")
	}

	parent = stripRemoteRefPrefixes(repo, parent)
	if currentBranch == parent {
		return errors.New("cannot adopt the current branch as its parent")
	}

	if repo.IsTrunkBranch(currentBranch) {
		return errors.New("cannot adopt the default branch")
	}

	isParentBranchTrunk := repo.IsTrunkBranch(parent)
	if isParentBranchTrunk {
		branch.Parent = meta.BranchState{
			Name:  parent,
			Trunk: true,
		}
	} else {
		_, exist := tx.Branch(parent)
		if !exist {
			return errors.New("parent branch is not adopted yet")
		}
		mergeBase, err := repo.MergeBase(ctx, parent, currentBranch)
		if err != nil {
			return err
		}
		branch.Parent = meta.BranchState{
			Name:                     parent,
			Trunk:                    false,
			BranchingPointCommitHash: mergeBase,
		}
	}

	if err := meta.ValidateNoCycle(tx, currentBranch, branch.Parent); err != nil {
		return err
	}
	tx.SetBranch(branch)
	if adoptFlags.DryRun {
		return nil
	}
	return tx.Commit()
}

type adoptViewModel struct {
	ctx               func() context.Context
	repo              *git.Repo
	db                meta.DB
	currentHEADBranch plumbing.ReferenceName
	branches          map[plumbing.ReferenceName]*treedetector.BranchPiece

	uiutils.BaseStackedView
}

func (vm *adoptViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return vm, vm.BaseStackedView.Update(msg)
}

func (vm *adoptViewModel) Init() tea.Cmd {
	return vm.initModel()
}

func (vm *adoptViewModel) initModel() tea.Cmd {
	return vm.AddView(
		actions.NewFindAdoptableLocalBranchesModel(
			vm.ctx(),
			vm.repo,
			vm.db,
			vm.initTreeSelector,
		),
	)
}

func (vm *adoptViewModel) initTreeSelector(branches map[plumbing.ReferenceName]*treedetector.BranchPiece, rootNodes []*stackutils.StackTreeNode, adoptionTargets []plumbing.ReferenceName) tea.Cmd {
	if branches == nil || len(rootNodes) == 0 || len(adoptionTargets) == 0 {
		return tea.Batch(
			vm.AddView(uiutils.SimpleMessageView{Message: colors.SuccessStyle.Render("✓ No branch to adopt")}),
			tea.Quit,
		)
	}
	vm.branches = branches
	infos := make(map[plumbing.ReferenceName]actions.BranchTreeInfo)
	for _, branch := range adoptionTargets {
		titleLine := branch.Short()
		var status []string
		if vm.currentHEADBranch == branch {
			status = append(status, "HEAD")
		}
		if len(status) != 0 {
			titleLine += " (" + strings.Join(status, ", ") + ")"
		}

		_, adopted := vm.db.ReadTx().Branch(branch.Short())
		body := ""
		if !adopted {
			piece := branches[branch]
			var lines []string
			for _, c := range piece.IncludedCommits {
				prTitle, _, _ := strings.Cut(c.Message, "\n")
				lines = append(lines, "  "+prTitle)
			}
			body = strings.Join(lines, "\n")
		}
		infos[branch] = actions.BranchTreeInfo{
			TitleLine: titleLine,
			Body:      body,
		}
	}
	cmd := vm.AddView(
		actions.NewAdoptTreeSelectorModel(
			infos,
			rootNodes,
			adoptionTargets,
			vm.currentHEADBranch,
			vm.initAdoption,
		),
	)
	if adoptFlags.DryRun {
		return tea.Batch(
			cmd,
			vm.AddView(uiutils.SimpleMessageView{Message: colors.SuccessStyle.Render("✓ Running as dry-run. Quitting without adopting branches.")}),
			tea.Quit,
		)
	}
	return cmd
}

func (vm *adoptViewModel) initAdoption(chosenTargets []plumbing.ReferenceName) tea.Cmd {
	if len(chosenTargets) == 0 {
		return tea.Batch(
			vm.AddView(uiutils.SimpleMessageView{Message: colors.SuccessStyle.Render("✓ No branch adopted")}),
			tea.Quit,
		)
	}
	var branches []actions.AdoptingBranch
	for _, target := range chosenTargets {
		piece := vm.branches[target]
		ab := actions.AdoptingBranch{
			Name: target.Short(),
			Parent: meta.BranchState{
				Name:  piece.Parent.Short(),
				Trunk: piece.ParentIsTrunk,
			},
		}
		if !piece.ParentIsTrunk {
			ab.Parent.BranchingPointCommitHash = piece.ParentMergeBase.String()
		}
		branches = append(branches, ab)
	}
	return vm.AddView(
		actions.NewAdoptBranchesModel(
			vm.ctx(),
			vm.db,
			branches,
			func() tea.Cmd { return tea.Quit },
		),
	)
}

func (vm *adoptViewModel) ExitError() error {
	if vm.Err != nil {
		return actions.ErrExitSilently{ExitCode: 1}
	}
	return nil
}

type remoteAdoptViewModel struct {
	ctx            func() context.Context
	repo           *git.Repo
	db             meta.DB
	ghClient       *gh.Client
	branchName     string
	nonInteractive bool

	uiutils.BaseStackedView
}

func (vm *remoteAdoptViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return vm, vm.BaseStackedView.Update(msg)
}

func (vm *remoteAdoptViewModel) Init() tea.Cmd {
	return vm.initModel()
}

func (vm *remoteAdoptViewModel) initModel() tea.Cmd {
	return vm.AddView(
		actions.NewGetRemoteStackedPRModel(
			vm.ctx(),
			vm.db.ReadTx().Repository(),
			vm.ghClient,
			vm.branchName,
			vm.initTreeSelector,
		),
	)
}

func (vm *remoteAdoptViewModel) initTreeSelector(prs []actions.RemotePRInfo) tea.Cmd {
	var adoptionTargets []plumbing.ReferenceName
	infos := make(map[plumbing.ReferenceName]actions.BranchTreeInfo)
	for _, prInfo := range prs {
		branch := plumbing.NewBranchReferenceName(prInfo.Name)
		if prInfo.PullRequest.State == githubv4.PullRequestStateOpen {
			// Check if the branch is already adopted.
			if _, ok := vm.db.ReadTx().Branch(prInfo.Name); !ok {
				adoptionTargets = append(adoptionTargets, branch)
			}
		}
		infos[branch] = actions.BranchTreeInfo{
			TitleLine: prInfo.Title,
			Body:      prInfo.PullRequest.Permalink,
		}
	}
	if vm.nonInteractive {
		if prs[0].PullRequest.State != githubv4.PullRequestStateOpen {
			return errorCmd(errors.Errorf(
				"pull request for %s is %s; --yes only adopts open pull requests",
				prs[0].Name, strings.ToLower(string(prs[0].PullRequest.State)),
			))
		}
		if last := prs[len(prs)-1]; last.UnlinkedParentPull != 0 {
			return errorCmd(errors.Errorf(
				"pull request #%d for %s has no av metadata, so --yes can't follow it to %s (#%d)",
				last.PullRequest.Number, last.Name, last.Parent.Name, last.UnlinkedParentPull,
			))
		}
	}
	if len(adoptionTargets) == 0 {
		return tea.Batch(
			vm.AddView(uiutils.SimpleMessageView{Message: colors.SuccessStyle.Render("✓ No branch to adopt")}),
			tea.Quit,
		)
	}
	if vm.nonInteractive {
		if adoptFlags.DryRun {
			branches, err := remoteAdoptingBranches(prs, adoptionTargets)
			if err != nil {
				return errorCmd(err)
			}
			lines := []string{"Would adopt:"}
			for _, ab := range branches {
				lines = append(lines, fmt.Sprintf("  %s (parent: %s)", ab.Name, ab.Parent.Name))
			}
			return tea.Batch(
				vm.AddView(uiutils.SimpleMessageView{Message: colors.SuccessStyle.Render(strings.Join(lines, "\n"))}),
				tea.Quit,
			)
		}
		return vm.initGitFetch(prs, adoptionTargets)
	}
	var lastNode *stackutils.StackTreeNode
	for _, prInfo := range prs {
		node := &stackutils.StackTreeNode{
			Branch: &stackutils.StackTreeBranchInfo{
				BranchName:       prInfo.Name,
				ParentBranchName: prInfo.Parent.Name,
			},
		}
		if lastNode != nil {
			node.Children = []*stackutils.StackTreeNode{lastNode}
		}
		lastNode = node
	}
	lastNode = &stackutils.StackTreeNode{
		Branch: &stackutils.StackTreeBranchInfo{
			BranchName:       prs[len(prs)-1].Parent.Name,
			ParentBranchName: "",
		},
		Children: []*stackutils.StackTreeNode{lastNode},
	}

	cmd := vm.AddView(
		actions.NewAdoptTreeSelectorModel(
			infos,
			[]*stackutils.StackTreeNode{lastNode},
			adoptionTargets,
			"",
			func(chosenTargets []plumbing.ReferenceName) tea.Cmd {
				return vm.initGitFetch(prs, chosenTargets)
			},
		),
	)
	if adoptFlags.DryRun {
		return tea.Batch(
			cmd,
			vm.AddView(uiutils.SimpleMessageView{Message: colors.SuccessStyle.Render("✓ Running as dry-run. Quitting without adopting branches.")}),
			tea.Quit,
		)
	}
	return cmd
}

func (vm *remoteAdoptViewModel) initGitFetch(prs []actions.RemotePRInfo, chosenTargets []plumbing.ReferenceName) tea.Cmd {
	if len(chosenTargets) == 0 {
		return tea.Batch(
			vm.AddView(uiutils.SimpleMessageView{Message: colors.SuccessStyle.Render("✓ No branch adopted")}),
			tea.Quit,
		)
	}
	branches, err := remoteAdoptingBranches(prs, chosenTargets)
	if err != nil {
		return errorCmd(err)
	}
	remote := vm.repo.GetRemoteName()
	refspecs := []string{}
	for _, ab := range branches {
		// Clone as a local branch, and create the remote-tracking ref so we can
		// set upstream tracking after adoption.
		refspecs = append(refspecs, fmt.Sprintf("refs/heads/%s:refs/heads/%s", ab.Name, ab.Name))
		refspecs = append(refspecs, fmt.Sprintf("refs/heads/%s:refs/remotes/%s/%s", ab.Name, remote, ab.Name))
	}
	return vm.AddView(
		actions.NewGitFetchModel(vm.ctx(), vm.repo, refspecs, func() tea.Cmd {
			return vm.initAdoption(branches)
		}),
	)
}

func errorCmd(err error) tea.Cmd {
	return func() tea.Msg { return err }
}

func remoteAdoptingBranches(prs []actions.RemotePRInfo, chosenTargets []plumbing.ReferenceName) ([]actions.AdoptingBranch, error) {
	chosenSet := make(map[string]bool)
	for _, target := range chosenTargets {
		chosenSet[target.Short()] = true
	}
	prMap := make(map[string]actions.RemotePRInfo)
	for _, prInfo := range prs {
		prMap[prInfo.Name] = prInfo
	}
	var branches []actions.AdoptingBranch
	for _, pr := range slices.Backward(prs) {
		if !chosenSet[pr.Name] {
			continue
		}
		// If the parent is already merged / closed, change the parent to the trunk branch.
		for !pr.Parent.Trunk {
			parentPRInfo, ok := prMap[pr.Parent.Name]
			if !ok {
				return nil, errors.Errorf("cannot find the pull request for %s, the parent of %s", pr.Parent.Name, pr.Name)
			}
			if parentPRInfo.PullRequest.State == githubv4.PullRequestStateOpen {
				break
			}
			if parentPRInfo.PullRequest.State == githubv4.PullRequestStateMerged {
				pr.Parent.Name = parentPRInfo.Parent.Name
				pr.Parent.Trunk = parentPRInfo.Parent.Trunk
			} else {
				pr.Parent = parentPRInfo.Parent
			}
		}
		branches = append(branches, actions.AdoptingBranch{
			Name:        pr.Name,
			Parent:      pr.Parent,
			PullRequest: &pr.PullRequest,
		})
	}
	return branches, nil
}

func (vm *remoteAdoptViewModel) initAdoption(branches []actions.AdoptingBranch) tea.Cmd {
	return vm.AddView(
		actions.NewAdoptBranchesModel(
			vm.ctx(),
			vm.db,
			branches,
			func() tea.Cmd {
				remote := vm.repo.GetRemoteName()
				hasChild := make(map[string]bool)
				for _, ab := range branches {
					hasChild[ab.Parent.Name] = true
					_ = vm.repo.BranchSetUpstream(vm.ctx(), ab.Name, remote)
				}
				for _, ab := range branches {
					if !hasChild[ab.Name] {
						// Check out the leaf branch. This can fail if the workspace is dirty. In that case, just quietly exit.
						_, _ = vm.repo.Git(vm.ctx(), "switch", ab.Name, "--quiet")
						break
					}
				}
				return tea.Quit
			},
		),
	)
}

func (vm *remoteAdoptViewModel) ExitError() error {
	if vm.Err != nil {
		return actions.ErrExitSilently{ExitCode: 1}
	}
	return nil
}

func init() {
	adoptCmd.Flags().StringVar(
		&adoptFlags.Parent, "parent", "",
		"force specifying the parent branch",
	)
	adoptCmd.Flags().BoolVar(
		&adoptFlags.DryRun, "dry-run", false,
		"dry-run adoption",
	)
	adoptCmd.Flags().StringVar(
		&adoptFlags.RemoteBranchName, "remote", "",
		"adopt branches from remote pull requests, starting from the specified branch",
	)
	adoptCmd.Flags().BoolVar(
		&adoptFlags.Yes, "yes", false,
		"with --remote, adopt the named branch and its open ancestors without prompting",
	)
	adoptCmd.MarkFlagsMutuallyExclusive("parent", "remote")

	_ = adoptCmd.RegisterFlagCompletionFunc(
		"parent",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			branches, _ := allBranches(cmd.Context())
			return branches, cobra.ShellCompDirectiveNoFileComp
		},
	)
}
