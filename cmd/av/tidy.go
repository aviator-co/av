package main

import (
	"context"
	"strings"

	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/gh/ghui"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gitui"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/uiutils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
)

var tidyCmd = &cobra.Command{
	Use:   "tidy",
	Short: "Tidy stacked branches",
	Long: strings.TrimSpace(`
Tidy stacked branches by removing deleted or merged branches.

This command detects which branches are deleted or merged and re-parents
children of merged branches. This operates on only av's internal metadata and
does not delete Git branches.
`),
	SilenceUsage: true,
	Args:         cobra.NoArgs,
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

		client, err := getGitHubClient(ctx)
		if err != nil {
			return err
		}

		return uiutils.RunBubbleTea(&tidyViewModel{
			repo:   repo,
			db:     db,
			client: client,
		})
	},
}

type tidyViewModel struct {
	repo   *git.Repo
	db     meta.DB
	client *gh.Client

	deleted  map[string]bool
	orphaned map[string]bool

	uiutils.BaseStackedView
}

func (vm *tidyViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return vm, vm.BaseStackedView.Update(msg)
}

func (vm *tidyViewModel) Init() tea.Cmd {
	return vm.initGitHubFetch()
}

func (vm *tidyViewModel) initGitHubFetch() tea.Cmd {
	ctx := context.Background()
	status, err := vm.repo.Status(ctx)
	if err != nil {
		return uiutils.ErrCmd(err)
	}
	currentBranch := status.CurrentBranch

	var targetBranches []plumbing.ReferenceName
	branches := vm.db.ReadTx().AllBranches()
	for name := range branches {
		targetBranches = append(targetBranches, plumbing.NewBranchReferenceName(name))
	}

	var currentBranchRef plumbing.ReferenceName
	if currentBranch != "" {
		currentBranchRef = plumbing.NewBranchReferenceName(currentBranch)
	}

	return vm.AddView(ghui.NewGitHubFetchModel(
		vm.repo,
		vm.db,
		vm.client,
		currentBranchRef,
		targetBranches,
		vm.initTidyDB,
	))
}

func (vm *tidyViewModel) initTidyDB() tea.Cmd {
	ctx := context.Background()
	deleted, orphaned, err := actions.TidyDB(ctx, vm.repo, vm.db)
	if err != nil {
		return uiutils.ErrCmd(err)
	}

	vm.deleted = deleted
	vm.orphaned = orphaned

	return vm.initPruneBranches()
}

func (vm *tidyViewModel) initPruneBranches() tea.Cmd {
	ctx := context.Background()
	status, err := vm.repo.Status(ctx)
	if err != nil {
		return uiutils.ErrCmd(err)
	}
	currentBranch := status.CurrentBranch

	var targetBranches []plumbing.ReferenceName
	branches := vm.db.ReadTx().AllBranches()
	for name, branch := range branches {
		if branch.MergeCommit != "" {
			targetBranches = append(targetBranches, plumbing.NewBranchReferenceName(name))
		}
	}

	return vm.AddView(gitui.NewPruneBranchModel(
		vm.repo,
		vm.db,
		"ask",
		targetBranches,
		currentBranch,
		func() tea.Cmd {
			return tea.Quit
		},
	))
}

func (vm *tidyViewModel) ExitError() error {
	if vm.Err != nil {
		return actions.ErrExitSilently{ExitCode: 1}
	}
	return nil
}
