package widgets

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// TreeRowSpec describes one row in a tree view: label, indentation
// depth, expandable/expanded state, selection state, and the
// horizontal scroll offset (number of runes shifted off the left
// edge — supplied by the owning view's Scrollable state).
type TreeRowSpec struct {
	Label      string
	Depth      int
	Expandable bool
	Expanded   bool
	Selected   bool
	ScrollX    int
}

// DrawTreeRow renders one row at (x, y) clipped to width, shifted
// left by spec.ScrollX runes.
//
// Layout (logical row coordinates, before scroll):
//
//	[ptr][indent…][marker][label]
//	 ptr     "▸" when Selected, else " "
//	 indent  2 spaces per depth level
//	 marker  "▼ " expanded, "▶ " collapsed-expandable, "  " leaf
func DrawTreeRow(screen tcell.Screen, x, y, width int, spec TreeRowSpec, normal, highlight tcell.Style) {
	if width <= 0 {
		return
	}
	style := normal
	if spec.Selected {
		style = highlight
	}
	fillSpan(screen, x, y, width, style)

	ptr := " "
	if spec.Selected {
		ptr = "▸"
	}
	marker := "  "
	if spec.Expandable {
		if spec.Expanded {
			marker = "▼ "
		} else {
			marker = "▶ "
		}
	}

	row := []rune(ptr)
	row = append(row, []rune(strings.Repeat(" ", spec.Depth*2))...)
	row = append(row, []rune(marker)...)
	row = append(row, []rune(spec.Label)...)

	off := spec.ScrollX
	if off < 0 {
		off = 0
	}
	if off >= len(row) {
		return
	}
	row = row[off:]
	for i, r := range row {
		if i >= width {
			break
		}
		screen.SetContent(x+i, y, r, nil, style)
	}
}
