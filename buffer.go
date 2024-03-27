package zedit

type Buffer interface {
	Line(n int) []rune
	SetLine(n int, line []rune)
	AppendLine(line []rune)
	Len() int
	LineLen(n int) int
	Rune(line, column int) rune
}

type MemBuffer struct {
	rows [][]rune
}

func NewMemBuffer() *MemBuffer {
	lines := make([][]rune, 0)
	return &MemBuffer{rows: lines}
}

func (b *MemBuffer) Line(n int) []rune {
	return b.rows[n]
}

func (b *MemBuffer) Len() int {
	return len(b.rows)
}

func (b *MemBuffer) SetLine(n int, line []rune) {
	b.rows[n] = line
}

func (b *MemBuffer) AppendLine(line []rune) {
	b.rows = append(b.rows, line)
}

func (b *MemBuffer) LineLen(n int) int {
	return len(b.rows[n])
}

func (b *MemBuffer) Rune(line, column int) rune {
	return b.rows[line][column]
}
