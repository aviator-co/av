package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/uiutils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/shurcooL/githubv4"
	"github.com/spf13/cobra"
)

var squashCmd = &cobra.Command{
	Use:   "squash",
	Short: "Squash commits of the current branch into a single commit",
	Args:  cobra.NoArgs,
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

		viewModel := &squashViewModel{
			repo: repo,
			db:   db,
		}

		if err := uiutils.RunBubbleTea(viewModel); err != nil {
			return err
		}

		fmt.Fprintln(
			os.Stdout,
			colors.Success(
				fmt.Sprintf(
					"Successfully squashed %d commits.",
					viewModel.squashedCommitCount,
				),
			),
		)

		return runPostCommitRestack(repo, db)
	},
}

type squashViewModel struct {
	repo *git.Repo
	db   meta.DB

	spinner             spinner.Model
	squashing           bool
	squashedCommitCount int
	err                 error
	done                bool
}

func (vm *squashViewModel) Init() tea.Cmd {
	vm.spinner = spinner.New(spinner.WithSpinner(spinner.Dot))
	return tea.Batch(vm.spinner.Tick, vm.runSquash)
}

func (vm *squashViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if vm.squashing {
			var cmd tea.Cmd
			vm.spinner, cmd = vm.spinner.Update(msg)
			return vm, cmd
		}
		return vm, nil

	case squashDoneMsg:
		vm.squashing = false
		vm.squashedCommitCount = int(msg)
		vm.done = true
		return vm, tea.Quit

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return vm, tea.Quit
		}

	case error:
		vm.err = msg
		return vm, tea.Quit
	}

	return vm, nil
}

func (vm *squashViewModel) View() string {
	if vm.done {
		if vm.err != nil {
			return vm.err.Error()
		}
		return ""
	}

	sb := strings.Builder{}

	if vm.squashing {
		sb.WriteString(colors.ProgressStyle.Render(vm.spinner.View() + "Squashing commits..."))
		sb.WriteString("\n")
	}

	if vm.err != nil {
		sb.WriteString(colors.Failure(vm.err.Error()))
		sb.WriteString("\n")
	}

	return sb.String()
}

func (vm *squashViewModel) ExitError() error {
	if vm.err != nil {
		return actions.ErrExitSilently{ExitCode: 1}
	}
	return nil
}

type squashDoneMsg int

func (vm *squashViewModel) runSquash() tea.Msg {
	vm.squashing = true
	count, err := runSquash(context.Background(), vm.repo, vm.db)
	if err != nil {
		return err
	}
	return squashDoneMsg(count)
}

func runSquash(ctx context.Context, repo *git.Repo, db meta.DB) (int, error) {
	status, err := repo.Status(ctx)
	if err != nil {
		return 0, errors.Errorf("cannot get the status of the repository: %v", err)
	}

	if !status.IsClean() {
		return 0, errors.New(
			"the working directory is not clean, please stash or commit them before running squash command.",
		)
	}

	currentBranchName, err := repo.CurrentBranchName(ctx)
	if err != nil {
		return 0, err
	}

	tx := db.WriteTx()
	defer tx.Abort()

	branch, branchExists := tx.Branch(currentBranchName)
	if !branchExists {
		return 0, errors.New("current branch does not exist in the database")
	}

	if branch.PullRequest != nil &&
		branch.PullRequest.State == githubv4.PullRequestStateMerged {
		return 0, errors.New("this branch has already been merged, squashing is not allowed")
	}

	commitIDs, err := repo.RevList(ctx, git.RevListOpts{
		Specifiers: []string{currentBranchName, "^" + branch.Parent.Name},
		Reverse:    true,
	})
	if err != nil {
		return 0, err
	}

	if len(commitIDs) <= 1 {
		return 0, errors.New("no commits to squash")
	}

	firstCommitSha := commitIDs[0]

	if _, err := repo.Git(ctx, "reset", "--soft", firstCommitSha); err != nil {
		return 0, err
	}

	_, err = repo.Git(ctx, "commit", "--amend", "--no-edit")
	if err != nil {
		return 0, err
	}

	return len(commitIDs), nil
}
