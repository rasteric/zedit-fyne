package zedit

// CharPos represents a character position in the grid.
// If IsLineNumber is true, then the position is in the line
// number display, Line contains the line number, and Column is 0.
// Otherwise, Line and Column contain the line, column pair
// of the grid closest to the position.
type CharPos struct {
	Line         int
	Column       int
	IsLineNumber bool
}

// CmpPos lexicographically compares to char positions and
// returns -1 if a is before b, 0 if a and b are equal positions,
// and 1 if a is after b. This is used for interval operation.
func CmpPos(a, b CharPos) int {
	if a.Line < b.Line {
		return -1
	}
	if a.IsLineNumber {
		if a.Line == b.Line {
			return 0
		}
		return 1
	}
	if a.Line > b.Line {
		return 1
	}
	if a.Column < b.Column {
		return -1
	}
	if a.Column == b.Column {
		return 0
	}
	return 1
}

func MaxPos(a, b CharPos) CharPos {
	n := CmpPos(a, b)
	if n == -1 {
		return b
	}
	return a
}

func MinPos(a, b CharPos) CharPos {
	n := CmpPos(a, b)
	if n <= 0 {
		return a
	}
	return b
}

type CharInterval struct {
	Start CharPos
	End   CharPos
}

// OutsideOf returns true if c1 is outside of c2.
func (c1 CharInterval) OutsideOf(c2 CharInterval) bool {
	return CmpPos(c1.End, c2.Start) == -1 || CmpPos(c1.Start, c2.End) == 1
}

// Contains returns true if the char interval contains the given position, false otherwise.
func (c CharInterval) Contains(pos CharPos) bool {
	return CmpPos(pos, c.Start) >= 0 && CmpPos(pos, c.End) <= 0
}

// MaybeSwap compares the start and the end, and if the end is before
// the start returns the interval where the end is the start and the start is the end.
// The function returns the unchanged interval otherwise.
func (c CharInterval) MaybeSwap() CharInterval {
	if CmpPos(c.Start, c.End) > 0 {
		return CharInterval{Start: c.End, End: c.Start}
	}
	return c
}

// Lines returns the number of lines, including incomplete lines of the interval.
func (c CharInterval) Lines() int {
	return c.End.Line - c.Start.Line + 1
}