package main

import (
	"sort"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/treedetector"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/sliceutils"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/aviator-co/av/internal/utils/uiutils"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
)

var stackAdoptFlags struct {
	Parent string
	DryRun bool
}

var stackAdoptCmd = &cobra.Command{
	Use:   "adopt",
	Short: "Adopt branches that are not managed by av",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}

		status, err := repo.Status()
		if err != nil {
			return err
		}

		currentBranch := status.CurrentBranch
		if stackAdoptFlags.Parent != "" {
			return stackAdoptForceAdoption(repo, db, currentBranch, stackAdoptFlags.Parent)
		}

		return uiutils.RunBubbleTea(&stackAdoptViewModel{
			repo:              repo,
			db:                db,
			currentHEADBranch: plumbing.NewBranchReferenceName(currentBranch),
			currentCursor:     plumbing.NewBranchReferenceName(currentBranch),
			chosenTargets:     make(map[plumbing.ReferenceName]bool),
			help:              help.New(),
			spinner:           spinner.New(spinner.WithSpinner(spinner.Dot)),
		})
	},
}

func stackAdoptForceAdoption(repo *git.Repo, db meta.DB, currentBranch, parent string) error {
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

	if isCurrentBranchTrunk, err := repo.IsTrunkBranch(currentBranch); err != nil {
		return errors.Wrap(err, "failed to check if the current branch is trunk")
	} else if isCurrentBranchTrunk {
		return errors.New("cannot adopt the default branch")
	}

	isParentBranchTrunk, err := repo.IsTrunkBranch(parent)
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
		mergeBase, err := repo.MergeBase(parent, currentBranch)
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
	if stackAdoptFlags.DryRun {
		return nil
	}
	return tx.Commit()
}

type stackAdoptTreeInfo struct {
	branches        map[plumbing.ReferenceName]*treedetector.BranchPiece
	rootNodes       []*stackutils.StackTreeNode
	adoptionTargets []plumbing.ReferenceName
}

type stackAdoptViewModel struct {
	repo              *git.Repo
	db                meta.DB
	currentHEADBranch plumbing.ReferenceName

	help               help.Model
	spinner            spinner.Model
	currentCursor      plumbing.ReferenceName
	chosenTargets      map[plumbing.ReferenceName]bool
	treeInfo           *stackAdoptTreeInfo
	adoptionComplete   bool
	adoptionInProgress bool

	err error
}

func (vm stackAdoptViewModel) Init() tea.Cmd {
	return tea.Batch(vm.spinner.Tick, vm.initCmd)
}

func (vm stackAdoptViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case error:
		vm.err = msg
		return vm, tea.Quit
	case *stackAdoptTreeInfo:
		vm.treeInfo = msg
		if len(vm.treeInfo.adoptionTargets) == 0 {
			return vm, tea.Quit
		}
		for _, branch := range vm.treeInfo.adoptionTargets {
			// By default choose everything.
			vm.chosenTargets[branch] = true
		}
		vm.currentCursor = vm.treeInfo.adoptionTargets[0]
		if stackAdoptFlags.DryRun {
			return vm, tea.Quit
		}
	case adoptionCompleteMsg:
		vm.adoptionComplete = true
		return vm, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return vm, tea.Quit
		}
		if vm.treeInfo != nil {
			switch msg.String() {
			case "up", "k":
				vm.currentCursor = vm.getPreviousBranch()
			case "down", "j":
				vm.currentCursor = vm.getNextBranch()
			case " ":
				vm.toggleAdoption(vm.currentCursor)
			case "enter":
				vm.adoptionInProgress = true
				return vm, vm.adoptBranches
			}
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		vm.spinner, cmd = vm.spinner.Update(msg)
		return vm, cmd
	}
	return vm, nil
}

func (vm stackAdoptViewModel) initCmd() tea.Msg {
	unmanagedBranches, err := vm.getUnmanagedBranches()
	if err != nil {
		return err
	}
	pieces, err := treedetector.DetectBranches(vm.repo, unmanagedBranches)
	if err != nil {
		return err
	}
	nodes := treedetector.ConvertToStackTree(vm.db, pieces, plumbing.HEAD, false)
	return &stackAdoptTreeInfo{
		branches:        pieces,
		rootNodes:       nodes,
		adoptionTargets: vm.getAdoptionTargets(nodes[0]),
	}
}

func (vm *stackAdoptViewModel) getUnmanagedBranches() ([]plumbing.ReferenceName, error) {
	tx := vm.db.ReadTx()
	adoptedBranches := tx.AllBranches()
	branches, err := vm.repo.GoGitRepo().Branches()
	if err != nil {
		return nil, err
	}
	var ret []plumbing.ReferenceName
	if err := branches.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference {
			return nil
		}
		if _, adopted := adoptedBranches[ref.Name().Short()]; !adopted {
			ret = append(ret, ref.Name())
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (vm stackAdoptViewModel) getAdoptionTargets(
	node *stackutils.StackTreeNode,
) []plumbing.ReferenceName {
	var ret []plumbing.ReferenceName
	for _, child := range node.Children {
		ret = append(ret, vm.getAdoptionTargets(child)...)
	}
	if node.Branch.ParentBranchName != "" {
		_, adopted := vm.db.ReadTx().Branch(node.Branch.BranchName)
		if !adopted {
			ret = append(ret, plumbing.NewBranchReferenceName(node.Branch.BranchName))
		}
	}
	return ret
}

func (vm stackAdoptViewModel) getPreviousBranch() plumbing.ReferenceName {
	if vm.treeInfo == nil {
		return vm.currentCursor
	}
	for i, branch := range vm.treeInfo.adoptionTargets {
		if branch == vm.currentCursor {
			if i == 0 {
				return vm.currentCursor
			}
			return vm.treeInfo.adoptionTargets[i-1]
		}
	}
	return vm.currentCursor
}

func (vm stackAdoptViewModel) getNextBranch() plumbing.ReferenceName {
	if vm.treeInfo == nil {
		return vm.currentCursor
	}
	for i, branch := range vm.treeInfo.adoptionTargets {
		if branch == vm.currentCursor {
			if i == len(vm.treeInfo.adoptionTargets)-1 {
				return vm.currentCursor
			}
			return vm.treeInfo.adoptionTargets[i+1]
		}
	}
	return vm.currentCursor
}

func (vm *stackAdoptViewModel) toggleAdoption(branch plumbing.ReferenceName) {
	if vm.treeInfo == nil {
		return
	}
	if vm.chosenTargets[branch] {
		// Going to unchoose. Unchoose all children as well.
		children := treedetector.GetChildren(vm.treeInfo.branches, branch)
		for bn := range children {
			delete(vm.chosenTargets, bn)
		}
		delete(vm.chosenTargets, branch)
	} else {
		// Going to choose. Choose all parents as well.
		piece := vm.treeInfo.branches[branch]
		for {
			if !sliceutils.Contains(vm.treeInfo.adoptionTargets, piece.Name) {
				break
			}
			vm.chosenTargets[piece.Name] = true
			if piece.Parent == "" || piece.ParentIsTrunk {
				break
			}
			piece = vm.treeInfo.branches[piece.Parent]
		}
	}
}

type adoptionCompleteMsg struct{}

func (vm stackAdoptViewModel) adoptBranches() tea.Msg {
	tx := vm.db.WriteTx()
	for branch := range vm.chosenTargets {
		piece := vm.treeInfo.branches[branch]
		bi, _ := tx.Branch(branch.Short())
		bi.Parent = meta.BranchState{
			Name:  piece.Parent.Short(),
			Trunk: piece.ParentIsTrunk,
		}
		if !piece.ParentIsTrunk {
			bi.Parent.Head = piece.ParentMergeBase.String()
		}
		tx.SetBranch(bi)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return adoptionCompleteMsg{}
}

func (vm stackAdoptViewModel) View() string {
	var ss []string
	if vm.treeInfo != nil {
		choosing := false
		if len(vm.treeInfo.adoptionTargets) == 0 {
			ss = append(ss, colors.SuccessStyle.Render("✓ No branch to adopt"))
		} else if vm.adoptionComplete {
			ss = append(ss, colors.SuccessStyle.Render("✓ Adoption complete"))
		} else if vm.adoptionInProgress {
			ss = append(ss, colors.ProgressStyle.Render(vm.spinner.View()+"Adopting the chosen branches..."))
		} else {
			choosing = true
			ss = append(ss, colors.QuestionStyle.Render("Choose which branches to adopt"))
		}
		for _, rootNode := range vm.treeInfo.rootNodes {
			ss = append(ss, "")
			ss = append(
				ss,
				stackutils.RenderTree(
					rootNode,
					func(branchName string, isTrunk bool) string {
						bn := plumbing.NewBranchReferenceName(branchName)
						out := vm.renderBranch(bn, isTrunk)
						if choosing && bn == vm.currentCursor {
							out = strings.TrimSuffix(out, "\n")
							out = colors.PromptChoice.Render(out)
						}
						return out
					},
				),
			)
		}
		if choosing {
			ss = append(ss, "")
			ss = append(ss, vm.help.ShortHelpView(promptKeys))
		}
	}
	if vm.adoptionInProgress || vm.adoptionComplete {
		ss = append(ss, "")
		var branches []plumbing.ReferenceName
		for branch := range vm.chosenTargets {
			branches = append(branches, branch)
		}
		sort.Slice(branches, func(i, j int) bool {
			return branches[i] < branches[j]
		})
		if len(branches) == 0 {
			ss = append(ss, "No branch is adopted")
		} else if vm.adoptionComplete {
			ss = append(ss, "Adopted the following branches:")
			ss = append(ss, "")
		} else if vm.adoptionInProgress {
			ss = append(ss, "Adopting the following branches:")
			ss = append(ss, "")
		}
		for _, branch := range branches {
			ss = append(ss, "  "+branch.Short())
			piece := vm.treeInfo.branches[branch]
			for _, c := range piece.IncludedCommits {
				title, _, _ := strings.Cut(c.Message, "\n")
				ss = append(ss, "    "+title)
			}
		}
	}

	var ret string
	if len(ss) != 0 {
		ret = lipgloss.NewStyle().MarginTop(1).MarginBottom(1).MarginLeft(2).Render(
			lipgloss.JoinVertical(0, ss...),
		) + "\n"
	}
	if vm.err != nil {
		ret += renderError(vm.err)
	}
	return ret
}

func (vm stackAdoptViewModel) ExitError() error {
	if vm.err != nil {
		return actions.ErrExitSilently{ExitCode: 1}
	}
	return nil
}

func (vm stackAdoptViewModel) renderBranch(branch plumbing.ReferenceName, isTrunk bool) string {
	if isTrunk {
		return branch.Short()
	}
	_, adopted := vm.db.ReadTx().Branch(branch.Short())

	sb := strings.Builder{}
	if adopted && !vm.chosenTargets[branch] {
		sb.WriteString(branch.Short())
	} else if vm.chosenTargets[branch] {
		sb.WriteString("[✓] " + branch.Short())
	} else {
		sb.WriteString("[ ] " + branch.Short())
	}

	var status []string
	if vm.currentHEADBranch == branch {
		status = append(status, "HEAD")
	}
	if len(status) != 0 {
		sb.WriteString(" (" + strings.Join(status, ", ") + ")")
	}
	if !adopted || vm.chosenTargets[branch] {
		sb.WriteString("\n")
		piece := vm.treeInfo.branches[branch]
		for _, c := range piece.IncludedCommits {
			title, _, _ := strings.Cut(c.Message, "\n")
			sb.WriteString("  " + title + "\n")
		}
	}
	return sb.String()
}

func init() {
	stackAdoptCmd.Flags().StringVar(
		&stackAdoptFlags.Parent, "parent", "",
		"force specifying the parent branch",
	)
	stackAdoptCmd.Flags().BoolVar(
		&stackAdoptFlags.DryRun, "dry-run", false,
		"dry-run adoption",
	)

	_ = stackSyncCmd.RegisterFlagCompletionFunc(
		"parent",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			branches, _ := allBranches()
			return branches, cobra.ShellCompDirectiveDefault
		},
	)
}

var promptKeys = []key.Binding{
	key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	key.NewBinding(
		key.WithKeys("space"),
		key.WithHelp("space", "select / unselect"),
	),
	key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "adopt selected branches"),
	),
	key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "cancel"),
	),
}
