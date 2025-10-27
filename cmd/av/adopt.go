package main

import (
	"context"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/treedetector"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/aviator-co/av/internal/utils/uiutils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
)

var adoptFlags struct {
	Parent string
	DryRun bool
}

var adoptCmd = &cobra.Command{
	Use:   "adopt",
	Short: "Adopt branches that are not managed by av",
	Long: strings.TrimSpace(`
Adopt branches that are not managed by av.

This command will show a list of branches that are not managed by av. You can choose which branches
should be adopted to av.

If you want to adopt the current branch, you can use the --parent flag to specify the parent branch.
For example, "av adopt --parent main" will adopt the current branch with the main branch as
the parent.`),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
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
		if adoptFlags.Parent != "" {
			return adoptForceAdoption(ctx, repo, db, currentBranch, adoptFlags.Parent)
		}

		return uiutils.RunBubbleTea(&adoptViewModel{
			repo:              repo,
			db:                db,
			currentHEADBranch: plumbing.NewBranchReferenceName(currentBranch),
		})
	},
}

func adoptForceAdoption(
	ctx context.Context,
	repo *git.Repo,
	db meta.DB,
	currentBranch, parent string,
) error {
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

	if isCurrentBranchTrunk, err := repo.IsTrunkBranch(ctx, currentBranch); err != nil {
		return errors.Wrap(err, "failed to check if the current branch is trunk")
	} else if isCurrentBranchTrunk {
		return errors.New("cannot adopt the default branch")
	}

	isParentBranchTrunk, err := repo.IsTrunkBranch(ctx, parent)
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
		mergeBase, err := repo.MergeBase(ctx, parent, currentBranch)
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
	if adoptFlags.DryRun {
		return nil
	}
	return tx.Commit()
}

type adoptViewModel struct {
	repo              *git.Repo
	db                meta.DB
	currentHEADBranch plumbing.ReferenceName
	branches          map[plumbing.ReferenceName]*treedetector.BranchPiece

	uiutils.BaseStackedView
}

func (vm *adoptViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return vm, vm.BaseStackedView.Update(msg)
}

func (vm *adoptViewModel) Init() tea.Cmd {
	return vm.initModel()
}

func (vm *adoptViewModel) initModel() tea.Cmd {
	return vm.AddView(
		actions.NewFindAdoptableLocalBranchesModel(
			vm.repo,
			vm.db,
			vm.initTreeSelector,
		),
	)
}

func (vm *adoptViewModel) initTreeSelector(branches map[plumbing.ReferenceName]*treedetector.BranchPiece, rootNodes []*stackutils.StackTreeNode, adoptionTargets []plumbing.ReferenceName) tea.Cmd {
	if branches == nil || len(rootNodes) == 0 || len(adoptionTargets) == 0 {
		return tea.Batch(
			vm.AddView(uiutils.SimpleMessageView{Message: colors.SuccessStyle.Render("✓ No branch to adopt")}),
			tea.Quit,
		)
	}
	vm.branches = branches
	cmd := vm.AddView(
		actions.NewAdoptTreeSelectorModel(
			vm.db,
			branches,
			rootNodes,
			adoptionTargets,
			vm.currentHEADBranch,
			vm.initAdoption,
		),
	)
	if adoptFlags.DryRun {
		return tea.Batch(
			cmd,
			vm.AddView(uiutils.SimpleMessageView{Message: colors.SuccessStyle.Render("✓ Running as dry-run. Quitting without adopting branches.")}),
			tea.Quit,
		)
	}
	return cmd
}

func (vm *adoptViewModel) initAdoption(chosenTargets []plumbing.ReferenceName) tea.Cmd {
	if len(chosenTargets) == 0 {
		return tea.Batch(
			vm.AddView(uiutils.SimpleMessageView{Message: colors.SuccessStyle.Render("✓ No branch adopted")}),
			tea.Quit,
		)
	}
	return vm.AddView(
		actions.NewAdoptBranchesModel(
			vm.db,
			chosenTargets,
			vm.branches,
			func() tea.Cmd { return tea.Quit },
		),
	)
}

func (vm *adoptViewModel) ExitError() error {
	if vm.Err != nil {
		return actions.ErrExitSilently{ExitCode: 1}
	}
	return nil
}

func init() {
	adoptCmd.Flags().StringVar(
		&adoptFlags.Parent, "parent", "",
		"force specifying the parent branch",
	)
	adoptCmd.Flags().BoolVar(
		&adoptFlags.DryRun, "dry-run", false,
		"dry-run adoption",
	)

	_ = adoptCmd.RegisterFlagCompletionFunc(
		"parent",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			branches, _ := allBranches(cmd.Context())
			return branches, cobra.ShellCompDirectiveNoFileComp
		},
	)
}
