package office

import (
	"strings"
)

// MaxCells is the cells of a table a read shows: 10,000, the owner's number
// for a sheet.
const MaxCells = 10000

// Table writes rows as a Markdown table whose first row is the header. Rows
// are padded to the widest; a cell's pipes are escaped and its line breaks
// become spaces, so a row is one line. No rows is no table.
func Table(rows [][]string) string {
	width := 0
	for _, r := range rows {
		width = max(width, len(r))
	}
	if len(rows) == 0 || width == 0 {
		return ""
	}
	var b strings.Builder
	for i, r := range rows {
		b.WriteString("|")
		for c := 0; c < width; c++ {
			cell := ""
			if c < len(r) {
				cell = tableCell(r[c])
			}
			b.WriteString(" " + cell + " |")
		}
		b.WriteString("\n")
		if i == 0 {
			b.WriteString("|" + strings.Repeat(" --- |", width) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func tableCell(s string) string {
	s = strings.Join(strings.Fields(strings.NewReplacer("\r", " ", "\n", " ", "\t", " ", " ", " ", " ", " ").Replace(s)), " ")
	return strings.ReplaceAll(s, "|", `\|`)
}
