package main

import (
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/stackutils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var stackSwitchCmd = &cobra.Command{
	Use:   "switch",
	Short: "switch to a different branch",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}
		tx := db.ReadTx()

		var currentBranch string
		if dh, err := repo.DetachedHead(); err != nil {
			return err
		} else if !dh {
			currentBranch, err = repo.CurrentBranchName()
			if err != nil {
				return err
			}
		}

		rootNodes := stackutils.BuildStackTreeAllBranches(tx, currentBranch, true)
		var branchList []*stackTreeBranchInfo
		branches := map[string]*stackTreeBranchInfo{}
		for _, node := range rootNodes {
			branchList = append(branchList, stackSwitchBranchList(repo, tx, branches, node)...)
		}

		if !isatty.IsTerminal(os.Stdout.Fd()) {
			return errors.New("stack switch command must be run in a terminal")
		}
		p := tea.NewProgram(stackSwitchViewModel{
			repo:                 repo,
			currentHEADBranch:    currentBranch,
			currentChoosenBranch: currentBranch,
			rootNodes:            rootNodes,
			branchList:           branchList,
			branches:             branches,
		})
		model, err := p.Run()
		if err != nil {
			return err
		}
		if err := model.(stackSwitchViewModel).checkoutError; err != nil {
			return actions.ErrExitSilently{ExitCode: 1}
		}
		return nil
	},
}

func stackSwitchBranchList(repo *git.Repo, tx meta.ReadTx, branches map[string]*stackTreeBranchInfo, node *stackutils.StackTreeNode) []*stackTreeBranchInfo {
	var ret []*stackTreeBranchInfo
	for _, child := range node.Children {
		ret = append(ret, stackSwitchBranchList(repo, tx, branches, child)...)
	}
	stbi := getStackTreeBranchInfo(repo, tx, node.Branch.BranchName)
	branches[node.Branch.BranchName] = stbi
	if !stbi.Deleted {
		ret = append(ret, stbi)
	}
	return ret
}

var stackSwitchStackBranchInfoStyles = stackBranchInfoStyles{
	BranchName:      lipgloss.NewStyle().Bold(true).Foreground(colors.Green600),
	HEAD:            lipgloss.NewStyle().Bold(true).Foreground(colors.Cyan600),
	Deleted:         lipgloss.NewStyle().Bold(true).Foreground(colors.Red700),
	NeedSync:        lipgloss.NewStyle().Bold(true).Foreground(colors.Red700),
	PullRequestLink: lipgloss.NewStyle().Foreground(colors.Black),
}

type stackSwitchViewModel struct {
	currentChoosenBranch string
	checkingOut          bool
	checkoutError        error

	repo              *git.Repo
	currentHEADBranch string
	rootNodes         []*stackutils.StackTreeNode
	branchList        []*stackTreeBranchInfo
	branches          map[string]*stackTreeBranchInfo
}

func (vm stackSwitchViewModel) Init() tea.Cmd {
	return nil
}

type checkoutErrMsg error

func (vm stackSwitchViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case checkoutErrMsg:
		vm.checkoutError = msg
		return vm, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return vm, tea.Quit
		case "up", "k":
			vm.currentChoosenBranch = vm.getPreviousBranch()
		case "down", "j":
			vm.currentChoosenBranch = vm.getNextBranch()
		case "enter", " ":
			vm.checkingOut = true
			return vm, vm.checkoutBranch
		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return vm, nil
}

func (vm stackSwitchViewModel) checkoutBranch() tea.Msg {
	if vm.currentChoosenBranch != vm.currentHEADBranch {
		if _, err := vm.repo.CheckoutBranch(&git.CheckoutBranch{
			Name: vm.currentChoosenBranch,
		}); err != nil {
			return checkoutErrMsg(err)
		}
	}
	return tea.QuitMsg{}
}

func (vm stackSwitchViewModel) getPreviousBranch() string {
	for i, branch := range vm.branchList {
		if branch.BranchName == vm.currentChoosenBranch {
			if i == 0 {
				return vm.currentChoosenBranch
			}
			return vm.branchList[i-1].BranchName
		}
	}
	return vm.currentChoosenBranch
}

func (vm stackSwitchViewModel) getNextBranch() string {
	for i, branch := range vm.branchList {
		if branch.BranchName == vm.currentChoosenBranch {
			if i == len(vm.branchList)-1 {
				return vm.currentChoosenBranch
			}
			return vm.branchList[i+1].BranchName
		}
	}
	return vm.currentChoosenBranch
}

func (vm stackSwitchViewModel) View() string {
	sb := strings.Builder{}
	for _, node := range vm.rootNodes {
		sb.WriteString(stackutils.RenderTree(node, func(branchName string, isTrunk bool) string {
			stbi := vm.branches[branchName]
			if branchName == vm.currentChoosenBranch {
				out := strings.TrimSuffix(renderStackTreeBranchInfo(stackSwitchStackBranchInfoStyles, stbi, vm.currentHEADBranch, branchName, isTrunk), "\n")
				out = lipgloss.NewStyle().Background(colors.Slate300).Render(out)
				return out
			}
			return renderStackTreeBranchInfo(stackTreeStackBranchInfoStyles, stbi, vm.currentHEADBranch, branchName, isTrunk)
		}))
	}
	if vm.checkingOut {
		sb.WriteString("Checking out branch " + vm.currentChoosenBranch + "...\n")
	}
	if vm.checkoutError != nil {
		sb.WriteString(vm.checkoutError.Error())
	}
	return sb.String()
}
