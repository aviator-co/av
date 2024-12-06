package main

import (
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/config"
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
	"github.com/erikgeiser/promptkit/selection"
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

const (
	changeNoticePrompt     = "Are you OK to continue with the new behavior?"
	continueWithSyncChoice = "OK! Continue with av sync, rebasing onto the latest trunk (we will not ask again)"
	abortSyncChoice        = "Nope. Abort av sync (we will ask again next time)"
)

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
		repo, err := getRepo()
		if err != nil {
			return err
		}
		db, err := getDB(repo)
		if err != nil {
			return err
		}
		client, err := getGitHubClient()
		if err != nil {
			return err
		}

		return uiutils.RunBubbleTea(&syncViewModel{
			repo:             repo,
			db:               db,
			client:           client,
			help:             help.New(),
			askingSyncChange: !config.UserState.NotifiedStackSyncChange,
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

	state              *syncState
	changeNoticePrompt *selection.Model[string]
	syncAllPrompt      *selection.Model[string]
	githubFetchModel   *ghui.GitHubFetchModel
	restackModel       *sequencerui.RestackModel
	githubPushModel    *ghui.GitHubPushModel
	pruneBranchModel   *gitui.PruneBranchModel

	askingSyncChange bool
	pushingToGitHub  bool
	pruningBranches  bool

	quitWithAbortChoice bool
	quitWithConflict    bool
	err                 error
}

func (vm *syncViewModel) Init() tea.Cmd {
	if vm.askingSyncChange && os.Getenv("AV_STACK_SYNC_CHANGE_NO_ASK") != "1" {
		vm.changeNoticePrompt = uiutils.NewPromptModel(
			changeNoticePrompt,
			[]string{continueWithSyncChoice, abortSyncChoice},
		)
		return vm.changeNoticePrompt.Init()
	}
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
			return vm, func() tea.Msg { return err }
		}
		vm.quitWithConflict = true
		return vm, tea.Quit
	case *sequencerui.RestackAbort:
		if err := vm.writeState(nil); err != nil {
			return vm, func() tea.Msg { return err }
		}
		return vm, tea.Quit
	case *sequencerui.RestackDone:
		if err := vm.writeState(nil); err != nil {
			return vm, func() tea.Msg { return err }
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

	case promptUserShouldSyncAllMsg:
		vm.syncAllPrompt = uiutils.NewPromptModel("You are on the trunk, do you want to sync all stacks?", []string{"Yes", "No"})
		return vm, vm.syncAllPrompt.Init()

	case tea.KeyMsg:
		if vm.syncAllPrompt != nil {
			switch msg.String() {
			case " ", "enter":
				c, err := vm.syncAllPrompt.Value()
				if err != nil {
					vm.err = err
					return vm, tea.Quit
				}
				vm.syncAllPrompt = nil
				if c == "Yes" {
					syncFlags.All = true
				}
				if c == "No" {
					return vm, tea.Quit
				}
				return vm, vm.initSync()
			case "ctrl+c":
				return vm, tea.Quit
			default:
				_, cmd := vm.syncAllPrompt.Update(msg)
				return vm, cmd
			}
		} else if vm.askingSyncChange {

			switch msg.String() {
			case " ", "enter":
				c, err := vm.changeNoticePrompt.Value()
				if err != nil {
					vm.err = err
					return vm, tea.Quit
				}
				vm.askingSyncChange = false
				if c == continueWithSyncChoice {
					config.UserState.NotifiedStackSyncChange = true
					if err := config.SaveUserState(); err != nil {
						vm.err = err
						return vm, tea.Quit
					}
					return vm, vm.initSync()
				} else {
					vm.quitWithAbortChoice = true
					return vm, tea.Quit
				}
			case "ctrl+c":
				return vm, tea.Quit
			default:
				_, cmd := vm.changeNoticePrompt.Update(msg)
				return vm, cmd
			}
		} else if vm.pushingToGitHub {
			switch msg.String() {
			case "ctrl+c":
				return vm, tea.Quit
			default:
				_, cmd := vm.githubPushModel.Update(msg)
				return vm, cmd
			}
		} else if vm.pruningBranches {
			switch msg.String() {
			case "ctrl+c":
				return vm, tea.Quit
			default:
				_, cmd := vm.pruneBranchModel.Update(msg)
				return vm, cmd
			}
		} else {
			switch msg.String() {
			case "ctrl+c":
				return vm, tea.Quit
			}
		}
	case error:
		vm.err = msg
		return vm, tea.Quit
	}
	return vm, nil
}

func (vm *syncViewModel) View() string {
	var ss []string
	if vm.syncAllPrompt != nil {
		ss = append(ss, vm.syncAllPrompt.View())
		ss = append(ss, vm.help.ShortHelpView(uiutils.PromptKeys))
	}
	if vm.changeNoticePrompt != nil {
		ss = append(ss, vm.viewChangeNotice())
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

var commandStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))

func (vm *syncViewModel) viewChangeNotice() string {
	boldStyle := lipgloss.NewStyle().Bold(true)
	sb := strings.Builder{}
	sb.WriteString(
		boldStyle.Render(
			"The behavior of ",
		) + commandStyle.Bold(true).
			Render("av sync") +
			boldStyle.Render(
				" has changed. We will now ask for confirmation before syncing the stack.\n",
			),
	)
	sb.WriteString("\n")
	sb.WriteString("* " + commandStyle.Render("av sync") + " is split into four commands:\n")
	sb.WriteString(
		"  * " + commandStyle.Render("av adopt") + " to adopt a new branch into the stack.\n",
	)
	sb.WriteString(
		"  * " + commandStyle.Render("av reparent") + " to change the parent branch.\n",
	)
	sb.WriteString(
		"  * " + commandStyle.Render("av restack") + " to rebase the stack locally.\n",
	)
	sb.WriteString(
		"  * " + commandStyle.Render(
			"av sync",
		) + " to rebase the stack with the remote repository.\n",
	)
	sb.WriteString(
		"* " + commandStyle.Render(
			"av sync",
		) + " will ask if you want to push to the remote repository.\n",
	)
	sb.WriteString(
		"* " + commandStyle.Render(
			"av sync",
		) + " will ask if you want to delete the branches that have been merged.\n",
	)
	sb.WriteString("\n")
	sb.WriteString(
		"With this change, " + commandStyle.Render(
			"av sync",
		) + " will always rebase onto the remote trunk branch (e.g., main or\n",
	)
	sb.WriteString(
		"master). If you do not want to rebase onto the remote trunk branch, please use " + commandStyle.Render(
			"av restack",
		) + ".\n",
	)
	sb.WriteString("\n")
	sb.WriteString(vm.changeNoticePrompt.View())
	sb.WriteString(vm.help.ShortHelpView(uiutils.PromptKeys))
	sb.WriteString("\n")
	return sb.String()
}

type promptUserShouldSyncAllMsg struct {
}

func (vm *syncViewModel) initSync() tea.Cmd {
	state, err := vm.readState()
	if err != nil {
		return func() tea.Msg { return err }
	}
	if state != nil {
		return vm.continueWithState(state)
	}
	if syncFlags.Abort || syncFlags.Continue || syncFlags.Skip {
		return func() tea.Msg { return errors.New("no restack in progress") }
	}

	isTrunkBranch, err := vm.repo.IsCurrentBranchTrunk()
	if err != nil {
		return func() tea.Msg { return err }
	}
	if isTrunkBranch && !syncFlags.All {
		return func() tea.Msg {
			return promptUserShouldSyncAllMsg{}
		}
	}
	vm.githubFetchModel, err = vm.createGitHubFetchModel()
	if err != nil {
		return func() tea.Msg { return err }
	}
	return vm.githubFetchModel.Init()
}

func (vm *syncViewModel) initSequencerState() tea.Cmd {
	state, err := vm.createState()
	if err != nil {
		return func() tea.Msg { return err }
	}
	if state == nil {
		return func() tea.Msg { return nothingToRestackError }
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
	status, err := vm.repo.Status()
	if err != nil {
		return nil, err
	}
	currentBranch := status.CurrentBranch

	var targetBranches []plumbing.ReferenceName
	if syncFlags.All {
		var err error
		targetBranches, err = planner.GetTargetBranches(
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
			targetBranches, err = planner.GetTargetBranches(vm.db.ReadTx(), vm.repo, true, planner.CurrentAndParents)
		} else {
			targetBranches, err = planner.GetTargetBranches(vm.db.ReadTx(), vm.repo, true, planner.CurrentStack)
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
	state := savedSyncState{
		RestackState: &sequencerui.RestackState{},
		SyncState: &syncState{
			Push:  syncFlags.Push,
			Prune: syncFlags.Prune,
		},
	}
	status, err := vm.repo.Status()
	if err != nil {
		return nil, err
	}
	currentBranch := status.CurrentBranch
	state.RestackState.InitialBranch = currentBranch

	var targetBranches []plumbing.ReferenceName
	if syncFlags.All {
		var err error
		targetBranches, err = planner.GetTargetBranches(
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
			targetBranches, err = planner.GetTargetBranches(vm.db.ReadTx(), vm.repo, true, planner.CurrentAndParents)
		} else {
			targetBranches, err = planner.GetTargetBranches(vm.db.ReadTx(), vm.repo, true, planner.CurrentStack)
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
