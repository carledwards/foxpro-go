package foxpro

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"

	"github.com/carledwards/foxpro-go/widgets"
)

// TreeNode is one entry in a tree. Children may be supplied directly
// or fetched lazily via Loader (called once on first expansion).
// Payload is whatever app-specific value the caller wants to recover
// later (the selected resource, an ID, a callback, etc.).
type TreeNode struct {
	Label    string
	Payload  interface{}
	Children []*TreeNode
	Expanded bool
	Loader   func(*TreeNode) []*TreeNode

	loaded bool
}

// HasChildren reports whether the node can be expanded.
func (n *TreeNode) HasChildren() bool {
	return len(n.Children) > 0 || (n.Loader != nil && !n.loaded)
}

// TreeView is a ContentProvider that renders a tree, handles arrow-key
// navigation, expand/collapse, and selection. Implements Scrollable,
// MouseHandler, and StatusHinter — both vertical and horizontal scroll.
type TreeView struct {
	Root       *TreeNode
	HideRoot   bool             // when true, the root row isn't drawn
	OnSelect   func(*TreeNode)  // called whenever the selection moves
	OnActivate func(*TreeNode)  // called on Enter (after toggle)

	selected     int
	scrollX      int
	scrollY      int
	lastVisibleW int
	lastVisibleH int
	contentW     int // widest visible row, recomputed on rebuild

	// flat / flatDepth are recomputed each rebuild; index = visible row.
	flat      []*TreeNode
	flatDepth []int
}

// NewTreeView returns a TreeView seeded with root.
func NewTreeView(root *TreeNode) *TreeView {
	v := &TreeView{Root: root}
	v.rebuild()
	return v
}

// SetRoot swaps the root and rebuilds the visible flat list. Selection
// is reset to the top. Use ReplaceRoot when the new tree comes from
// refreshed data and you want user-driven state (expansion, selection)
// to survive.
func (v *TreeView) SetRoot(root *TreeNode) {
	v.Root = root
	v.selected = 0
	v.rebuild()
}

// Rebuild recomputes the visible flat row list from the current
// TreeNode graph. Call this after mutating exported fields (HideRoot,
// for instance) or after editing TreeNode children directly without
// going through SetRoot / ReplaceRoot.
func (v *TreeView) Rebuild() { v.rebuild() }

// ReplaceRoot is like SetRoot but preserves the user's expansion
// state and selection across the swap by matching nodes via their
// label-path. Use it when you rebuild a tree from refreshed data so
// the UI doesn't snap back to a fully-collapsed top-row state.
func (v *TreeView) ReplaceRoot(root *TreeNode) {
	if v.Root != nil {
		MergeTreeState(v.Root, root)
	}
	selPath := v.SelectedPath()
	v.Root = root
	v.selected = 0
	v.rebuild()
	if len(selPath) > 0 {
		v.SelectByPath(selPath)
	}
}

// SelectedPath returns the chain of labels from the root down to the
// currently selected node, or nil if nothing is selected. The path
// always starts with the root's label, even when HideRoot is set, so
// it round-trips cleanly through SelectByPath.
func (v *TreeView) SelectedPath() []string {
	if v.selected < 0 || v.selected >= len(v.flat) {
		return nil
	}
	targetDepth := v.flatDepth[v.selected]
	rev := []string{v.flat[v.selected].Label}
	currentDepth := targetDepth - 1
	for i := v.selected - 1; i >= 0 && currentDepth >= 0; i-- {
		if v.flatDepth[i] == currentDepth {
			rev = append(rev, v.flat[i].Label)
			currentDepth--
		}
	}
	if v.HideRoot && v.Root != nil {
		rev = append(rev, v.Root.Label)
	}
	out := make([]string, len(rev))
	for i, s := range rev {
		out[len(rev)-1-i] = s
	}
	return out
}

// SelectByPath selects the first node whose label-path from the root
// matches segments. The first segment must equal the root's label.
// Returns true if the target node was found and is currently visible
// in the flat list (i.e. its ancestors are expanded). Caller is
// responsible for expanding ancestors if needed.
func (v *TreeView) SelectByPath(segments []string) bool {
	if len(segments) == 0 || v.Root == nil || v.Root.Label != segments[0] {
		return false
	}
	cur := v.Root
	for i := 1; i < len(segments); i++ {
		var found *TreeNode
		for _, c := range cur.Children {
			if c.Label == segments[i] {
				found = c
				break
			}
		}
		if found == nil {
			return false
		}
		cur = found
	}
	v.rebuild()
	for i, n := range v.flat {
		if n == cur {
			v.selected = i
			v.ensureVisible()
			v.fireSelect()
			return true
		}
	}
	return false
}

// MergeTreeState copies the Expanded flag from oldRoot onto newRoot
// for nodes whose label-path matches. Lazy-loaded children that
// hadn't been expanded yet are left untouched on the new tree.
func MergeTreeState(oldRoot, newRoot *TreeNode) {
	if oldRoot == nil || newRoot == nil || oldRoot.Label != newRoot.Label {
		return
	}
	newRoot.Expanded = oldRoot.Expanded
	if len(oldRoot.Children) == 0 || len(newRoot.Children) == 0 {
		return
	}
	oldByLabel := make(map[string]*TreeNode, len(oldRoot.Children))
	for _, c := range oldRoot.Children {
		oldByLabel[c.Label] = c
	}
	for _, c := range newRoot.Children {
		if oc, ok := oldByLabel[c.Label]; ok {
			MergeTreeState(oc, c)
		}
	}
}

// Selected returns the highlighted node (or nil).
func (v *TreeView) Selected() *TreeNode {
	if v.selected < 0 || v.selected >= len(v.flat) {
		return nil
	}
	return v.flat[v.selected]
}

func (v *TreeView) rebuild() {
	v.flat = v.flat[:0]
	v.flatDepth = v.flatDepth[:0]
	var walk func(n *TreeNode, depth int)
	walk = func(n *TreeNode, depth int) {
		if !(v.HideRoot && depth == 0) {
			v.flat = append(v.flat, n)
			v.flatDepth = append(v.flatDepth, depth)
		}
		if n.Expanded {
			if !n.loaded && n.Loader != nil {
				n.Children = n.Loader(n)
				n.loaded = true
			}
			for _, c := range n.Children {
				walk(c, depth+1)
			}
		}
	}
	if v.Root != nil {
		walk(v.Root, 0)
	}
	if v.selected >= len(v.flat) {
		v.selected = len(v.flat) - 1
	}
	if v.selected < 0 {
		v.selected = 0
	}
	// Recompute the widest row (1 ptr + depth*2 indent + 2 marker + label).
	v.contentW = 0
	for i, n := range v.flat {
		w := 3 + v.renderDepth(i)*2 + utf8.RuneCountInString(n.Label)
		if w > v.contentW {
			v.contentW = w
		}
	}
}

// renderDepth returns the visual indent depth for the row at flat[i].
// HideRoot pulls everything up by one level.
func (v *TreeView) renderDepth(i int) int {
	d := v.flatDepth[i]
	if v.HideRoot {
		d--
	}
	if d < 0 {
		d = 0
	}
	return d
}

func (v *TreeView) Draw(screen tcell.Screen, inner Rect, theme Theme, focused bool) {
	v.lastVisibleW = inner.W
	v.lastVisibleH = inner.H
	v.clampScroll()
	for row := 0; row < inner.H && v.scrollY+row < len(v.flat); row++ {
		i := v.scrollY + row
		node := v.flat[i]
		spec := widgets.TreeRowSpec{
			Label:      node.Label,
			Depth:      v.renderDepth(i),
			Expandable: node.HasChildren(),
			Expanded:   node.Expanded,
			Selected:   focused && i == v.selected,
			ScrollX:    v.scrollX,
		}
		widgets.DrawTreeRow(screen, inner.X, inner.Y+row, inner.W, spec, theme.WindowBG, theme.Focus)
	}
}

func (v *TreeView) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp:
		if v.selected > 0 {
			v.selected--
			v.ensureVisible()
			v.fireSelect()
		}
		return true
	case tcell.KeyDown:
		if v.selected < len(v.flat)-1 {
			v.selected++
			v.ensureVisible()
			v.fireSelect()
		}
		return true
	case tcell.KeyPgUp:
		step := v.lastVisibleH - 1
		if step < 1 {
			step = 1
		}
		v.selected -= step
		if v.selected < 0 {
			v.selected = 0
		}
		v.ensureVisible()
		v.fireSelect()
		return true
	case tcell.KeyPgDn:
		step := v.lastVisibleH - 1
		if step < 1 {
			step = 1
		}
		v.selected += step
		if v.selected > len(v.flat)-1 {
			v.selected = len(v.flat) - 1
		}
		v.ensureVisible()
		v.fireSelect()
		return true
	case tcell.KeyHome:
		v.selected = 0
		v.ensureVisible()
		v.fireSelect()
		return true
	case tcell.KeyEnd:
		v.selected = len(v.flat) - 1
		v.ensureVisible()
		v.fireSelect()
		return true
	case tcell.KeyLeft:
		// Shift+Left scrolls the tree horizontally; plain Left
		// collapses the current node.
		if ev.Modifiers()&tcell.ModShift != 0 {
			v.scrollX -= 4
			v.clampScroll()
		} else if node := v.Selected(); node != nil && node.Expanded {
			node.Expanded = false
			v.rebuild()
			v.ensureVisible()
		}
		return true
	case tcell.KeyRight:
		if ev.Modifiers()&tcell.ModShift != 0 {
			v.scrollX += 4
			v.clampScroll()
		} else if node := v.Selected(); node != nil && node.HasChildren() && !node.Expanded {
			node.Expanded = true
			v.rebuild()
			v.ensureVisible()
		}
		return true
	case tcell.KeyRune:
		// `<` and `>` literal keys also pan horizontally — handy on
		// terminals that don't deliver Shift+arrow at all.
		switch ev.Rune() {
		case '<', ',':
			v.scrollX -= 4
			v.clampScroll()
			return true
		case '>', '.':
			v.scrollX += 4
			v.clampScroll()
			return true
		}
	case tcell.KeyEnter:
		node := v.Selected()
		if node == nil {
			return true
		}
		if node.HasChildren() {
			node.Expanded = !node.Expanded
			v.rebuild()
			v.ensureVisible()
		}
		if v.OnActivate != nil {
			v.OnActivate(node)
		}
		return true
	}
	return false
}

// ensureVisible nudges scrollY just enough to keep `selected` inside
// the visible viewport. No-op when lastVisibleH is 0 (before the
// first Draw); the next Draw will clamp anyway.
func (v *TreeView) ensureVisible() {
	if v.lastVisibleH <= 0 {
		return
	}
	if v.selected < v.scrollY {
		v.scrollY = v.selected
	} else if v.selected >= v.scrollY+v.lastVisibleH {
		v.scrollY = v.selected - v.lastVisibleH + 1
	}
	v.clampScroll()
}

// HandleMouse: click on a row selects it; click on the expand marker
// (the [▼/▶] glyph column) toggles expansion.
func (v *TreeView) HandleMouse(ev *tcell.EventMouse, inner Rect) bool {
	mx, my := ev.Position()
	if !inner.Contains(mx, my) {
		return false
	}
	row := my - inner.Y
	i := v.scrollY + row
	if i < 0 || i >= len(v.flat) {
		return false
	}
	v.selected = i
	v.ensureVisible()
	v.fireSelect()
	node := v.flat[i]
	markerX := inner.X + 1 + v.renderDepth(i)*2 - v.scrollX
	if node.HasChildren() && (mx == markerX || mx == markerX+1) {
		node.Expanded = !node.Expanded
		v.rebuild()
	}
	return true
}

// Scrollable.
func (v *TreeView) ContentSize() (int, int) { return v.contentW, len(v.flat) }

func (v *TreeView) ScrollOffset() (int, int) {
	v.clampScroll()
	return v.scrollX, v.scrollY
}

func (v *TreeView) SetScrollOffset(x, y int) {
	v.scrollX = x
	v.scrollY = y
	v.clampScroll()
}

func (v *TreeView) clampScroll() {
	maxTop := len(v.flat) - v.lastVisibleH
	if maxTop < 0 {
		maxTop = 0
	}
	if v.scrollY > maxTop {
		v.scrollY = maxTop
	}
	if v.scrollY < 0 {
		v.scrollY = 0
	}
	maxLeft := v.contentW - v.lastVisibleW
	if maxLeft < 0 {
		maxLeft = 0
	}
	if v.scrollX > maxLeft {
		v.scrollX = maxLeft
	}
	if v.scrollX < 0 {
		v.scrollX = 0
	}
}

// StatusHint.
func (v *TreeView) StatusHint() string {
	return "↑↓: move  ←→: collapse/expand  Enter: open "
}

func (v *TreeView) fireSelect() {
	if v.OnSelect != nil {
		v.OnSelect(v.Selected())
	}
}
