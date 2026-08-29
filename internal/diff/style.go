package diff

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	FileHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00F5D4")).
			Background(lipgloss.Color("#1E293B")).
			Padding(0, 1)

	HunkHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00BBF9")).
			Italic(true)

	FocusedHunkHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FEE440")).
				Background(lipgloss.Color("#334155")).
				Padding(0, 1)

	AdditionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22C55E"))

	DeletionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))

	ContextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8"))

	GutterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748B"))

	BadgeStagedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#22C55E"))

	BadgeUnstagedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#EAB308"))
)

// RenderLine renders a single DiffLine with line numbers and syntax color.
func RenderLine(line DiffLine, width int) string {
	var gutter string
	if line.OldLine > 0 && line.NewLine > 0 {
		gutter = fmt.Sprintf("%4d %4d ", line.OldLine, line.NewLine)
	} else if line.OldLine > 0 {
		gutter = fmt.Sprintf("%4d      ", line.OldLine)
	} else if line.NewLine > 0 {
		gutter = fmt.Sprintf("     %4d ", line.NewLine)
	} else {
		gutter = "          "
	}
	styledGutter := GutterStyle.Render(gutter)

	var symbol string
	var contentStyle lipgloss.Style

	switch line.Type {
	case LineAddition:
		symbol = "+"
		contentStyle = AdditionStyle
	case LineDeletion:
		symbol = "-"
		contentStyle = DeletionStyle
	case LineContext:
		symbol = " "
		contentStyle = ContextStyle
	}

	content := line.Content
	maxContentWidth := width - 12
	if maxContentWidth > 10 && len(content) > maxContentWidth {
		content = content[:maxContentWidth]
	}

	styledSymbol := contentStyle.Render(symbol)
	styledContent := contentStyle.Render(content)

	return fmt.Sprintf("%s%s %s", styledGutter, styledSymbol, styledContent)
}

// RenderHunk renders a DiffHunk with header and lines.
func RenderHunk(hunk DiffHunk, width int, isFocused bool) string {
	var sb strings.Builder

	var headerStyled string
	statusBadge := ""
	if hunk.IsStaged {
		statusBadge = " " + BadgeStagedStyle.Render("[STAGED]")
	}

	if isFocused {
		headerStyled = FocusedHunkHeaderStyle.Render("▶ " + hunk.Header + statusBadge)
	} else {
		headerStyled = HunkHeaderStyle.Render("  " + hunk.Header + statusBadge)
	}
	sb.WriteString(headerStyled)
	sb.WriteString("\n")

	for _, line := range hunk.Lines {
		sb.WriteString(RenderLine(line, width))
		sb.WriteString("\n")
	}

	return sb.String()
}

// RenderDiffFile renders a DiffFile including header, binary indication, or all hunks.
func RenderDiffFile(file DiffFile, width int, focusedHunkIndex int) string {
	var sb strings.Builder

	path := file.NewPath
	if path == "" {
		path = file.OldPath
	}
	fileHeader := FileHeaderStyle.Render(fmt.Sprintf("FILE: %s", path))
	sb.WriteString(fileHeader)
	sb.WriteString("\n\n")

	if file.IsBinary {
		sb.WriteString(ContextStyle.Render("  Binary file differences cannot be displayed in text mode.\n\n"))
		return sb.String()
	}

	for i, hunk := range file.Hunks {
		isFocused := (i == focusedHunkIndex)
		sb.WriteString(RenderHunk(hunk, width, isFocused))
		sb.WriteString("\n")
	}

	return sb.String()
}
