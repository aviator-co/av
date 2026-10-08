package stackutils

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func RenderTree(
	node *StackTreeNode,
	branchDataFn func(branchName string, isTrunk bool) string,
) string {
	return strings.TrimSuffix(renderTreeInternal(0, node, true, branchDataFn, 0, nil), "\n")
}

type LineRange struct {
	Start, End int
}

func RenderTreeWithLineRanges(
	node *StackTreeNode,
	branchDataFn func(branchName string, isTrunk bool) string,
) (string, map[string]LineRange) {
	ranges := map[string]LineRange{}
	out := strings.TrimSuffix(renderTreeInternal(0, node, true, branchDataFn, 0, ranges), "\n")
	return out, ranges
}

func renderTreeInternal(
	columns int,
	node *StackTreeNode,
	isTrunk bool,
	branchDataFn func(branchName string, isTrunk bool) string,
	startLine int,
	ranges map[string]LineRange,
) string {
	sb := strings.Builder{}
	for i, child := range node.Children {
		sb.WriteString(renderTreeInternal(
			columns+i,
			child,
			false,
			branchDataFn,
			startLine+strings.Count(sb.String(), "\n"),
			ranges,
		))
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
		for range height - 1 {
			lhs += "\n" + contLine
		}
	}
	if ranges != nil {
		start := startLine + strings.Count(sb.String(), "\n")
		ranges[node.Branch.BranchName] = LineRange{Start: start, End: start + max(height, 1)}
	}
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, lhs, branchData))
	sb.WriteString("\n")
	return sb.String()
}
