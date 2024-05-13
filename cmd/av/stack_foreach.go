package main

import (
	"fmt"
	"os"
	"os/exec"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/executils"
	"github.com/spf13/cobra"
)

var stackForEachFlags struct {
	previous   bool
	subsequent bool
}

var stackForEachCmd = &cobra.Command{
	Use:     "for-each [flags] -- <command> [args...] ggit ",
	Aliases: []string{"foreach", "fe"},
	Short:   "execute a command for each branch in the current stack",
	Long: `Execute a command for each branch in the current stack.

To prevent flags for the command to be executed from being parsed as flags for
this command, use the "--" separator (see examples below).

Output from the command will be printed to stdout/stderr as it is generated.

Examples:
  Print the current HEAD commit for each branch in the stack:
    $ av stack for-each -- git rev-parse HEAD

  Push every branch in the stack:
	$ av stack for-each -- git push --force
  Note that the "--" separator is required here to prevent "--force" from being
  interpreted as a flag for the "stack for-each" command.
`,
	Args: cobra.MinimumNArgs(1),
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
		currentBranch, err := repo.CurrentBranchName()
		if err != nil {
			return err
		}

		var branches []string
		if stackForEachFlags.previous {
			branches, err = meta.PreviousBranches(tx, currentBranch)
			if err != nil {
				return err
			}
			branches = append(branches, currentBranch)
		} else if stackForEachFlags.subsequent {
			branches = meta.SubsequentBranches(tx, currentBranch)
			branches = append([]string{currentBranch}, branches...)
		} else {
			branches, err = meta.StackBranches(tx, currentBranch)
			if err != nil {
				return err
			}
		}

		_, _ = fmt.Fprint(os.Stderr,
			"Executing command ", colors.CliCmd(executils.FormatCommandLine(args)),
			" for ", colors.UserInput(len(branches)), " branches:\n",
		)
		for _, branch := range branches {
			_, _ = fmt.Fprint(os.Stderr,
				"  - switching to branch ", colors.UserInput(branch), "\n",
			)
			if _, err := repo.CheckoutBranch(&git.CheckoutBranch{Name: branch}); err != nil {
				return errors.Wrapf(err, "failed to switch to branch %q", branch)
			}
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return errors.Wrapf(err, "failed to execute command for branch %q", branch)
			}
		}

		// Switch back to the original branch.
		// We only do this on success, because on failure, it's likely that the
		// user will want to be on the branch that had issues.
		if _, err := repo.CheckoutBranch(&git.CheckoutBranch{Name: currentBranch}); err != nil {
			return errors.Wrapf(err, "failed to switch back to branch %q", currentBranch)
		}
		return nil
	},
}

func init() {
	stackForEachCmd.Flags().BoolVar(
		&stackForEachFlags.previous, "previous", false,
		"apply the command only to the current branch and all previous branches in the stack",
	)
	stackForEachCmd.Flags().BoolVar(
		&stackForEachFlags.subsequent, "subsequent", false,
		"apply the command only to the current branch and all subsequent branches in the stack",
	)
	stackForEachCmd.MarkFlagsMutuallyExclusive("previous", "subsequent")
}
