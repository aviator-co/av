package main

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/treedetector"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/aviator-co/av/internal/utils/uiutils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
)

var adoptFlags struct {
	Parent           string
	DryRun           bool
	RemoteBranchName string
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
`),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
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
			return uiutils.RunBubbleTea(&remoteAdoptViewModel{
				repo:       repo,
				db:         db,
				ghClient:   client,
				branchName: adoptFlags.RemoteBranchName,
			})
		}
		return uiutils.RunBubbleTea(&adoptViewModel{
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

	if isCurrentBranchTrunk, err := repo.IsTrunkBranch(ctx, currentBranch); err != nil {
		return errors.Wrap(err, "failed to check if the current branch is trunk")
	} else if isCurrentBranchTrunk {
		return errors.New("cannot adopt the default branch")
	}

	isParentBranchTrunk, err := repo.IsTrunkBranch(ctx, parent)
	if err != nil {
		return errors.Wrap(err, "failed to check if the parent branch is trunk")
	}
	if isParentBranchTrunk {
		branch.Parent = meta.BranchState{
			Name:  parent,
			Trunk: true,
		}
		tx.SetBranch(branch)
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
			Name:  parent,
			Trunk: false,
			Head:  mergeBase,
		}
		tx.SetBranch(branch)
	}
	if adoptFlags.DryRun {
		return nil
	}
	return tx.Commit()
}

type adoptViewModel struct {
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
			ab.Parent.Head = piece.ParentMergeBase.String()
		}
		branches = append(branches, ab)
	}
	return vm.AddView(
		actions.NewAdoptBranchesModel(
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
	repo       *git.Repo
	db         meta.DB
	ghClient   *gh.Client
	branchName string

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
			vm.db.ReadTx().Repository(),
			vm.ghClient,
			vm.branchName,
			vm.initTreeSelector,
		),
	)
}

func (vm *remoteAdoptViewModel) initTreeSelector(prs []actions.RemotePRInfo) tea.Cmd {
	if len(prs) == 0 {
		return tea.Batch(
			vm.AddView(uiutils.SimpleMessageView{Message: colors.SuccessStyle.Render("✓ No branch to adopt")}),
			tea.Quit,
		)
	}
	var adoptionTargets []plumbing.ReferenceName
	infos := make(map[plumbing.ReferenceName]actions.BranchTreeInfo)
	for _, prInfo := range prs {
		branch := plumbing.NewBranchReferenceName(prInfo.Name)
		adoptionTargets = append(adoptionTargets, branch)
		infos[branch] = actions.BranchTreeInfo{
			TitleLine: prInfo.Title,
			Body:      prInfo.PullRequest.Permalink,
		}
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
	refspecs := []string{}
	for _, target := range chosenTargets {
		// Directly clone as a local branch.
		refspecs = append(refspecs, fmt.Sprintf("refs/heads/%s:refs/heads/%s", target.Short(), target.Short()))
	}
	return vm.AddView(
		actions.NewGitFetchModel(vm.repo, refspecs, func() tea.Cmd {
			return vm.initAdoption(prs, chosenTargets)
		}),
	)
}

func (vm *remoteAdoptViewModel) initAdoption(prs []actions.RemotePRInfo, chosenTargets []plumbing.ReferenceName) tea.Cmd {
	var branches []actions.AdoptingBranch
	for _, target := range chosenTargets {
		idx := slices.IndexFunc(prs, func(prInfo actions.RemotePRInfo) bool {
			return prInfo.Name == target.Short()
		})
		if idx == -1 {
			return uiutils.ErrCmd(fmt.Errorf("internal error: failed to find PR info for branch %s", target.Short()))
		}
		pr := prs[idx]
		ab := actions.AdoptingBranch{
			Name:        target.Short(),
			Parent:      pr.Parent,
			PullRequest: &pr.PullRequest,
		}
		branches = append(branches, ab)
	}
	return vm.AddView(
		actions.NewAdoptBranchesModel(
			vm.db,
			branches,
			func() tea.Cmd { return tea.Quit },
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

	_ = adoptCmd.RegisterFlagCompletionFunc(
		"parent",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			branches, _ := allBranches(cmd.Context())
			return branches, cobra.ShellCompDirectiveNoFileComp
		},
	)
}
