package stackutils

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func RenderTree(node *StackTreeNode, branchDataFn func(branchName string, isTrunk bool) string) string {
	return strings.TrimSuffix(renderTreeInternal(0, node, true, branchDataFn), "\n")
}

func renderTreeInternal(columns int, node *StackTreeNode, isTrunk bool, branchDataFn func(branchName string, isTrunk bool) string) string {
	sb := strings.Builder{}
	for i, child := range node.Children {
		sb.WriteString(renderTreeInternal(columns+i, child, false, branchDataFn))
	}
	if len(node.Children) > 1 {
		sb.WriteString(" ")
		sb.WriteString(strings.Repeat(" │", columns))
		sb.WriteString(" ├")
		sb.WriteString(strings.Repeat("─┴", len(node.Children)-2))
		sb.WriteString("─┘")
		sb.WriteString("\n")
	} else if len(node.Children) == 1 {
		sb.WriteString(" ")
		sb.WriteString(strings.Repeat(" │", columns+1))
		sb.WriteString("\n")
	} else if columns > 0 {
		sb.WriteString(" ")
		sb.WriteString(strings.Repeat(" │", columns))
		sb.WriteString("\n")
	}

	firstLine := " " + strings.Repeat(" │", columns) + " * "
	contLine := " " + strings.Repeat(" │", columns+1) + " "

	branchData := strings.TrimSuffix(branchDataFn(node.Branch.BranchName, isTrunk), "\n")
	height := lipgloss.Height(branchData)
	var lhs string
	if height == 0 {
		lhs = firstLine
	} else {
		lhs = firstLine
		for i := 0; i < height-1; i++ {
			lhs += "\n" + contLine
		}
	}
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, lhs, branchData))
	sb.WriteString("\n")
	return sb.String()
}
