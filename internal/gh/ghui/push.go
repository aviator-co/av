package ghui

import (
	"context"
	"fmt"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	avconfig "github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/ghutils"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/aviator-co/av/internal/utils/uiutils"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/erikgeiser/promptkit/selection"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/shurcooL/githubv4"
)

const (
	continuePush = "Yes. Push the branches to GitHub."
	abortPush    = "No. Do not push the branches to GitHub."

	reasonAlreadyUpToDate   = "Already up-to-date."
	reasonNotPushedToRemote = "No remote branch yet."
	reasonPRIsMerged        = "PR is already merged."
	reasonPRIsClosed        = "PR is closed."
	reasonParentNotPushed   = "Parent branch is not pushed to remote."
	reasonNoPR              = "Some branches in a stack do not have a PR."
)

type pushCandidate struct {
	branch       plumbing.ReferenceName
	remoteCommit *object.Commit
	localCommit  *object.Commit
}

type noPushBranch struct {
	branch plumbing.ReferenceName
	reason string
}

func NewGitHubPushModel(
	repo *git.Repo,
	db meta.DB,
	client *gh.Client,
	pushFlag string,
	targetBranches []plumbing.ReferenceName,
) *GitHubPushModel {
	var makeDraftBeforePush bool
	if avconfig.Av.PullRequest.RebaseWithDraft != nil {
		makeDraftBeforePush = *avconfig.Av.PullRequest.RebaseWithDraft
	} else {
		makeDraftBeforePush = ghutils.HasCodeowners(repo)
	}
	return &GitHubPushModel{
		repo:                repo,
		db:                  db,
		client:              client,
		makeDraftBeforePush: makeDraftBeforePush,
		pushFlag:            pushFlag,
		targetBranches:      targetBranches,
		spinner:             spinner.New(spinner.WithSpinner(spinner.Dot)),
		help:                help.New(),
		chooseNoPush:        pushFlag == "no",
	}
}

type GitHubPushProgress struct {
	candidateCalculationDone bool
	gitPushDone              bool
}

type GitHubPushDone struct{}

type GitHubPushModel struct {
	repo                *git.Repo
	db                  meta.DB
	client              *gh.Client
	makeDraftBeforePush bool
	pushFlag            string
	targetBranches      []plumbing.ReferenceName
	spinner             spinner.Model
	help                help.Model

	chooseNoPush   bool
	pushCandidates []pushCandidate
	noPushBranches []noPushBranch
	pushPrompt     *selection.Model[string]

	calculatingCandidates bool
	askingForConfirmation bool
	runningGitPush        bool
	done                  bool
}

func (vm *GitHubPushModel) Init() tea.Cmd {
	vm.calculatingCandidates = true
	return tea.Batch(vm.spinner.Tick, vm.calculateChangedBranches)
}

func (vm *GitHubPushModel) Update(msg tea.Msg) (*GitHubPushModel, tea.Cmd) {
	switch msg := msg.(type) {
	case *GitHubPushProgress:
		if msg.candidateCalculationDone {
			vm.calculatingCandidates = false
			if len(vm.pushCandidates) == 0 || vm.chooseNoPush {
				vm.done = true
				return vm, func() tea.Msg { return &GitHubPushDone{} }
			}
			if vm.pushFlag == "yes" {
				vm.runningGitPush = true
				return vm, vm.runUpdate
			}
			vm.askingForConfirmation = true
			vm.pushPrompt = uiutils.NewPromptModel("Are you OK with pushing these branches to remote?", []string{continuePush, abortPush})
			return vm, vm.pushPrompt.Init()
		}
		if msg.gitPushDone {
			vm.runningGitPush = false
			vm.done = true
			return vm, func() tea.Msg { return &GitHubPushDone{} }
		}
	case tea.KeyMsg:
		if vm.askingForConfirmation {
			switch msg.String() {
			case "enter":
				c, err := vm.pushPrompt.Value()
				if err != nil {
					return vm, func() tea.Msg { return err }
				}
				vm.askingForConfirmation = false
				vm.pushPrompt = nil
				if c != continuePush {
					vm.chooseNoPush = true
					vm.done = true
					return vm, func() tea.Msg { return &GitHubPushDone{} }
				}
				vm.runningGitPush = true
				return vm, vm.runUpdate
			case "ctrl+c":
				return vm, tea.Quit
			default:
				_, cmd := vm.pushPrompt.Update(msg)
				return vm, cmd
			}
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		vm.spinner, cmd = vm.spinner.Update(msg)
		return vm, cmd
	}
	return vm, nil
}

func (vm *GitHubPushModel) View() string {
	if vm.calculatingCandidates {
		return colors.ProgressStyle.Render(vm.spinner.View() + "Finding the changed branches...")
	}

	sb := strings.Builder{}
	if len(vm.pushCandidates) == 0 {
		sb.WriteString(colors.SuccessStyle.Render("✓ Nothing to push to GitHub"))
	} else if vm.askingForConfirmation {
		sb.WriteString("Confirming the push to GitHub")
	} else if vm.runningGitPush {
		sb.WriteString(colors.ProgressStyle.Render(vm.spinner.View() + "Pushing to GitHub..."))
	} else if vm.done {
		if vm.chooseNoPush {
			sb.WriteString(colors.SuccessStyle.Render("✓ Not pushing to GitHub"))
		} else {
			sb.WriteString(colors.SuccessStyle.Render("✓ Pushed to GitHub"))
		}
	}

	if len(vm.noPushBranches) > 0 || len(vm.pushCandidates) > 0 {
		sb.WriteString("\n")
	}

	if len(vm.noPushBranches) > 0 {
		sb.WriteString("\n")
		sb.WriteString("  Following branches do not need a push.\n")
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().MarginLeft(4).Render(vm.viewNoPushBranches()))
	}
	if len(vm.pushCandidates) > 0 {
		sb.WriteString("\n")
		if vm.runningGitPush {
			sb.WriteString("  Following branches are being pushed...\n")
		} else if vm.done && !vm.chooseNoPush {
			sb.WriteString("  Following branches are pushed.\n")
		} else {
			sb.WriteString("  Following branches need to be pushed.\n")
		}
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().MarginLeft(4).Render(vm.viewPushCandidates()))
	}

	if vm.pushPrompt != nil {
		sb.WriteString("\n")
		sb.WriteString(vm.pushPrompt.View())
		sb.WriteString(vm.help.ShortHelpView(uiutils.PromptKeys))
	}
	return sb.String()
}

func (vm *GitHubPushModel) viewPushCandidates() string {
	sb := strings.Builder{}
	for i, branch := range vm.pushCandidates {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(branch.branch.Short() + "\n")
		sb.WriteString(
			"  Remote: " + branch.remoteCommit.Hash.String()[:7] + " " + getFirstLine(
				branch.remoteCommit.Message,
			) + " " + branch.remoteCommit.Committer.When.String() + " (" + humanize.Time(
				branch.remoteCommit.Committer.When,
			) + ")\n",
		)
		sb.WriteString(
			"  Local:  " + branch.localCommit.Hash.String()[:7] + " " + getFirstLine(
				branch.localCommit.Message,
			) + " " + branch.localCommit.Committer.When.String() + " (" + humanize.Time(
				branch.localCommit.Committer.When,
			) + ")\n",
		)
		avbr, _ := vm.db.ReadTx().Branch(branch.branch.Short())
		if avbr.PullRequest != nil && avbr.PullRequest.Permalink != "" {
			sb.WriteString("  PR:     " + avbr.PullRequest.Permalink + "\n")
		}
	}
	return sb.String()
}

func (vm *GitHubPushModel) viewNoPushBranches() string {
	sb := strings.Builder{}
	for _, branch := range vm.noPushBranches {
		sb.WriteString(branch.branch.Short() + ": " + branch.reason + "\n")
	}
	return sb.String()
}

func (vm *GitHubPushModel) runUpdate() (ret tea.Msg) {
	ghPRs, err := vm.getPRs()
	if err != nil {
		return err
	}
	if vm.makeDraftBeforePush {
		if err := vm.makePRsDraft(ghPRs); err != nil {
			return err
		}
	}
	defer func() {
		if vm.makeDraftBeforePush {
			if err := vm.undraftPRs(ghPRs); err != nil {
				ret = err
			}
		}
	}()
	if err := vm.updatePRs(ghPRs); err != nil {
		return err
	}
	if err := vm.runGitPush(); err != nil {
		return err
	}
	return &GitHubPushProgress{gitPushDone: true}
}

func (vm *GitHubPushModel) runGitPush() error {
	pushArgs := []string{"push", vm.repo.GetRemoteName(), "--atomic"}
	for _, branch := range vm.pushCandidates {
		// Do a compare-and-swap to be strict on what we show as a difference.
		pushArgs = append(
			pushArgs,
			fmt.Sprintf(
				"--force-with-lease=%s:%s",
				branch.branch.String(),
				branch.remoteCommit.Hash.String(),
			),
		)
	}
	for _, branch := range vm.pushCandidates {
		// Push the exact commit hash to be strict on what we show as a difference.
		pushArgs = append(pushArgs,
			fmt.Sprintf("%s:%s", branch.localCommit.Hash.String(), branch.branch.String()),
		)
	}
	res, err := vm.repo.Run(&git.RunOpts{
		Args: pushArgs,
	})
	if err != nil {
		return errors.WrapIff(err, "failed to push branches to GitHub")
	}
	if res.ExitCode != 0 {
		return errors.Errorf("failed to push branches to GitHub\n%s\n%s", res.Stdout, res.Stderr)
	}

	for _, branch := range vm.pushCandidates {
		if err := vm.repo.BranchSetConfig(branch.branch.Short(), "av-pushed-remote", vm.repo.GetRemoteName()); err != nil {
			return err
		}
		if err := vm.repo.BranchSetConfig(branch.branch.Short(), "av-pushed-ref", branch.branch.String()); err != nil {
			return err
		}
		if err := vm.repo.BranchSetConfig(branch.branch.Short(), "av-pushed-commit", branch.localCommit.Hash.String()); err != nil {
			return err
		}
	}
	return nil
}

func (vm *GitHubPushModel) getPRs() (map[plumbing.ReferenceName]*gh.PullRequest, error) {
	prs := map[plumbing.ReferenceName]*gh.PullRequest{}
	for _, branch := range vm.pushCandidates {
		avbr, _ := vm.db.ReadTx().Branch(branch.branch.Short())
		if avbr.PullRequest != nil {
			pr, err := vm.client.PullRequest(context.Background(), avbr.PullRequest.ID)
			if err != nil {
				return nil, err
			}
			prs[branch.branch] = pr
		}
	}
	return prs, nil
}

func (vm *GitHubPushModel) makePRsDraft(ghPRs map[plumbing.ReferenceName]*gh.PullRequest) error {
	for _, pr := range ghPRs {
		if pr.State == "OPEN" && !pr.IsDraft {
			if _, err := vm.client.ConvertPullRequestToDraft(context.Background(), pr.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (vm *GitHubPushModel) updatePRs(ghPRs map[plumbing.ReferenceName]*gh.PullRequest) error {
	for br, pr := range ghPRs {
		avbr, _ := vm.db.ReadTx().Branch(br.Short())
		prMeta := createPRMetadata(avbr, vm)

		var stackToWrite *stackutils.StackTreeNode
		if avconfig.Av.PullRequest.WriteStack {
			var err error
			if stackToWrite, err = stackutils.BuildStackTreeCurrentStack(vm.db.ReadTx(), br.Short(), false); err != nil {
				return err
			}
		}
		prBody := actions.AddPRMetadataAndStack(
			pr.Body,
			prMeta,
			avbr.Name,
			stackToWrite,
			vm.db.ReadTx(),
		)
		if _, err := vm.client.UpdatePullRequest(context.Background(), githubv4.UpdatePullRequestInput{
			PullRequestID: pr.ID,
			BaseRefName:   githubv4.NewString(githubv4.String(avbr.Parent.Name)),
			Body:          githubv4.NewString(githubv4.String(prBody)),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (vm *GitHubPushModel) undraftPRs(ghPRs map[plumbing.ReferenceName]*gh.PullRequest) error {
	for _, pr := range ghPRs {
		if pr.State == "OPEN" && !pr.IsDraft {
			if _, err := vm.client.MarkPullRequestReadyForReview(context.Background(), pr.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (vm *GitHubPushModel) calculateChangedBranches() tea.Msg {
	repo := vm.repo.GoGitRepo()
	remote, err := repo.Remote(vm.repo.GetRemoteName())
	if err != nil {
		return errors.Errorf("failed to get remote %s: %v", vm.repo.GetRemoteName(), err)
	}
	remoteConfig := remote.Config()

	var noPushBranches []noPushBranch
	var pushCandidates []pushCandidate
	for _, br := range vm.targetBranches {
		avbr, _ := vm.db.ReadTx().Branch(br.Short())
		if avbr.MergeCommit != "" ||
			(avbr.PullRequest != nil && avbr.PullRequest.State == "MERGED") {
			noPushBranches = append(noPushBranches, noPushBranch{
				branch: br,
				reason: reasonPRIsMerged,
			})
			continue
		}
		if avbr.PullRequest != nil && avbr.PullRequest.State == "CLOSED" {
			noPushBranches = append(noPushBranches, noPushBranch{
				branch: br,
				reason: reasonPRIsClosed,
			})
			continue
		}

		rtb := mapToRemoteTrackingBranch(remoteConfig, br)
		if rtb == nil {
			noPushBranches = append(noPushBranches, noPushBranch{
				branch: br,
				reason: reasonNotPushedToRemote,
			})
			continue
		}

		remoteRef, err := repo.Reference(*rtb, true)
		if err != nil {
			noPushBranches = append(noPushBranches, noPushBranch{
				branch: br,
				reason: reasonNotPushedToRemote,
			})
			continue
		}

		localRef, err := repo.Reference(br, true)
		if err != nil {
			return err
		}

		if localRef.Hash() == remoteRef.Hash() && !isDifferencePRMetadata(avbr, vm) {
			noPushBranches = append(noPushBranches, noPushBranch{
				branch: br,
				reason: reasonAlreadyUpToDate,
			})
			continue
		}

		if !vm.allParentsHaveRemoteTrackingBranch(remoteConfig, br) {
			// If a parent doesn't have a remote tracking branch, the PR cannot be made
			// with that branch as the base. We cannot push this branch.
			noPushBranches = append(noPushBranches, noPushBranch{
				branch: br,
				reason: reasonParentNotPushed,
			})
			continue
		}
		if !vm.allBranchesOnStackHavePRs(br) {
			// If an ancestor branch doesn't have a PR, the PR cannot be made with the
			// right metadata.
			noPushBranches = append(noPushBranches, noPushBranch{
				branch: br,
				reason: reasonNoPR,
			})
			continue
		}

		remoteRefCommit, err := repo.CommitObject(remoteRef.Hash())
		if err != nil {
			return err
		}
		localRefCommit, err := repo.CommitObject(localRef.Hash())
		if err != nil {
			return err
		}

		pushCandidates = append(pushCandidates, pushCandidate{
			branch:       br,
			remoteCommit: remoteRefCommit,
			localCommit:  localRefCommit,
		})
	}
	vm.noPushBranches = noPushBranches
	vm.pushCandidates = pushCandidates
	return &GitHubPushProgress{candidateCalculationDone: true}
}

func (vm *GitHubPushModel) allParentsHaveRemoteTrackingBranch(
	remoteConfig *config.RemoteConfig,
	br plumbing.ReferenceName,
) bool {
	avbr, _ := vm.db.ReadTx().Branch(br.Short())
	parent := avbr.Parent
	for !parent.Trunk {
		avbr, _ := vm.db.ReadTx().Branch(parent.Name)
		rtb := mapToRemoteTrackingBranch(remoteConfig, plumbing.NewBranchReferenceName(parent.Name))
		if rtb == nil {
			return false
		}
		parent = avbr.Parent
	}
	return true
}

func (vm *GitHubPushModel) allBranchesOnStackHavePRs(br plumbing.ReferenceName) bool {
	avbr, _ := vm.db.ReadTx().Branch(br.Short())
	for {
		if avbr.PullRequest == nil {
			return false
		}
		if avbr.Parent.Trunk {
			return true
		}
		avbr, _ = vm.db.ReadTx().Branch(avbr.Parent.Name)
	}
}

func getFirstLine(s string) string {
	idx := strings.Index(s, "\n")
	if idx == -1 {
		return s
	}
	return s[:idx]
}

func createPRMetadata(branch meta.Branch, vm *GitHubPushModel) actions.PRMetadata {
	trunk, _ := meta.Trunk(vm.db.ReadTx(), branch.Name)

	metadata := actions.PRMetadata{
		Parent:     branch.Parent.Name,
		ParentHead: branch.Parent.Head,
		Trunk:      trunk,
	}

	if !branch.Parent.Trunk {
		parent, _ := vm.db.ReadTx().Branch(branch.Parent.Name)
		if parent.PullRequest != nil {
			metadata.ParentPull = parent.PullRequest.Number
		}
	}

	return metadata
}

// Compare local metadata with PR metadata for any changes
// If something error occurs, return true to be safe
func isDifferencePRMetadata(avbr meta.Branch, vm *GitHubPushModel) bool {
	local := createPRMetadata(avbr, vm)

	pr, err := vm.client.PullRequest(context.Background(), avbr.PullRequest.ID)
	if err != nil {
		return true
	}

	prMeta, err := actions.ReadPRMetadata(pr.Body)
	if err != nil {
		return true
	}

	return local == prMeta
}
