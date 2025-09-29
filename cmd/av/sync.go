package main

import (
	"context"
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/gh/ghui"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gitui"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/sequencer"
	"github.com/aviator-co/av/internal/sequencer/planner"
	"github.com/aviator-co/av/internal/sequencer/sequencerui"
	"github.com/aviator-co/av/internal/utils/sliceutils"
	"github.com/aviator-co/av/internal/utils/uiutils"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
)

var syncFlags struct {
	All           bool
	RebaseToTrunk bool
	Current       bool
	Abort         bool
	Continue      bool
	Skip          bool
	Push          string
	Prune         string
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize stacked branches with GitHub",
	Long: strings.TrimSpace(`
Synchronize stacked branches to be up-to-date with their parent branches.

By default, this command will sync all branches starting at the root of the
stack and recursively rebasing each branch based on the latest commit from the
parent branch.

If the --all flag is given, this command will sync all branches in the repository.

If the --current flag is given, this command will not recursively sync dependent
branches of the current branch within the stack. This allows you to make changes
to the current branch before syncing the rest of the stack.

If the --rebase-to-trunk flag is given, this command will synchronize changes from the
latest commit to the repository base branch (e.g., main or master) into the
stack. This is useful for rebasing a whole stack on the latest changes from the
base branch.
`),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		if !sliceutils.Contains(
			[]string{"ask", "yes", "no"},
			strings.ToLower(syncFlags.Push),
		) {
			return errors.New("invalid value for --push; must be one of ask, yes, no")
		}
		if !sliceutils.Contains(
			[]string{"ask", "yes", "no"},
			strings.ToLower(syncFlags.Prune),
		) {
			return errors.New("invalid value for --prune; must be one of ask, yes, no")
		}
		if cmd.Flags().Changed("no-fetch") {
			return actions.ErrExitSilently{ExitCode: 1}
		}
		if cmd.Flags().Changed("trunk") {
			return actions.ErrExitSilently{ExitCode: 1}
		}
		if cmd.Flags().Changed("parent") {
			return actions.ErrExitSilently{ExitCode: 1}
		}
		repo, err := getRepo(ctx)
		if err != nil {
			return err
		}
		db, err := getDB(ctx, repo)
		if err != nil {
			return err
		}
		client, err := getGitHubClient(ctx)
		if err != nil {
			return err
		}

		return uiutils.RunBubbleTea(&syncViewModel{
			repo:   repo,
			db:     db,
			client: client,
			help:   help.New(),
		})
	},
}

type savedSyncState struct {
	RestackState *sequencerui.RestackState
	SyncState    *syncState
}

type syncState struct {
	TargetBranches []plumbing.ReferenceName
	Prune          string
	Push           string
}

type syncViewModel struct {
	repo   *git.Repo
	db     meta.DB
	client *gh.Client
	help   help.Model
	views  []tea.Model

	state            *syncState
	syncAllPrompt    tea.Model
	githubFetchModel *ghui.GitHubFetchModel
	restackModel     *sequencerui.RestackModel
	githubPushModel  *ghui.GitHubPushModel
	pruneBranchModel *gitui.PruneBranchModel

	pushingToGitHub bool
	pruningBranches bool

	quitWithConflict bool
	err              error
}

func (vm *syncViewModel) Init() tea.Cmd {
	return vm.initSync()
}

func (vm *syncViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmds []tea.Cmd
		if vm.githubFetchModel != nil {
			var cmd tea.Cmd
			vm.githubFetchModel, cmd = vm.githubFetchModel.Update(msg)
			cmds = append(cmds, cmd)
		}
		if vm.restackModel != nil {
			var cmd tea.Cmd
			vm.restackModel, cmd = vm.restackModel.Update(msg)
			cmds = append(cmds, cmd)
		}
		if vm.githubPushModel != nil {
			var cmd tea.Cmd
			vm.githubPushModel, cmd = vm.githubPushModel.Update(msg)
			cmds = append(cmds, cmd)
		}
		if vm.pruneBranchModel != nil {
			var cmd tea.Cmd
			vm.pruneBranchModel, cmd = vm.pruneBranchModel.Update(msg)
			cmds = append(cmds, cmd)
		}
		return vm, tea.Batch(cmds...)

	case preAvSyncHookDoneMsg:
		var err error
		vm.githubFetchModel, err = vm.createGitHubFetchModel()
		if err != nil {
			return vm, uiutils.ErrCmd(err)
		}
		return vm, vm.githubFetchModel.Init()

	case *ghui.GitHubFetchProgress:
		var cmd tea.Cmd
		vm.githubFetchModel, cmd = vm.githubFetchModel.Update(msg)
		return vm, cmd
	case *ghui.GitHubFetchDone:
		return vm, vm.initSequencerState()

	case *sequencerui.RestackProgress:
		var cmd tea.Cmd
		vm.restackModel, cmd = vm.restackModel.Update(msg)
		return vm, cmd
	case *sequencerui.RestackConflict:
		if err := vm.writeState(vm.restackModel.State); err != nil {
			return vm, uiutils.ErrCmd(err)
		}
		vm.quitWithConflict = true
		return vm, tea.Quit
	case *sequencerui.RestackAbort:
		if err := vm.writeState(nil); err != nil {
			return vm, uiutils.ErrCmd(err)
		}
		return vm, tea.Quit
	case *sequencerui.RestackDone:
		if err := vm.writeState(nil); err != nil {
			return vm, uiutils.ErrCmd(err)
		}
		return vm, vm.initPushBranches()

	case *ghui.GitHubPushProgress:
		var cmd tea.Cmd
		vm.githubPushModel, cmd = vm.githubPushModel.Update(msg)
		return vm, cmd
	case *ghui.GitHubPushDone:
		vm.pushingToGitHub = false
		return vm, vm.initPruneBranches()

	case *gitui.PruneBranchProgress:
		var cmd tea.Cmd
		vm.pruneBranchModel, cmd = vm.pruneBranchModel.Update(msg)
		return vm, cmd
	case *gitui.PruneBranchDone:
		vm.pruningBranches = false
		return vm, tea.Quit

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return vm, tea.Quit
		}
		if vm.syncAllPrompt != nil {
			_, cmd := vm.syncAllPrompt.Update(msg)
			return vm, cmd
		} else if vm.pushingToGitHub {
			_, cmd := vm.githubPushModel.Update(msg)
			return vm, cmd
		} else if vm.pruningBranches {
			_, cmd := vm.pruneBranchModel.Update(msg)
			return vm, cmd
		}
	case error:
		vm.err = msg
		return vm, tea.Quit
	}
	return vm, nil
}

func (vm *syncViewModel) View() string {
	var ss []string
	for _, v := range vm.views {
		ss = append(ss, v.View())
	}
	if vm.githubFetchModel != nil {
		ss = append(ss, vm.githubFetchModel.View())
	}
	if vm.restackModel != nil {
		ss = append(ss, vm.restackModel.View())
	}
	if vm.githubPushModel != nil {
		ss = append(ss, vm.githubPushModel.View())
	}
	if vm.pruneBranchModel != nil {
		ss = append(ss, vm.pruneBranchModel.View())
	}

	var ret string
	if len(ss) != 0 {
		ret = lipgloss.NewStyle().MarginTop(1).MarginBottom(1).MarginLeft(2).Render(
			lipgloss.JoinVertical(0, ss...),
		)
	}
	if vm.err != nil {
		if len(ret) != 0 {
			ret += "\n"
		}
		ret += renderError(vm.err)
	}
	return ret
}

type preAvSyncHookDoneMsg struct{}

func (vm *syncViewModel) initSync() tea.Cmd {
	state, err := vm.readState()
	if err != nil {
		return uiutils.ErrCmd(err)
	}
	if state != nil {
		return vm.continueWithState(state)
	}
	if syncFlags.Abort || syncFlags.Continue || syncFlags.Skip {
		return uiutils.ErrCmd(errors.New("no restack in progress"))
	}

	isTrunkBranch, err := vm.repo.IsCurrentBranchTrunk(context.Background())
	if err != nil {
		return uiutils.ErrCmd(err)
	}
	continuation := func() tea.Msg {
		output, err := vm.repo.Run(
			context.Background(),
			&git.RunOpts{
				Args:        []string{"hook", "run", "--ignore-missing", "pre-av-sync"},
				Interactive: true,
				ExitError:   true,
			},
		)
		var messages []string
		if output != nil {
			if len(output.Stdout) != 0 {
				messages = append(messages, string(output.Stdout))
			}
			if len(output.Stderr) != 0 {
				messages = append(messages, string(output.Stderr))
			}
		}
		if len(messages) != 0 {
			vm.views = append(vm.views, uiutils.SimpleMessageView{Message: strings.Join(messages, "\n")})
		}
		if err != nil {
			return errors.Errorf("pre-av-sync hook failed: %v", err)
		}
		return preAvSyncHookDoneMsg{}
	}
	if isTrunkBranch && !syncFlags.All {
		vm.syncAllPrompt = &uiutils.NewlineModel{Model: uiutils.NewPromptModel(
			"You are on the trunk, do you want to sync all stacks?",
			[]string{"Yes", "No"},
			func(choice string) tea.Cmd {
				if choice == "Yes" {
					syncFlags.All = true
				}
				if choice == "No" {
					return tea.Quit
				}
				return continuation
			},
		)}
		vm.views = append(vm.views, vm.syncAllPrompt)
		return vm.syncAllPrompt.Init()
	}
	return continuation
}

func (vm *syncViewModel) initSequencerState() tea.Cmd {
	state, err := vm.createState()
	if err != nil {
		return uiutils.ErrCmd(err)
	}
	if state == nil {
		return uiutils.ErrCmd(nothingToRestackError)
	}
	return vm.continueWithState(state)
}

func (vm *syncViewModel) continueWithState(state *savedSyncState) tea.Cmd {
	vm.state = state.SyncState
	vm.restackModel = sequencerui.NewRestackModel(vm.repo, vm.db)
	vm.restackModel.Command = "av sync"
	vm.restackModel.State = state.RestackState
	vm.restackModel.Abort = syncFlags.Abort
	vm.restackModel.Continue = syncFlags.Continue
	vm.restackModel.Skip = syncFlags.Skip
	return vm.restackModel.Init()
}

func (vm *syncViewModel) readState() (*savedSyncState, error) {
	var state savedSyncState
	if err := vm.repo.ReadStateFile(git.StateFileKindSyncV2, &state); err != nil &&
		os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &state, nil
}

func (vm *syncViewModel) writeState(seqModel *sequencerui.RestackState) error {
	if seqModel == nil {
		return vm.repo.WriteStateFile(git.StateFileKindSyncV2, nil)
	}
	var state savedSyncState
	state.RestackState = seqModel
	state.SyncState = vm.state
	return vm.repo.WriteStateFile(git.StateFileKindSyncV2, &state)
}

func (vm *syncViewModel) createGitHubFetchModel() (*ghui.GitHubFetchModel, error) {
	ctx := context.Background()
	status, err := vm.repo.Status(ctx)
	if err != nil {
		return nil, err
	}
	currentBranch := status.CurrentBranch

	var targetBranches []plumbing.ReferenceName
	if syncFlags.All {
		var err error
		targetBranches, err = planner.GetTargetBranches(
			ctx,
			vm.db.ReadTx(),
			vm.repo,
			true,
			planner.AllBranches,
		)
		if err != nil {
			return nil, err
		}
	} else {
		if _, exist := vm.db.ReadTx().Branch(currentBranch); !exist {
			return nil, errors.New("current branch is not adopted to av")
		}
		var err error
		if syncFlags.Current {
			targetBranches, err = planner.GetTargetBranches(ctx, vm.db.ReadTx(), vm.repo, true, planner.CurrentAndParents)
		} else {
			targetBranches, err = planner.GetTargetBranches(ctx, vm.db.ReadTx(), vm.repo, true, planner.CurrentStack)
		}
		if err != nil {
			return nil, err
		}
	}

	var currentBranchRef plumbing.ReferenceName
	if currentBranch != "" {
		currentBranchRef = plumbing.NewBranchReferenceName(currentBranch)
	}

	return ghui.NewGitHubFetchModel(
		vm.repo,
		vm.db,
		vm.client,
		currentBranchRef,
		targetBranches,
	), nil
}

func (vm *syncViewModel) createState() (*savedSyncState, error) {
	ctx := context.Background()
	state := savedSyncState{
		RestackState: &sequencerui.RestackState{},
		SyncState: &syncState{
			Push:  syncFlags.Push,
			Prune: syncFlags.Prune,
		},
	}
	status, err := vm.repo.Status(ctx)
	if err != nil {
		return nil, err
	}
	currentBranch := status.CurrentBranch
	state.RestackState.InitialBranch = currentBranch

	var targetBranches []plumbing.ReferenceName
	if syncFlags.All {
		var err error
		targetBranches, err = planner.GetTargetBranches(
			ctx,
			vm.db.ReadTx(),
			vm.repo,
			true,
			planner.AllBranches,
		)
		if err != nil {
			return nil, err
		}
		state.RestackState.RestackingAll = true
	} else {
		if _, exist := vm.db.ReadTx().Branch(currentBranch); !exist {
			return nil, errors.New("current branch is not adopted to av")
		}
		var err error
		if syncFlags.Current {
			targetBranches, err = planner.GetTargetBranches(ctx, vm.db.ReadTx(), vm.repo, true, planner.CurrentAndParents)
		} else {
			targetBranches, err = planner.GetTargetBranches(ctx, vm.db.ReadTx(), vm.repo, true, planner.CurrentStack)
		}
		if err != nil {
			return nil, err
		}
		state.RestackState.RelatedBranches = append(state.RestackState.RelatedBranches, currentBranch)
	}
	state.SyncState.TargetBranches = targetBranches

	var currentBranchRef plumbing.ReferenceName
	if currentBranch != "" {
		currentBranchRef = plumbing.NewBranchReferenceName(currentBranch)
	}
	ops, err := planner.PlanForSync(
		ctx,
		vm.db.ReadTx(),
		vm.repo,
		currentBranchRef,
		syncFlags.All,
		syncFlags.Current,
		syncFlags.RebaseToTrunk,
	)
	if err != nil {
		return nil, err
	}
	state.RestackState.Seq = sequencer.NewSequencer(vm.repo.GetRemoteName(), vm.db, ops)
	return &state, nil
}

func (vm *syncViewModel) initPushBranches() tea.Cmd {
	vm.githubPushModel = ghui.NewGitHubPushModel(
		vm.repo,
		vm.db,
		vm.client,
		vm.state.Push,
		vm.state.TargetBranches,
	)
	vm.pushingToGitHub = true
	return vm.githubPushModel.Init()
}

func (vm *syncViewModel) initPruneBranches() tea.Cmd {
	vm.pruneBranchModel = gitui.NewPruneBranchModel(
		vm.repo,
		vm.db,
		vm.state.Prune,
		vm.state.TargetBranches,
		vm.restackModel.State.InitialBranch,
	)
	vm.pruningBranches = true
	return vm.pruneBranchModel.Init()
}

func (vm *syncViewModel) ExitError() error {
	if errors.Is(vm.err, nothingToRestackError) {
		return nil
	}
	if vm.err != nil {
		return actions.ErrExitSilently{ExitCode: 1}
	}
	if vm.quitWithConflict {
		return actions.ErrExitSilently{ExitCode: 1}
	}
	return nil
}

func init() {
	syncCmd.Flags().BoolVar(
		&syncFlags.All, "all", false,
		"synchronize all branches",
	)
	syncCmd.Flags().BoolVar(
		&syncFlags.Current, "current", false,
		"only sync changes to the current branch\n(don't recurse into descendant branches)",
	)
	syncCmd.Flags().StringVar(
		&syncFlags.Push, "push", "ask",
		"push the rebased branches to the remote repository\n(ask|yes|no)",
	)
	syncCmd.Flags().StringVar(
		&syncFlags.Prune, "prune", "ask",
		"delete branches that have been merged into the parent branch\n(ask|yes|no)",
	)
	syncCmd.Flags().Lookup("prune").NoOptDefVal = "ask"
	syncCmd.Flags().BoolVar(
		&syncFlags.RebaseToTrunk, "rebase-to-trunk", false,
		"rebase the branches to the latest trunk always",
	)

	syncCmd.Flags().BoolVar(
		&syncFlags.Continue, "continue", false,
		"continue an in-progress sync",
	)
	syncCmd.Flags().BoolVar(
		&syncFlags.Abort, "abort", false,
		"abort an in-progress sync",
	)
	syncCmd.Flags().BoolVar(
		&syncFlags.Skip, "skip", false,
		"skip the current commit and continue an in-progress sync",
	)
	syncCmd.MarkFlagsMutuallyExclusive("current", "all")
	syncCmd.MarkFlagsMutuallyExclusive("continue", "abort", "skip")

	// Deprecated flags
	syncCmd.Flags().Bool("no-fetch", false,
		"(deprecated; use av restack for offline restacking) do not fetch the latest status from GitHub",
	)
	_ = syncCmd.Flags().
		MarkDeprecated("no-fetch", "please use av restack for offline restacking")

	syncCmd.Flags().Bool("trunk", false,
		"(deprecated; use --rebase-to-trunk to rebase all branches to trunk) rebase the stack on the trunk branch",
	)
	_ = syncCmd.Flags().
		MarkDeprecated("trunk", "please use --rebase-to-trunk to rebase all branches to trunk")

	syncCmd.Flags().String("parent", "",
		"(deprecated; use 'av adopt' or 'av reparent') parent branch to rebase onto",
	)
	_ = syncCmd.Flags().
		MarkDeprecated("parent", "please use 'av adopt' or 'av reparent'")
}
