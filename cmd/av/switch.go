package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/aviator-co/av/internal/utils/uiutils"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var switchCmd = &cobra.Command{
	Use:               "switch [<branch> | <url>]",
	Short:             "Interactively switch to a different branch",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: branchNameArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		tx := db.ReadTx()
		if len(args) > 0 {
			branch, err := parseBranchName(tx, args[0])
			if err != nil {
				return err
			}
			if _, err := repo.CheckoutBranch(ctx, &git.CheckoutBranch{Name: branch}); err != nil {
				return err
			}
			return nil
		}

		rootNodes := stackutils.BuildStackTreeAllBranches(tx, currentBranch, true)
		var branchList []*stackTreeBranchInfo
		branches := map[string]*stackTreeBranchInfo{}
		for _, node := range rootNodes {
			branchList = append(
				branchList,
				switchBranchList(ctx, repo, tx, branches, node)...,
			)
		}
		if len(branchList) == 0 {
			return errors.New("no branches found")
		}

		// Remove deleted branches from the tree so they don't show
		// up in the interactive view.
		rootNodes = pruneDeletedBranches(rootNodes, branches)

		if !isatty.IsTerminal(os.Stdout.Fd()) {
			return errors.New("switch command must be run in a terminal")
		}
		return uiutils.RunBubbleTea(&switchViewModel{
			repo:                repo,
			help:                help.New(),
			currentHEADBranch:   currentBranch,
			currentChosenBranch: getInitialChosenBranch(branchList, currentBranch),
			rootNodes:           rootNodes,
			branchList:          branchList,
			branches:            branches,
			spinner:             spinner.New(spinner.WithSpinner(spinner.Dot)),
		})
	},
}

func getInitialChosenBranch(branchList []*stackTreeBranchInfo, currentBranch string) string {
	for _, branch := range branchList {
		if branch.BranchName == currentBranch {
			return currentBranch
		}
	}
	// If the current branch is not in the list, choose the first branch
	return branchList[0].BranchName
}

func switchBranchList(
	ctx context.Context,
	repo *git.Repo,
	tx meta.ReadTx,
	branches map[string]*stackTreeBranchInfo,
	node *stackutils.StackTreeNode,
) []*stackTreeBranchInfo {
	var ret []*stackTreeBranchInfo
	for _, child := range node.Children {
		ret = append(ret, switchBranchList(ctx, repo, tx, branches, child)...)
	}
	stbi := getStackTreeBranchInfo(ctx, repo, tx, node.Branch.BranchName)
	branches[node.Branch.BranchName] = stbi
	if !stbi.Deleted {
		ret = append(ret, stbi)
	}
	return ret
}

func parseBranchName(tx meta.ReadTx, input string) (string, error) {
	if branch, err := parsePullRequestURL(tx, input); err == nil {
		return branch, nil
	}

	return input, nil
}

var PULL_REQUEST_URL_REGEXP = regexp.MustCompile(`^/([^/]+)/([^/]+)/pull/(\d+)`)

func parsePullRequestURL(tx meta.ReadTx, prURL string) (string, error) {
	u, err := url.Parse(prURL)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse URL")
	}

	if u.Scheme != "https" && u.Scheme != "http" {
		return "", errors.New("URL is not a pull request URL")
	}

	m := PULL_REQUEST_URL_REGEXP.FindStringSubmatch(u.Path)
	if m == nil {
		return "", errors.New(fmt.Sprintf("URL is not a pull request URL format:%s", prURL))
	}

	prNumber, err := strconv.Atoi(m[3])
	if err != nil {
		return "", errors.Wrap(err, "failed to parse pull request ID")
	}

	branches := tx.AllBranches()
	for _, branch := range branches {
		if branch.PullRequest != nil && branch.PullRequest.GetNumber() == int64(prNumber) {
			return branch.Name, nil
		}
	}

	return "", fmt.Errorf("failed to detect branch from pull request URL:%s", prURL)
}

var switchKeys = append(
	slices.Clone(uiutils.PromptKeys[:3]),
	key.NewBinding(
		key.WithKeys("esc", "ctrl+c"),
		key.WithHelp("esc", "cancel"),
	),
)

type switchViewModel struct {
	currentChosenBranch string
	checkingOut         bool
	checkedOut          bool
	err                 error
	help                help.Model
	spinner             spinner.Model

	repo              *git.Repo
	currentHEADBranch string
	rootNodes         []*stackutils.StackTreeNode
	branchList        []*stackTreeBranchInfo
	branches          map[string]*stackTreeBranchInfo

	height       int
	scrollOffset int
}

func (vm switchViewModel) Init() tea.Cmd {
	return vm.spinner.Tick
}

type checkoutDoneMsg struct{}

func (vm switchViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case error:
		vm.err = msg
		return vm, tea.Quit
	case checkoutDoneMsg:
		vm.checkingOut = false
		vm.checkedOut = true
		return vm, tea.Quit
	case tea.WindowSizeMsg:
		vm.height = msg.Height
		vm.scrollOffset = vm.adjustScrollOffset()
	case tea.KeyPressMsg:
		if !vm.checkingOut && !vm.checkedOut {
			switch msg.String() {
			case "ctrl+c", "esc":
				return vm, tea.Quit
			case "up", "k", "ctrl+p":
				vm.currentChosenBranch = vm.getPreviousBranch()
				vm.scrollOffset = vm.adjustScrollOffset()
			case "down", "j", "ctrl+n":
				vm.currentChosenBranch = vm.getNextBranch()
				vm.scrollOffset = vm.adjustScrollOffset()
			case "enter", "space":
				vm.checkingOut = true
				return vm, vm.checkoutBranch
			}
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		vm.spinner, cmd = vm.spinner.Update(msg)
		return vm, cmd
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return vm, nil
}

func (vm switchViewModel) checkoutBranch() tea.Msg {
	if vm.currentChosenBranch != vm.currentHEADBranch {
		if _, err := vm.repo.CheckoutBranch(context.Background(), &git.CheckoutBranch{
			Name: vm.currentChosenBranch,
		}); err != nil {
			return err
		}
	}
	return checkoutDoneMsg{}
}

func (vm switchViewModel) getPreviousBranch() string {
	for i, branch := range vm.branchList {
		if branch.BranchName == vm.currentChosenBranch {
			if i == 0 {
				return vm.currentChosenBranch
			}
			return vm.branchList[i-1].BranchName
		}
	}
	return vm.currentChosenBranch
}

func (vm switchViewModel) getNextBranch() string {
	for i, branch := range vm.branchList {
		if branch.BranchName == vm.currentChosenBranch {
			if i == len(vm.branchList)-1 {
				return vm.currentChosenBranch
			}
			return vm.branchList[i+1].BranchName
		}
	}
	return vm.currentChosenBranch
}

func (vm switchViewModel) View() tea.View {
	treeLines, _ := vm.renderTree()
	if visible := vm.visibleTreeHeight(len(treeLines)); visible < len(treeLines) {
		offset := min(max(vm.scrollOffset, 0), len(treeLines)-visible)
		treeLines = treeLines[offset : offset+visible]
	}
	ret := vm.renderFrame(treeLines)
	if vm.err != nil {
		ret += uiutils.RenderError(vm.err)
	}
	return tea.NewView(ret)
}

func (vm switchViewModel) renderFrame(treeLines []string) string {
	var ss []string
	if vm.checkingOut {
		ss = append(
			ss,
			colors.ProgressStyle.Render(vm.spinner.View()+"Checking out the chosen branch..."),
		)
	} else if vm.checkedOut {
		ss = append(ss, colors.SuccessStyle.Render("✓ Checked out branch"))
	} else {
		ss = append(ss, colors.QuestionStyle.Render("Choose which branch to check out"))
	}
	ss = append(ss, "")
	ss = append(ss, treeLines...)
	ss = append(ss, "")
	if vm.checkingOut {
		ss = append(ss, "Checking out branch "+vm.currentChosenBranch+"...")
	} else if vm.checkedOut {
		ss = append(ss, "Checked out branch "+vm.currentChosenBranch)
	} else {
		ss = append(ss, vm.help.ShortHelpView(switchKeys))
	}
	return lipgloss.NewStyle().MarginTop(1).MarginBottom(1).MarginLeft(2).Render(
		lipgloss.JoinVertical(0, ss...),
	) + "\n"
}

func (vm switchViewModel) renderTree() ([]string, map[string]stackutils.LineRange) {
	var lines []string
	ranges := map[string]stackutils.LineRange{}
	for _, node := range vm.rootNodes {
		out, nodeRanges := stackutils.RenderTreeWithLineRanges(
			node,
			func(branchName string, isTrunk bool) string {
				stbi := vm.branches[branchName]
				out := vm.renderBranchInfo(
					stbi,
					vm.currentHEADBranch,
					branchName,
					isTrunk,
				)
				if branchName == vm.currentChosenBranch {
					out = colors.PromptChoice.Render(out)
				}
				return out
			},
		)
		for name, r := range nodeRanges {
			ranges[name] = stackutils.LineRange{Start: len(lines) + r.Start, End: len(lines) + r.End}
		}
		lines = append(lines, strings.Split(out, "\n")...)
	}
	return lines, ranges
}

func (vm switchViewModel) visibleTreeHeight(treeHeight int) int {
	if vm.height <= 0 {
		return treeHeight
	}
	chrome := lipgloss.Height(vm.renderFrame(nil))
	return min(treeHeight, max(vm.height-chrome, 1))
}

func (vm switchViewModel) adjustScrollOffset() int {
	lines, ranges := vm.renderTree()
	visible := vm.visibleTreeHeight(len(lines))
	offset := vm.scrollOffset
	if r, ok := ranges[vm.currentChosenBranch]; ok {
		if r.End > offset+visible {
			offset = r.End - visible
		}
		if r.Start < offset {
			offset = r.Start
		}
	}
	return min(max(offset, 0), len(lines)-visible)
}

func (switchViewModel) renderBranchInfo(
	stbi *stackTreeBranchInfo,
	currentBranchName string,
	branchName string,
	isTrunk bool,
) string {
	var stats []string
	if branchName == currentBranchName {
		stats = append(stats, "HEAD")
	}
	line := branchName
	if len(stats) > 0 {
		line += " (" + strings.Join(stats, ", ") + ")"
	}

	var ss []string
	ss = append(ss, line)
	if !isTrunk {
		if stbi.PullRequestLink != "" {
			ss = append(ss, stbi.PullRequestLink)
		} else {
			ss = append(ss, "No pull request")
		}
	}
	return strings.Join(ss, "\n")
}

// pruneDeletedBranches removes deleted branches from the tree, promoting their
// children to the parent node.
func pruneDeletedBranches(
	nodes []*stackutils.StackTreeNode,
	branches map[string]*stackTreeBranchInfo,
) []*stackutils.StackTreeNode {
	var result []*stackutils.StackTreeNode
	for _, node := range nodes {
		node.Children = pruneDeletedBranches(node.Children, branches)
		stbi := branches[node.Branch.BranchName]
		if stbi != nil && stbi.Deleted {
			// Skip this node but keep its children.
			result = append(result, node.Children...)
		} else {
			result = append(result, node)
		}
	}
	return result
}

func (vm switchViewModel) ExitError() error {
	if vm.err != nil {
		return actions.ErrExitSilently{ExitCode: 1}
	}
	return nil
}
