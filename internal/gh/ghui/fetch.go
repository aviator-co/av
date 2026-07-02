package ghui

import (
	"context"
	"maps"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
)

func NewGitHubFetchModel(
	repo *git.Repo,
	db meta.DB,
	client *gh.Client,
	currentBranch plumbing.ReferenceName,
	targetBranches []plumbing.ReferenceName,
	onDone func() tea.Cmd,
) *GitHubFetchModel {
	return &GitHubFetchModel{
		repo:           repo,
		db:             db,
		client:         client,
		currentBranch:  currentBranch,
		targetBranches: targetBranches,
		spinner:        spinner.New(spinner.WithSpinner(spinner.Dot)),
		onDone:         onDone,

		runningGitFetch:             true,
		runningGitHubAPIBranch:      -1,
		runningCheckCommitHistory:   false,
		runningPropagateMergeCommit: false,
	}
}

type GitHubFetchProgress struct {
	gitFetchIsDone               bool
	apiFetchIsDone               bool
	checkCommitHistoryIsDone     bool
	mergeCommitPropagationIsDone bool
}

type GitHubFetchModel struct {
	repo           *git.Repo
	db             meta.DB
	client         *gh.Client
	currentBranch  plumbing.ReferenceName
	targetBranches []plumbing.ReferenceName
	spinner        spinner.Model
	onDone         func() tea.Cmd

	runningGitFetch             bool
	runningGitHubAPIBranch      int
	runningCheckCommitHistory   bool
	runningPropagateMergeCommit bool
}

func (vm *GitHubFetchModel) Init() tea.Cmd {
	return tea.Batch(vm.spinner.Tick, vm.runGitFetch)
}

func (vm *GitHubFetchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case *GitHubFetchProgress:
		if msg.gitFetchIsDone {
			vm.runningGitFetch = false
			vm.runningGitHubAPIBranch = 0
			return vm, vm.runGitHubAPIFetch
		}
		if msg.apiFetchIsDone {
			vm.runningGitHubAPIBranch++
			if len(vm.targetBranches) <= vm.runningGitHubAPIBranch {
				vm.runningCheckCommitHistory = true
				return vm, vm.updateMergeCommitsFromCommitMessage
			}
			return vm, vm.runGitHubAPIFetch
		}
		if msg.checkCommitHistoryIsDone {
			vm.runningCheckCommitHistory = false
			vm.runningPropagateMergeCommit = true
			return vm, vm.updateMergeCommitsFromChildren
		}
		if msg.mergeCommitPropagationIsDone {
			vm.runningPropagateMergeCommit = false
			return vm, vm.onDone()
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		vm.spinner, cmd = vm.spinner.Update(msg)
		return vm, cmd
	}
	return vm, nil
}

func (vm *GitHubFetchModel) View() tea.View {
	sb := strings.Builder{}
	showTree := false
	if vm.runningGitFetch {
		sb.WriteString(colors.ProgressStyle.Render(vm.spinner.View() + "Running git fetch..."))
		showTree = true
	} else if vm.runningGitHubAPIBranch >= 0 && vm.runningGitHubAPIBranch < len(vm.targetBranches) {
		sb.WriteString(colors.ProgressStyle.Render(vm.spinner.View() + "Querying GitHub API for " + vm.targetBranches[vm.runningGitHubAPIBranch].Short() + "..."))
		showTree = true
	} else if vm.runningCheckCommitHistory {
		sb.WriteString(colors.ProgressStyle.Render(vm.spinner.View() + "Checking commit history for merge commits..."))
		showTree = true
	} else if vm.runningPropagateMergeCommit {
		sb.WriteString(colors.ProgressStyle.Render(vm.spinner.View() + "Checking if sub-stacks are merged already..."))
		showTree = true
	} else {
		sb.WriteString(colors.SuccessStyle.Render("✓ GitHub fetch is done"))
	}

	if showTree {
		sb.WriteString("\n")

		syncedBranches := map[plumbing.ReferenceName]bool{}
		pendingBranches := map[plumbing.ReferenceName]bool{}
		for i, br := range vm.targetBranches {
			if i > vm.runningGitHubAPIBranch {
				pendingBranches[br] = true
			} else if i < vm.runningGitHubAPIBranch {
				syncedBranches[br] = true
			}
		}
		var brs []string
		for _, br := range vm.targetBranches {
			brs = append(brs, br.Short())
		}
		var nodes []*stackutils.StackTreeNode
		var err error
		nodes, err = stackutils.BuildStackTreeRelatedBranchStacks(
			vm.db.ReadTx(),
			vm.currentBranch.Short(),
			true,
			brs,
		)
		if err != nil {
			sb.WriteString("Failed to build stack tree: " + err.Error())
		} else {
			sb.WriteString("\n")
			for _, node := range nodes {
				sb.WriteString(stackutils.RenderTree(node, func(branchName string, isTrunk bool) string {
					var suffix string
					avbr, _ := vm.db.ReadTx().Branch(branchName)
					if avbr.MergeCommit != "" {
						suffix = " (merged)"
					}
					bn := plumbing.NewBranchReferenceName(branchName)
					if syncedBranches[bn] {
						return colors.SuccessStyle.Render("✓ " + branchName + suffix)
					}
					if pendingBranches[bn] {
						return colors.ProgressStyle.Render(branchName + suffix)
					}
					if vm.runningGitHubAPIBranch > 0 && vm.runningGitHubAPIBranch < len(vm.targetBranches) && vm.targetBranches[vm.runningGitHubAPIBranch] == bn {
						return colors.ProgressStyle.Render(vm.spinner.View() + branchName + suffix)
					}
					return branchName
				}))
			}
		}
	}
	return tea.NewView(sb.String())
}

func (vm *GitHubFetchModel) runGitFetch() tea.Msg {
	remote := vm.repo.GetRemoteName()
	if _, err := vm.repo.Git(context.Background(), "fetch", remote); err != nil {
		return errors.Errorf("failed to fetch from %s: %v", remote, err)
	}
	return &GitHubFetchProgress{gitFetchIsDone: true}
}

func (vm *GitHubFetchModel) runGitHubAPIFetch() tea.Msg {
	if len(vm.targetBranches) <= vm.runningGitHubAPIBranch {
		return &GitHubFetchProgress{apiFetchIsDone: true}
	}
	br := vm.targetBranches[vm.runningGitHubAPIBranch]
	tx := vm.db.WriteTx()
	defer tx.Abort()
	avbr, _ := tx.Branch(br.Short())
	if avbr.MergeCommit != "" {
		return &GitHubFetchProgress{apiFetchIsDone: true}
	}
	_, err := actions.UpdatePullRequestState(context.Background(), vm.client, tx, br.Short())
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return errors.Errorf("failed to commit: %v", err)
	}
	return &GitHubFetchProgress{apiFetchIsDone: true}
}

func (vm *GitHubFetchModel) updateMergeCommitsFromCommitMessage() tea.Msg {
	ctx := context.Background()
	trunkBranches := map[plumbing.ReferenceName][]string{}
	for _, br := range vm.targetBranches {
		trunk, ok := meta.Trunk(vm.db.ReadTx(), br.Short())
		if !ok {
			continue
		}
		trunkRef := plumbing.NewBranchReferenceName(trunk)
		trunkBranches[trunkRef] = append(trunkBranches[trunkRef], br.Short())
	}

	repo := vm.repo.GoGitRepo()
	remote, err := repo.Remote(vm.repo.GetRemoteName())
	if err != nil {
		return errors.Errorf("failed to get remote %s: %v", vm.repo.GetRemoteName(), err)
	}
	remoteConfig := remote.Config()

	mergedPRs := map[int64]string{}
	for trunkRef, branches := range trunkBranches {
		rtb := mapToRemoteTrackingBranch(remoteConfig, trunkRef)
		if rtb == nil {
			// No remote tracking branch. Skip.
			continue
		}
		ref, err := repo.Reference(*rtb, true)
		if err != nil {
			return errors.Errorf("failed to get reference %q: %v", rtb, err)
		}
		tip := ref.Hash().String()

		// A commit that closes a PR of one of the target branches cannot be
		// older than the point where their stacks forked from the trunk, so
		// only that range of the trunk history needs to be scanned. The
		// commit count cap is a best-effort safety valve for when the fork
		// point is very old (or cannot be determined).
		revisionRange := []string{"--max-count=10000", tip}
		mergeBaseArgs := append([]string{"merge-base", "--octopus", tip}, branches...)
		if base, err := vm.repo.Git(ctx, mergeBaseArgs...); err == nil {
			if base == tip {
				continue
			}
			revisionRange = []string{"--max-count=10000", base + ".." + tip}
		}
		cis, err := vm.repo.Log(ctx, git.LogOpts{RevisionRange: revisionRange})
		if err != nil {
			return errors.Errorf("failed to read the commit history of %q: %v", rtb, err)
		}
		maps.Copy(mergedPRs, git.FindClosesPullRequestComments(cis))
	}
	for _, br := range vm.targetBranches {
		tx := vm.db.WriteTx()
		avbr, _ := tx.Branch(br.Short())
		if avbr.MergeCommit != "" {
			tx.Abort()
			continue
		}
		if avbr.PullRequest != nil && avbr.PullRequest.Number != 0 {
			if hash, ok := mergedPRs[avbr.PullRequest.Number]; ok {
				avbr.MergeCommit = hash
				tx.SetBranch(avbr)
			}
		}
		if err := tx.Commit(); err != nil {
			return errors.Errorf("failed to commit: %v", err)
		}
	}
	return &GitHubFetchProgress{checkCommitHistoryIsDone: true}
}

func (vm *GitHubFetchModel) updateMergeCommitsFromChildren() tea.Msg {
	// If child branches are merged into trunk, the parent branches are also merged.
	// We need to verify the merge commit is actually in the trunk history before
	// propagating to prevent incorrectly marking branches as merged when a downstream
	// PR is flattened into its parent (GitHub marks it "merged" but it's not in trunk).
	ctx := context.Background()
	repo := vm.repo.GoGitRepo()
	remote, err := repo.Remote(vm.repo.GetRemoteName())
	if err != nil {
		return errors.Errorf("failed to get remote %s: %v", vm.repo.GetRemoteName(), err)
	}
	remoteConfig := remote.Config()

	// Build a map of trunk references and their remote tracking branches
	trunkRefs := map[plumbing.ReferenceName]plumbing.ReferenceName{}
	for _, br := range vm.targetBranches {
		avbr, _ := vm.db.ReadTx().Branch(br.Short())
		if avbr.Parent.Trunk {
			trunkRef := plumbing.NewBranchReferenceName(avbr.Parent.Name)
			// Skip if we've already processed this trunk
			if _, ok := trunkRefs[trunkRef]; ok {
				continue
			}
			rtb := mapToRemoteTrackingBranch(remoteConfig, trunkRef)
			if rtb != nil {
				trunkRefs[trunkRef] = *rtb
			}
		}
	}

	for _, br := range vm.targetBranches {
		tx := vm.db.WriteTx()
		avbr, _ := tx.Branch(br.Short())
		if avbr.MergeCommit == "" {
			tx.Abort()
			continue
		}

		// Check if we should propagate this merge commit up the stack
		shouldPropagate, err := vm.shouldPropagateMergeCommit(ctx, tx, avbr, trunkRefs)
		if err != nil {
			return err
		}
		if !shouldPropagate {
			tx.Abort()
			continue
		}

		// Propagate the merge commit to parent branches
		parent := avbr.Parent
		for !parent.Trunk {
			parentBr, ok := tx.Branch(parent.Name)
			if !ok {
				break
			}
			if parentBr.MergeCommit != "" {
				break
			}
			parentBr.MergeCommit = avbr.MergeCommit
			tx.SetBranch(parentBr)
			parent = parentBr.Parent
		}
		if err := tx.Commit(); err != nil {
			return errors.Errorf("failed to commit: %v", err)
		}
	}
	return &GitHubFetchProgress{mergeCommitPropagationIsDone: true}
}

// shouldPropagateMergeCommit verifies that a merge commit is actually in the trunk
// history before allowing it to be propagated to parent branches. This prevents
// incorrectly marking branches as merged when a downstream PR is flattened into
// its parent (GitHub marks it "merged" but it's not in trunk).
func (vm *GitHubFetchModel) shouldPropagateMergeCommit(
	ctx context.Context,
	tx meta.ReadTx,
	branch meta.Branch,
	trunkRefs map[plumbing.ReferenceName]plumbing.ReferenceName,
) (bool, error) {
	// Verify the merge commit is actually in the trunk history
	trunk, hasTrunk := meta.Trunk(tx, branch.Name)
	if !hasTrunk {
		return false, nil
	}

	trunkRef := plumbing.NewBranchReferenceName(trunk)
	remoteTrunkRef, ok := trunkRefs[trunkRef]
	if !ok {
		// No remote tracking branch for this trunk
		return false, nil
	}

	// Check if the merge commit is reachable from the remote trunk
	repo := vm.repo.GoGitRepo()
	ref, err := repo.Reference(remoteTrunkRef, true)
	if err != nil {
		return false, nil
	}

	// Use git merge-base to check if the merge commit is an ancestor of the trunk
	isAncestor, err := vm.repo.IsAncestor(ctx, branch.MergeCommit, ref.Hash().String())
	if err != nil || !isAncestor {
		// Merge commit is not in trunk history, don't propagate
		return false, nil
	}

	return true, nil
}

func mapToRemoteTrackingBranch(
	remoteConfig *config.RemoteConfig,
	refName plumbing.ReferenceName,
) *plumbing.ReferenceName {
	for _, fetch := range remoteConfig.Fetch {
		if fetch.Match(refName) {
			dst := fetch.Dst(refName)
			return &dst
		}
	}
	return nil
}
