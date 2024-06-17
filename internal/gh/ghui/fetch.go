package ghui

import (
	"context"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func NewGitHubFetchModel(repo *git.Repo, db meta.DB, client *gh.Client, currentBranch plumbing.ReferenceName, targetBranches []plumbing.ReferenceName) *GitHubFetchModel {
	return &GitHubFetchModel{
		repo:           repo,
		db:             db,
		client:         client,
		currentBranch:  currentBranch,
		targetBranches: targetBranches,
		spinner:        spinner.New(spinner.WithSpinner(spinner.Dot)),

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

type GitHubFetchDone struct{}

type GitHubFetchModel struct {
	repo           *git.Repo
	db             meta.DB
	client         *gh.Client
	currentBranch  plumbing.ReferenceName
	targetBranches []plumbing.ReferenceName
	spinner        spinner.Model

	runningGitFetch             bool
	runningGitHubAPIBranch      int
	runningCheckCommitHistory   bool
	runningPropagateMergeCommit bool
}

func (vm *GitHubFetchModel) Init() tea.Cmd {
	return tea.Batch(vm.spinner.Tick, vm.runGitFetch)
}

func (vm *GitHubFetchModel) Update(msg tea.Msg) (*GitHubFetchModel, tea.Cmd) {
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
			return vm, func() tea.Msg { return &GitHubFetchDone{} }
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		vm.spinner, cmd = vm.spinner.Update(msg)
		return vm, cmd
	}
	return vm, nil
}

func (vm *GitHubFetchModel) View() string {
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
		nodes, err = stackutils.BuildStackTreeRelatedBranchStacks(vm.db.ReadTx(), vm.currentBranch.Short(), true, brs)
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
	return sb.String()
}

func (vm *GitHubFetchModel) runGitFetch() tea.Msg {
	remote := vm.repo.GetRemoteName()
	if _, err := vm.repo.Git("fetch", remote); err != nil {
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
	trunkRefs := map[plumbing.ReferenceName]bool{}
	for _, br := range vm.targetBranches {
		avbr, _ := vm.db.ReadTx().Branch(br.Short())
		if avbr.Parent.Trunk {
			trunkRefs[plumbing.NewBranchReferenceName(avbr.Parent.Name)] = true
		}
	}

	repo := vm.repo.GoGitRepo()
	remote, err := repo.Remote(vm.repo.GetRemoteName())
	if err != nil {
		return errors.Errorf("failed to get remote %s: %v", vm.repo.GetRemoteName(), err)
	}
	remoteConfig := remote.Config()

	// For each trunk commits, look first 10000 commits to find the recently merge
	// commit. If we want to do this properly, we should look into the commits between
	// tip of the trunks and the merge bases of the branches. However, without
	// pre-calculated generation numbers (which is available in commit-graph), this
	// anyway would require a full history traversal.
	//
	// This is anyway a best-effort approach, so we just look into the first 10000.
	mergedPRs := map[int64]plumbing.Hash{}
	for trunkRef := range trunkRefs {
		rtb := mapToRemoteTrackingBranch(remoteConfig, trunkRef)
		if rtb == nil {
			// No remote tracking branch. Skip.
			continue
		}
		ref, err := repo.Reference(*rtb, true)
		if err != nil {
			return errors.Errorf("failed to get reference %q: %v", rtb, err)
		}
		c, err := repo.CommitObject(ref.Hash())
		if err != nil {
			return errors.Errorf("failed to get commit %q: %v", ref.Hash(), err)
		}
		visited := 0
		_ = object.NewCommitPreorderIter(c, nil, nil).ForEach(func(c *object.Commit) error {
			m := git.FindClosesPullRequestComments([]*git.CommitInfo{{
				Hash: c.Hash.String(),
				Body: c.Message,
			}})
			for pr := range m {
				mergedPRs[pr] = c.Hash
			}
			visited += 1
			if visited >= 10000 {
				return errors.New("stop")
			}
			return nil
		})
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
				avbr.MergeCommit = hash.String()
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
	// If child branches are merged, the parent branch is also merged.
	for _, br := range vm.targetBranches {
		tx := vm.db.WriteTx()
		avbr, _ := tx.Branch(br.Short())
		if avbr.MergeCommit == "" {
			tx.Abort()
			continue
		}
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

func mapToRemoteTrackingBranch(remoteConfig *config.RemoteConfig, refName plumbing.ReferenceName) *plumbing.ReferenceName {
	for _, fetch := range remoteConfig.Fetch {
		if fetch.Match(refName) {
			dst := fetch.Dst(refName)
			return &dst
		}
	}
	return nil
}
