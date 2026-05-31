package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// Render returns an aligned, borderless table: a bold header row over
// space-padded columns. Borderless keeps the output copy- and grep-friendly and
// close to the existing tabwriter layout. headers and each row must share the
// same column count; cell content is taken verbatim (callers pre-format any
// glyphs or colors).
func Render(headers []string, rows [][]string) string {
	t := table.New().
		Border(lipgloss.Border{}).
		BorderTop(false).BorderBottom(false).
		BorderLeft(false).BorderRight(false).
		BorderColumn(false).BorderRow(false).BorderHeader(false).
		Headers(headers...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return Header.PaddingRight(2)
			}
			return lipgloss.NewStyle().PaddingRight(2)
		})
	for _, r := range rows {
		t.Row(r...)
	}
	return t.String()
}
