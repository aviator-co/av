package main

import (
	"context"
	"fmt"
	"strconv"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/uiutils"
	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/erikgeiser/promptkit/selection"
	"github.com/spf13/cobra"
)

var nextFlags struct {
	// should we go to the last
	Last bool
}

var nextCmd = &cobra.Command{
	Use:   "next [<n>|--last]",
	Short: "Checkout the next branch in the stack",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		n := 1
		if len(args) == 1 {
			var err error
			n, err = strconv.Atoi(args[0])
			if err != nil {
				return errors.New("invalid number (unable to parse)")
			}

			if n <= 0 {
				return errors.New("invalid number (must be >= 1)")
			}
		}

		stackNext, err := newNextModel(ctx, nextFlags.Last, n)
		if err != nil {
			return err
		}
		return uiutils.RunBubbleTea(&stackNext)
	},
}

func init() {
	nextCmd.Flags().BoolVar(
		&nextFlags.Last, "last", false,
		"checkout the last branch in the stack",
	)
}

type stackNextModel struct {
	currentBranch string
	db            meta.DB
	repo          *git.Repo
	help          help.Model

	selection   *selection.Model[string]
	lastInStack bool
	nInStack    int
	err         error
}

func newNextModel(ctx context.Context, lastInStack bool, nInStack int) (stackNextModel, error) {
	repo, err := getRepo(ctx)
	if err != nil {
		return stackNextModel{}, err
	}

	db, err := getDB(ctx, repo)
	if err != nil {
		return stackNextModel{}, err
	}

	currentBranch, err := repo.CurrentBranchName(ctx)
	if err != nil {
		return stackNextModel{}, err
	}

	return stackNextModel{
		currentBranch: currentBranch,
		db:            db,
		repo:          repo,
		help:          help.New(),
		nInStack:      nInStack,
		lastInStack:   lastInStack,
	}, nil
}

func (m stackNextModel) currentBranchChildren() []string {
	tx := m.db.ReadTx()
	children := meta.Children(tx, m.currentBranch)
	options := make([]string, 0, len(children))
	for _, child := range children {
		options = append(options, child.Name)
	}

	return options
}

type branchCheckedOutMsg struct{}

func (m stackNextModel) checkoutCurrentBranch() tea.Msg {
	if _, err := m.repo.CheckoutBranch(context.Background(), &git.CheckoutBranch{
		Name: m.currentBranch,
	}); err != nil {
		return err
	}

	return branchCheckedOutMsg{}
}

type (
	checkoutBranchMsg struct{}
	nextBranchMsg     struct{}
	showSelectionMsg  struct{}
)

func (m stackNextModel) nextBranch() tea.Msg {
	currentBranchChildren := m.currentBranchChildren()

	if m.lastInStack && len(currentBranchChildren) == 0 {
		return checkoutBranchMsg{}
	}

	if m.nInStack == 0 && !m.lastInStack {
		return checkoutBranchMsg{}
	}

	if m.nInStack > 0 && len(currentBranchChildren) == 0 {
		return errors.New("invalid number, already on last branch in the stack")
	}

	if len(currentBranchChildren) == 0 {
		return fmt.Errorf("there are no children of branch %s", colors.UserInput(m.currentBranch))
	}

	if len(currentBranchChildren) == 1 {
		return nextBranchMsg{}
	}

	return showSelectionMsg{}
}

func (m stackNextModel) Init() tea.Cmd {
	return m.nextBranch
}

func (vm stackNextModel) View() string {
	var ss []string
	if vm.selection != nil {
		ss = append(ss, vm.selection.View()+vm.help.ShortHelpView(uiutils.PromptKeys))
	}

	var ret string
	if len(ss) != 0 {
		ret = lipgloss.NewStyle().MarginTop(1).MarginBottom(1).MarginLeft(2).Render(
			lipgloss.JoinVertical(0, ss...),
		) + "\n"
	}
	if vm.err != nil {
		if len(ret) != 0 {
			ret += "\n"
		}
		ret += renderError(vm.err)
	}
	return ret
}

func (m stackNextModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case error:
		m.err = msg
		return m, tea.Quit
	case branchCheckedOutMsg:
		return m, tea.Quit
	case checkoutBranchMsg:
		return m, m.checkoutCurrentBranch
	case nextBranchMsg:
		m.currentBranch = m.currentBranchChildren()[0]
		m.nInStack--
		return m, m.nextBranch

	case showSelectionMsg:
		m.selection = uiutils.NewLegacyPromptModel(
			fmt.Sprintf("There are multiple children of branch %s. Which branch would you like to follow?", colors.UserInput(m.currentBranch)),
			m.currentBranchChildren(),
		)
		return m, m.selection.Init()

	case tea.KeyMsg:
		if m.selection != nil {
			switch msg.String() {
			case "enter":
				currentBranch, err := m.selection.Value()
				if err != nil {
					m.err = err
					return m, tea.Quit
				}
				m.currentBranch = currentBranch
				m.nInStack--

				return m, m.nextBranch
			case "ctrl+c":
				return m, tea.Quit
			default:
				_, cmd := m.selection.Update(msg)
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m stackNextModel) ExitError() error {
	if m.err != nil {
		return actions.ErrExitSilently{ExitCode: 1}
	}
	return nil
}
