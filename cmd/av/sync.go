package main

import (
	"context"
	"os"
	"path/filepath"
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
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/sliceutils"
	"github.com/aviator-co/av/internal/utils/uiutils"
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
	views  []tea.Model

	state        *syncState
	restackState *sequencerui.RestackState

	quitWithConflict bool
	err              error
}

func (vm *syncViewModel) Init() tea.Cmd {
	return vm.initSync()
}

func (vm *syncViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return vm, tea.Quit
		}
	case error:
		vm.err = msg
		return vm, tea.Quit
	}
	if len(vm.views) > 0 {
		idx := len(vm.views) - 1
		var cmd tea.Cmd
		vm.views[idx], cmd = vm.views[idx].Update(msg)
		return vm, cmd
	}
	return vm, nil
}

func (vm *syncViewModel) View() string {
	var ss []string
	for _, v := range vm.views {
		r := v.View()
		if r != "" {
			ss = append(ss, r)
		}
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

func (vm *syncViewModel) addView(m tea.Model) tea.Cmd {
	vm.views = append(vm.views, m)
	return m.Init()
}

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
	if isTrunkBranch && !syncFlags.All {
		return vm.initTrunkCheck()
	}
	return vm.initPreAvHook()
}

func (vm *syncViewModel) initTrunkCheck() tea.Cmd {
	return vm.addView(&uiutils.NewlineModel{Model: uiutils.NewPromptModel(
		"You are on the trunk, do you want to sync all stacks?",
		[]string{"Yes", "No"},
		func(choice string) tea.Cmd {
			// The callback must return a tea.Cmd to continue the execution chain.
			// When user selects "No", we quit immediately.
			if choice == "No" {
				return tea.Quit
			}
			// When user selects "Yes", we set the flag and continue to initPreAvHook.
			if choice == "Yes" {
				syncFlags.All = true
			}
			return vm.initPreAvHook()
		},
	)})
}

func (vm *syncViewModel) initPreAvHook() tea.Cmd {
	return vm.addView(newPreAvSyncHookModel(vm.repo, vm.initGitFetch))
}

func (vm *syncViewModel) initGitFetch() tea.Cmd {
	ctx := context.Background()
	status, err := vm.repo.Status(ctx)
	if err != nil {
		return uiutils.ErrCmd(err)
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
			return uiutils.ErrCmd(err)
		}
	} else {
		if _, exist := vm.db.ReadTx().Branch(currentBranch); !exist {
			return uiutils.ErrCmd(errors.New("current branch is not adopted to av"))
		}
		var err error
		if syncFlags.Current {
			targetBranches, err = planner.GetTargetBranches(ctx, vm.db.ReadTx(), vm.repo, true, planner.CurrentAndParents)
		} else {
			targetBranches, err = planner.GetTargetBranches(ctx, vm.db.ReadTx(), vm.repo, true, planner.CurrentStack)
		}
		if err != nil {
			return uiutils.ErrCmd(err)
		}
	}

	var currentBranchRef plumbing.ReferenceName
	if currentBranch != "" {
		currentBranchRef = plumbing.NewBranchReferenceName(currentBranch)
	}

	return vm.addView(ghui.NewGitHubFetchModel(
		vm.repo,
		vm.db,
		vm.client,
		currentBranchRef,
		targetBranches,
		vm.initSequencerState,
	))
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
	vm.restackState = state.RestackState
	return vm.addView(sequencerui.NewRestackModel(vm.repo, vm.db, state.RestackState, sequencerui.RestackStateOptions{
		Command:  "av sync",
		Abort:    syncFlags.Abort,
		Continue: syncFlags.Continue,
		Skip:     syncFlags.Skip,
		OnConflict: func() tea.Cmd {
			if err := vm.writeState(vm.restackState); err != nil {
				return uiutils.ErrCmd(err)
			}
			vm.quitWithConflict = true
			return tea.Quit
		},
		OnAbort: func() tea.Cmd {
			if err := vm.writeState(nil); err != nil {
				return uiutils.ErrCmd(err)
			}
			return tea.Quit
		},
		OnDone: func() tea.Cmd {
			if err := vm.writeState(nil); err != nil {
				return uiutils.ErrCmd(err)
			}
			return vm.initPushBranches()
		},
	}))
}

func (vm *syncViewModel) initPushBranches() tea.Cmd {
	return vm.addView(ghui.NewGitHubPushModel(
		vm.repo,
		vm.db,
		vm.client,
		vm.state.Push,
		vm.state.TargetBranches,
		vm.initPruneBranches,
	))
}

func (vm *syncViewModel) initPruneBranches() tea.Cmd {
	return vm.addView(gitui.NewPruneBranchModel(
		vm.repo,
		vm.db,
		vm.state.Prune,
		vm.state.TargetBranches,
		vm.restackState.InitialBranch,
		func() tea.Cmd {
			return tea.Quit
		},
	))
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

type preAvSyncHookModel struct {
	repo   *git.Repo
	onDone func() tea.Cmd

	hasHook  bool
	complete bool
}

type preAvSyncHookProgress struct{}

func newPreAvSyncHookModel(repo *git.Repo, onDone func() tea.Cmd) *preAvSyncHookModel {
	return &preAvSyncHookModel{
		repo:   repo,
		onDone: onDone,
	}
}

func (m *preAvSyncHookModel) Init() tea.Cmd {
	_, err := os.Lstat(filepath.Join(m.repo.GitDir(), "hooks", "pre-av-sync"))
	if err != nil {
		if os.IsNotExist(err) {
			return func() tea.Msg { return preAvSyncHookProgress{} }
		}
		return uiutils.ErrCmd(err)
	}
	m.hasHook = true
	cmd := m.repo.Cmd(context.Background(), []string{"hook", "run", "--ignore-missing", "pre-av-sync"}, nil)
	// Use tea.ExecProcess so that the hook can take over the terminal, allowing user to create
	// an interactive hook.
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return errors.Errorf("pre-av-sync hook failed: %v", err)
		}
		m.complete = true
		return preAvSyncHookProgress{}
	})
}

func (m *preAvSyncHookModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case preAvSyncHookProgress:
		return m, m.onDone()
	}
	return m, nil
}

func (m *preAvSyncHookModel) View() string {
	if !m.hasHook {
		// Do not even render anything if there is no hook for simplicity. We show the
		// status only when the hook exists.
		return ""
	}
	// NOTE: It is tempting to add a spinner here, but because the hook runs with a terminal
	// control, the spinner actually doesn't render / updated until the hook is done. Because
	// the terminal control is regained after the hook is done, and bubbletea will not erase the
	// content rendered while it didn't have a control (which is a sane behavior as it cannot
	// tell what was rendered while that) that previous spinner render will never be updated.
	// Due to this, we will have a better visual experience by not showing a spinner at all
	// here.
	if m.complete {
		return colors.SuccessStyle.Render("✓ pre-av-sync hook completed")
	}
	// We don't have to render anything here because the pre-av-sync failure message will be
	// rendered through error. The stdout / stderr of the hook will be shown directly in the
	// terminal as bubbletea won't erase the terminal content prior to the terminal control
	// take over.
	return ""
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
