package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade the av CLI to the latest version",
	Long: `Upgrade the av CLI to the latest version.

This command checks for the latest release and updates the CLI accordingly.
If the CLI was installed via a package manager (e.g., Homebrew, AUR), it will
suggest using the package manager to perform the upgrade.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := actions.UpgradeCLI(runtime.GOOS, runtime.GOARCH); err != nil {
			fmt.Fprintln(os.Stderr, colors.Failure("Failed to upgrade av CLI:", err))
			return actions.ErrExitSilently{ExitCode: 1}
		}
		fmt.Fprintln(os.Stdout, colors.Success("Successfully upgraded av CLI to the latest version."))
		return nil
	},
}
