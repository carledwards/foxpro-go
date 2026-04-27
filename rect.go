package foxpro

// Rect is a window bounding box in screen cells. X,Y is top-left.
type Rect struct {
	X, Y, W, H int
}

// Contains reports whether the cell at (cx,cy) lies inside r.
func (r Rect) Contains(cx, cy int) bool {
	return cx >= r.X && cx < r.X+r.W && cy >= r.Y && cy < r.Y+r.H
}

// Inner returns the content area, excluding the 1-cell border on every side.
func (r Rect) Inner() Rect {
	return Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}
}
