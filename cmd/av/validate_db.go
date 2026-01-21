package main

import (
	"context"
	"fmt"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

type diagnosticSeverity string

const (
	diagnosticError   diagnosticSeverity = "error"
	diagnosticWarning diagnosticSeverity = "warning"
)

type diagnosticIssue struct {
	severity diagnosticSeverity
	branch   string
	message  string
}

var validateDBCmd = &cobra.Command{
	Use:   "validate-db",
	Short: "Validate av metadata",
	Long: `Validate av metadata for common consistency issues, including cyclical
branch dependencies and missing parents.`,
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

		issues, err := validateDB(ctx, repo, db.ReadTx())
		if err != nil {
			return err
		}

		fmt.Print(renderValidation(issues))
		return nil
	},
}

func validateDB(ctx context.Context, repo *git.Repo, tx meta.ReadTx) ([]diagnosticIssue, error) {
	branches := tx.AllBranches()
	issues := make([]diagnosticIssue, 0)

	for branchName, branch := range branches {
		if branchName == "" {
			continue
		}

		exists, err := repo.DoesBranchExist(ctx, branchName)
		if err != nil {
			return nil, err
		}
		if !exists {
			issues = append(issues, diagnosticIssue{
				severity: diagnosticError,
				branch:   branchName,
				message:  "branch is missing from the Git repository",
			})
			continue
		}

		if branch.Parent.Name == "" && !branch.Parent.Trunk {
			issues = append(issues, diagnosticIssue{
				severity: diagnosticError,
				branch:   branchName,
				message:  "parent is empty but not marked as trunk",
			})
		}

		if branch.Parent.Name == branchName {
			issues = append(issues, diagnosticIssue{
				severity: diagnosticError,
				branch:   branchName,
				message:  "parent points to itself",
			})
		} else if err := meta.ValidateNoCycle(tx, branchName, branch.Parent); err != nil {
			issues = append(issues, diagnosticIssue{
				severity: diagnosticError,
				branch:   branchName,
				message:  err.Error(),
			})
		}
	}

	return issues, nil
}

func renderValidation(issues []diagnosticIssue) string {
	if len(issues) == 0 {
		return lipgloss.NewStyle().MarginTop(1).MarginBottom(1).MarginLeft(2).Render(
			colors.SuccessStyle.Render("✓ No av metadata issues found"),
		) + "\n"
	}

	var errors, warnings []diagnosticIssue
	for _, issue := range issues {
		switch issue.severity {
		case diagnosticError:
			errors = append(errors, issue)
		case diagnosticWarning:
			warnings = append(warnings, issue)
		}
	}

	var ss []string
	if len(errors) > 0 {
		ss = append(ss, colors.FailureStyle.Render("✗ Issues found in av metadata"))
	} else {
		ss = append(ss, colors.Warning("! Warnings found in av metadata"))
	}

	if len(errors) > 0 {
		ss = append(ss, "")
		ss = append(ss, "  Errors:")
		for _, issue := range errors {
			ss = append(ss, fmt.Sprintf("  * %s: %s", issue.branch, issue.message))
		}
	}

	if len(warnings) > 0 {
		ss = append(ss, "")
		ss = append(ss, "  Warnings:")
		for _, issue := range warnings {
			ss = append(ss, fmt.Sprintf("  * %s: %s", issue.branch, issue.message))
		}
	}

	return lipgloss.NewStyle().MarginTop(1).MarginBottom(1).MarginLeft(2).Render(
		lipgloss.JoinVertical(0, ss...),
	) + "\n"
}
