package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/cleanup"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	// This is an arbitrary limit on the branch name length.
	branchNameLength = 200
)

var (
	// See man 1 git-check-ref-format for the refname spec. This pattern is more restrictive
	// than the spec.
	//
	// * Do not allow slashes because creating a branch directory from a commit message is
	//   unlikely.
	// * Do not allow dots because dots cannot be placed on a certain location and it's unlikely
	//   the user wants to have a dot in the branch name.
	branchNameReplacedPattern = regexp.MustCompile("[^-_a-zA-Z0-9]")

	multipleSpacePattern = regexp.MustCompile(" +")
)

var commitFlags struct {
	Message      []string
	All          bool
	Amend        bool
	Edit         bool
	CreateBranch bool
	BranchName   string
	AllChanges   bool
	Parent       string
}

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Record changes to the repository with commits",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		if commitFlags.Amend {
			if commitFlags.CreateBranch || commitFlags.BranchName != "" {
				return errors.New("cannot create a branch and amend at the same time")
			}
			return amendCmd(
				ctx,
				strings.Join(commitFlags.Message, "\n\n"),
				commitFlags.Edit,
				commitFlags.All,
			)
		}

		if commitFlags.CreateBranch || commitFlags.BranchName != "" {
			return branchAndCommit(
				ctx,
				commitFlags.BranchName,
				strings.Join(commitFlags.Message, "\n\n"),
				commitFlags.AllChanges,
				commitFlags.All,
				commitFlags.Parent,
			)
		}

		if commitFlags.Parent != "" {
			return errors.New("parent flag is only allowed with -b/--branch or --branch-name")
		}

		repo, err := getRepo(ctx)
		if err != nil {
			return err
		}

		db, err := getDB(ctx, repo)
		if err != nil {
			return err
		}

		// We need to run git commit before bubbletea grabs the terminal. Otherwise,
		// we need to make p.ReleaseTerminal() and p.RestoreTerminal().
		if err := runCreate(ctx, repo, db); err != nil {
			fmt.Fprint(os.Stderr, "\n", colors.Failure("Failed to create commit."), "\n")
			return actions.ErrExitSilently{ExitCode: 1}
		}

		return runPostCommitRestack(repo, db)
	},
}

func runCreate(ctx context.Context, repo *git.Repo, db meta.DB) error {
	currentBranch, err := repo.CurrentBranchName()
	if err != nil {
		return errors.WrapIf(err, "failed to determine current branch")
	}

	// Handle "--all-changes"
	if commitFlags.AllChanges {
		_, err := repo.Run(ctx, &git.RunOpts{
			Args:      []string{"add", "--all"},
			ExitError: true,
		})
		if err != nil {
			fmt.Fprint(os.Stderr,
				"\n", colors.Failure("Failed to stage files: ", err.Error()), "\n",
			)
			return actions.ErrExitSilently{ExitCode: 1}
		}
	}

	commitArgs := []string{"commit"}
	if commitFlags.All {
		commitArgs = append(commitArgs, "--all")
	}
	if len(commitFlags.Message) > 0 {
		commitArgs = append(commitArgs, "--message", strings.Join(commitFlags.Message, "\n\n"))
	}

	tx := db.WriteTx()
	defer tx.Abort()

	branch, _ := tx.Branch(currentBranch)
	if branch.PullRequest != nil && branch.PullRequest.State == githubv4.PullRequestStateMerged {
		fmt.Fprint(
			os.Stderr,
			colors.Failure("This branch has already been merged, commit is not allowed"),
			"\n",
		)
		return errors.New("this branch has already been merged, commit is not allowed")
	}

	if _, err := repo.Run(ctx, &git.RunOpts{
		Args:        commitArgs,
		ExitError:   true,
		Interactive: true,
	}); err != nil {
		return err
	}
	return nil
}

func amendCmd(ctx context.Context, message string, edit bool, all bool) error {
	repo, err := getRepo(ctx)
	if err != nil {
		return err
	}

	db, err := getDB(ctx, repo)
	if err != nil {
		return err
	}

	// We need to run git commit --amend before bubbletea grabs the terminal. Otherwise,
	// we need to make p.ReleaseTerminal() and p.RestoreTerminal().
	if err := runAmend(ctx, repo, db, message, edit, all); err != nil {
		fmt.Fprint(os.Stderr, "\n", colors.Failure("Failed to amend."), "\n")
		return actions.ErrExitSilently{ExitCode: 1}
	}

	return runPostCommitRestack(repo, db)
}

func runAmend(
	ctx context.Context,
	repo *git.Repo,
	db meta.DB,
	message string,
	edit bool,
	all bool,
) error {
	currentBranch, err := repo.CurrentBranchName()
	if err != nil {
		return errors.WrapIf(err, "failed to determine current branch")
	}

	commitArgs := []string{"commit", "--amend"}
	if !edit && message == "" {
		commitArgs = append(commitArgs, "--no-edit")
	}
	if all {
		commitArgs = append(commitArgs, "--all")
	}
	if message != "" {
		commitArgs = append(commitArgs, "--message", message)
	}

	tx := db.WriteTx()
	defer tx.Abort()

	branch, _ := tx.Branch(currentBranch)
	if branch.PullRequest != nil && branch.PullRequest.State == githubv4.PullRequestStateMerged {
		fmt.Fprint(
			os.Stderr,
			colors.Failure("This branch has already been merged, amending is not allowed"),
			"\n",
		)
		return errors.New("this branch has already been merged, amending is not allowed")
	}

	// Handle "--all-changes"
	if commitFlags.AllChanges {
		_, err := repo.Run(ctx, &git.RunOpts{
			Args:      []string{"add", "--all"},
			ExitError: true,
		})
		if err != nil {
			fmt.Fprint(os.Stderr,
				"\n", colors.Failure("Failed to stage files: ", err.Error()), "\n",
			)
			return actions.ErrExitSilently{ExitCode: 1}
		}
	}

	if _, err := repo.Run(ctx, &git.RunOpts{
		Args:        commitArgs,
		ExitError:   true,
		Interactive: true,
	}); err != nil {
		return err
	}
	return nil
}

func branchAndCommit(
	ctx context.Context,
	branchName string,
	message string,
	all bool,
	allModified bool,
	parentBranchName string,
) (reterr error) {
	if branchName == "" {
		if message == "" {
			return errors.New(
				"Need a branch name (--branch-name <name>) or a commit message (-m <message>)",
			)
		}
		branchName = branchNameFromMessage(message)
		if branchName == "" {
			return errors.New("Cannot create a valid branch name from the message")
		}
	}

	repo, err := getRepo(ctx)
	if err != nil {
		return err
	}

	db, err := getDB(ctx, repo)
	if err != nil {
		return err
	}

	err = createBranch(ctx, repo, db, branchName, parentBranchName)
	if err != nil {
		return err
	}

	tx := db.WriteTx()
	var cu cleanup.Cleanup
	defer cu.Cleanup()
	cu.Add(func() {
		logrus.WithError(reterr).Debug("aborting db transaction")
		tx.Abort()
	})

	// For "--all" and "--all-modified",
	var addArgs []string
	if all {
		addArgs = append(addArgs, "--all")
	} else if allModified {
		// This is meant to mirror `git commit --all` which does not add
		// unstaged files to the index (which is different from `git add --all`).
		addArgs = append(addArgs, "--update")
	}
	if len(addArgs) > 0 {
		_, err := repo.Run(ctx, &git.RunOpts{
			Args:      append([]string{"add"}, addArgs...),
			ExitError: true,
		})
		if err != nil {
			fmt.Fprint(os.Stderr,
				"\n", colors.Failure("Failed to stage files: ", err.Error()), "\n",
			)
			return actions.ErrExitSilently{ExitCode: 1}
		}
	}

	commitArgs := []string{"commit"}
	if message != "" {
		commitArgs = append(commitArgs, "--message", message)
	}

	if _, err := repo.Run(ctx, &git.RunOpts{
		Args:        commitArgs,
		ExitError:   true,
		Interactive: true,
	}); err != nil {
		fmt.Fprint(os.Stderr,
			"\n", colors.Failure("Failed to create commit."), "\n",
		)
		return actions.ErrExitSilently{ExitCode: 1}
	}

	// Cancel the cleanup **after** the commit is successful (so that we
	// delete the created branch).
	cu.Cancel()
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func branchNameFromMessage(message string) string {
	message = strings.SplitN(strings.TrimSpace(message), "\n", 2)[0]
	name := branchNameReplacedPattern.ReplaceAllLiteralString(message, " ")
	name = strings.TrimSpace(name)
	name = multipleSpacePattern.ReplaceAllLiteralString(name, "-")
	if len(name) > branchNameLength {
		name = name[:branchNameLength]
	}
	name = strings.ToLower(name)
	return name
}

func init() {
	commitCmd.Flags().
		StringArrayVarP(&commitFlags.Message, "message", "m", nil, "the commit message")
	commitCmd.Flags().
		BoolVarP(&commitFlags.All, "all", "a", false, "automatically stage modified and deleted files (same as git commit --all)")
	commitCmd.Flags().
		BoolVarP(&commitFlags.AllChanges, "all-changes", "A", false, "automatically stage all files, including untracked files")
	commitCmd.Flags().
		BoolVar(&commitFlags.Amend, "amend", false, "amend the last commit")
	commitCmd.Flags().
		BoolVar(&commitFlags.Edit, "edit", false, "edit an amended commit's message")
	commitCmd.Flags().
		BoolVarP(&commitFlags.CreateBranch, "branch", "b", false,
			"create a new branch with automatically generated name and commit to it")
	commitCmd.Flags().
		StringVar(&commitFlags.BranchName, "branch-name", "", "create a new branch with the given name and commit to it")
	commitCmd.Flags().
		StringVar(&commitFlags.Parent, "parent", "", "the parent branch to base the new branch off of")

	_ = branchCmd.RegisterFlagCompletionFunc(
		"parent",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			branches, _ := allBranches(cmd.Context())
			return branches, cobra.ShellCompDirectiveNoFileComp
		},
	)

	commitCmd.MarkFlagsMutuallyExclusive("all", "all-changes")

	deprecatedAmendCmd := deprecateCommand(*commitAmendCmd, "av commit --amend", "amend")
	deprecatedAmendCmd.Hidden = true
	deprecatedCreateCmd := deprecateCommand(*commitCmd, "av commit", "create")
	deprecatedCreateCmd.Hidden = true
	deprecatedSplitCmd := deprecateCommand(*splitCommitCmd, "av split-commit", "split")
	deprecatedSplitCmd.Hidden = true

	deprecatedAmendCmd.Flags().
		StringVarP(&commitAmendFlags.Message, "message", "m", "", "the commit message")
	deprecatedAmendCmd.Flags().
		BoolVar(&commitAmendFlags.NoEdit, "no-edit", false, "amend a commit without changing its commit message")
	deprecatedAmendCmd.MarkFlagsMutuallyExclusive("message", "no-edit")
	deprecatedAmendCmd.Flags().
		BoolVarP(&commitAmendFlags.All, "all", "a", false, "automatically stage modified files (same as git commit --all)")

	commitCmd.AddCommand(
		deprecatedAmendCmd,
		deprecatedCreateCmd,
		deprecatedSplitCmd,
	)
}
