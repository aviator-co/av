package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/cleanup"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// See https://git-scm.com/docs/git-cherry-pick#_options
// for more details on these options for cherry-pick
var cherryPickFlags struct {
	Edit                 bool
	X                    bool
	R                    bool
	CleanupMode          string
	SignOff              bool
	MainlineParent       string
	FastForward          bool
	GPGSign              string
	NoGPGSign            bool
	AllowEmpty           bool
	AllowEmptyMessage    bool
	Empty                string // only valid values are [ "drop" | "keep" | "stop" ]
	Strategy             string
	KeepRedundantCommits bool
	StrategyOption       string
	RerereAutoUpdate     bool
	NoRerereAutoUpdate   bool

	// Sequencer SubCommands
	Continue bool
	Abort    bool
	Quit     bool
	Skip     bool
}

// TODO: check if required
type cherryPickError struct {
	message string
}

func (err *cherryPickError) Error() string {
	return fmt.Sprintf("ERROR: [cherry-pick] - %s", err.message)
}

var cherryPickCmd = &cobra.Command{
	Use:   "cherry-pick [flags] <commits...>",
	Short: "Apply the changes introduced by some existing commits to the stack",
	Long: strings.TrimSpace(`
		Applies changes from existing commits to the current branch.

		Please note that this command will not be making updates to remote.

		Use cherry-pick to selectively apply individual commits from other branches (or points in history) to your current branch. This is useful for backporting fixes or applying specific features.

		If conflicts occur, resolve them, stage the changes, and use 'av cherry-pick --continue'. 
		Use '--skip' to skip the commit, '--quit' to abort and leave conflicts, or '--abort' to restore the original state.

		Examples:
		av cherry-pick <commit-hash>
		av cherry-pick A..B
	`),
	Args: cobra.RangeArgs(0, 100), // TODO: check whether to support 1 commit or multiple commits
	RunE: func(cmd *cobra.Command, args []string) (reterr error) {
		isContinueSet, _ := cmd.Flags().GetBool("continue")
		isSkipSet, _ := cmd.Flags().GetBool("skip")
		isAbortSet, _ := cmd.Flags().GetBool("abort")
		isQuitSet, _ := cmd.Flags().GetBool("quit")

		if len(args) == 0 && !isContinueSet && !isAbortSet && !isSkipSet && !isQuitSet {
			// The only time we don't want to suppress the usage message is when
			// a user runs `av branch` with no arguments.
			return cmd.Usage()
		}

		// Add warnings for all flags that don't yet have support
		cmd.Flags().Visit(func(f *pflag.Flag) {
			fmt.Fprint(os.Stdout,
				"\n", colors.Warning(fmt.Sprintf("currently: there's no support for %s flag. coming soon.", f.Name)), "\n",
			)
		})

		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}

		return runCherryPick(repo, db, args)
	},
}

func init() {
	cherryPickCmd.Flags().
		BoolVarP(&cherryPickFlags.Edit, "edit", "e", false, "to edit the commit message prior to commit")
	cherryPickCmd.Flags().
		StringVar(&cherryPickFlags.CleanupMode, "cleanup", "", "determines how the commit message will be cleaned up before being passed on to the commit machinery")
	cherryPickCmd.Flags().
		BoolVar(&cherryPickFlags.X, "x", false, `append a line that says "(cherry picked from commit ...)" to the original commit message`)
	cherryPickCmd.Flags().
		BoolVar(&cherryPickFlags.R, "r", false, "a no-op, see https://git-scm.com/docs/git-cherry-pick#Documentation/git-cherry-pick.txt--r")
	cherryPickCmd.Flags().
		BoolVarP(&cherryPickFlags.SignOff, "signoff", "s", false, `add a "Signed-off-by" trailer at the end of the commit message`)
	cherryPickCmd.Flags().
		BoolVar(&cherryPickFlags.FastForward, "ff", false, "If the current HEAD is the same as the parent of the cherry-pick'ed commit, then a fast forward to this commit will be performed")
	cherryPickCmd.Flags().
		StringVarP(&cherryPickFlags.MainlineParent, "mainline", "m", "", "option specifies the parent number (starting from 1) of the mainline and allows cherry-pick to replay the change")
	cherryPickCmd.Flags().
		StringVar(&cherryPickFlags.GPGSign, "gpg-sign", "", "to GPG sign commits")
	cherryPickCmd.Flags().
		BoolVar(&cherryPickFlags.NoGPGSign, "no-gpg-sign", false, "to countermand gpg-sign")
	cherryPickCmd.Flags().
		BoolVar(&cherryPickFlags.AllowEmpty, "allow-empty", false, "option overrides default behavior by allowing empty commits to be preserved automatically in a cherry-pick")
	cherryPickCmd.Flags().
		BoolVar(&cherryPickFlags.AllowEmptyMessage, "allow-empty-message", false, "by default, cherry-picking a commit with an empty message will fail. This option overrides that behavior")
	cherryPickCmd.Flags().
		StringVar(&cherryPickFlags.Empty, "empty", "", "handles commits being cherry-picked that are redundant with changes already in the current history. for info see https://git-scm.com/docs/git-cherry-pick#Documentation/git-cherry-pick.txt---emptydropkeepstop")
	cherryPickCmd.Flags().
		BoolVar(&cherryPickFlags.KeepRedundantCommits, "keep-redundant-commits", false, "deprecated synonym for --empty=keep")
	cherryPickCmd.Flags().
		StringVar(&cherryPickFlags.Strategy, "strategy", "", "uses a given merge strategy. see more: https://git-scm.com/docs/git-merge#Documentation/git-merge.txt---strategyltstrategygt")
	cherryPickCmd.Flags().
		StringVar(&cherryPickFlags.StrategyOption, "strategy-option", "", "pass the merge strategy-specific option through to the merge strategy. see more: https://git-scm.com/docs/git-merge#Documentation/git-merge.txt---strategy-optionltoptiongt")
	cherryPickCmd.Flags().
		BoolVar(&cherryPickFlags.RerereAutoUpdate, "rerere-autoupdate", false, "reuses a recorded resolution on the current conflict to update the files in the working tree, allow it to also update the index with the result of resolution")
	cherryPickCmd.Flags().
		BoolVar(&cherryPickFlags.NoRerereAutoUpdate, "no-rerere-autoupdate", false, `a good way to double-check what "rerere" did and catch potential mismerges, before committing the result`)

	// sequencer subcommands (as flags)
	cherryPickCmd.Flags().
		BoolVar(&cherryPickFlags.Continue, "continue", false, "continues performing the operation in progress. Can also be used to continue after resolving conflicts in a failed cherry-pick or revert")
	cherryPickCmd.Flags().
		BoolVar(&cherryPickFlags.Abort, "abort", false, "cancel the operation and return to the pre-sequence state")
	cherryPickCmd.Flags().
		BoolVar(&cherryPickFlags.Skip, "skip", false, "skip the current commit and continue with the rest of the sequence")
	cherryPickCmd.Flags().
		BoolVar(&cherryPickFlags.Quit, "quit", false, "forget about the current operation in progress, can also be used to clear the sequencer state")
}

func runCherryPick(
	repo *git.Repo,
	db meta.DB,
	args []string,
) (reterr error) {
	tx := db.WriteTx()
	cu := cleanup.New(func() {
		logrus.WithError(reterr).Debug("aborting db transaction")
		tx.Abort()
	})
	defer cu.Cleanup()

	// TODO: add support for multiple IDs
	if len(args) > 1 {
		fmt.Fprint(os.Stdout,
			"\n", colors.Warning("currently: only 1 commit can be cherry-picked at a time"), "\n",
		)
	}

	commitId := args[0]

	gitOutput, reterr := repo.Run(&git.RunOpts{ // TODO: git output needs to be checked here
		Args:      append([]string{"cherry-pick"}, commitId),
		ExitError: true,
	})

	if reterr != nil {
		return &cherryPickError{
			message: fmt.Sprintf("ERROR: [cherry-pick] - %v", reterr),
		}
	}

	if gitOutput.ExitCode == 0 {
		for _, line := range gitOutput.Lines() {
			fmt.Println(line)
		}
		fmt.Fprint(os.Stdout,
			"\n", colors.Success(
				fmt.Sprintf("cherry-picked commit: [ %s ] to current branch", commitId),
			), "\n",
		)
	}

	cu.Cancel()
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
