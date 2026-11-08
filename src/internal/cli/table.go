package cli

import (
	"fmt"
	"unicode/utf8"
)

type tableColumn struct {
	Header string
	Width  int
}

func (a *App) printTable(columns []tableColumn, rows [][]string, sep string) {
	if len(columns) == 0 {
		return
	}
	sep = normalizeSeparator(sep)
	widths := computeColumnWidths(columns, rows)
	formats := buildFormatStrings(columns, widths)

	a.writeTableRow(formats, sep, extractHeaders(columns))
	for _, row := range rows {
		a.writeTableRow(formats, sep, row)
	}
}

func normalizeSeparator(sep string) string {
	if sep == "" {
		return "\t"
	}
	return sep
}

func computeColumnWidths(columns []tableColumn, rows [][]string) []int {
	widths := make([]int, len(columns))
	for i, col := range columns {
		if col.Width <= 0 {
			continue
		}
		widths[i] = utf8.RuneCountInString(col.Header)
		for _, row := range rows {
			if i >= len(row) {
				continue
			}
			if length := utf8.RuneCountInString(row[i]); length > widths[i] {
				widths[i] = length
			}
		}
	}
	return widths
}

func buildFormatStrings(columns []tableColumn, widths []int) []string {
	formats := make([]string, len(columns))
	for i, col := range columns {
		if col.Width > 0 {
			width := widths[i]
			if width == 0 {
				width = utf8.RuneCountInString(col.Header)
			}
			formats[i] = fmt.Sprintf("%%-%ds", width)
			continue
		}
		formats[i] = "%s"
	}
	return formats
}

func extractHeaders(columns []tableColumn) []string {
	headers := make([]string, len(columns))
	for i, col := range columns {
		headers[i] = col.Header
	}
	return headers
}

func (a *App) writeTableRow(formats []string, sep string, values []string) {
	for i, format := range formats {
		if i > 0 {
			fmt.Fprint(a.stdout, sep)
		}
		value := ""
		if i < len(values) {
			value = values[i]
		}
		fmt.Fprintf(a.stdout, format, value)
	}
	fmt.Fprint(a.stdout, "\n")
}
