package output

import (
	"fmt"
	"io"
	"strings"
)

func Rows(w io.Writer, headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	writeRow(w, widths, headers)
	for i, width := range widths {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprint(w, strings.Repeat("-", width))
	}
	fmt.Fprintln(w)
	for _, row := range rows {
		writeRow(w, widths, row)
	}
}

func writeRow(w io.Writer, widths []int, row []string) {
	for i, width := range widths {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		cell := ""
		if i < len(row) {
			cell = row[i]
		}
		fmt.Fprintf(w, "%-*s", width, cell)
	}
	fmt.Fprintln(w)
}
