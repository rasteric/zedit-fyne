package main

import (
	"fyne.io/fyne/v2/app"
	lorem "github.com/drhodes/golorem"
	"github.com/rasteric/zedit-fyne"
)

func main() {
	a := app.New()
	w := a.NewWindow("Example")
	// w.SetFixedSize(true)
	ed := zedit.NewEditor(80, 40, w.Canvas())
	ed.Config.ShowLineNumbers = true

	ed.AddEmacsShortcuts()
	s := ""
	for i := 0; i < 100; i++ {
		s += lorem.Sentence(5, 30)
		s += "\n"
	}
	s = s[:len(s)-1]
	ed.SetText(s)
	w.SetContent(ed)
	li, _ := ed.ParaToLine(10)
	li2 := ed.FindParagraphEnd(li, ed.Config.HardLF)
	ed.Select(zedit.CharInterval{Start: zedit.CharPos{Line: li, Column: 0},
		End: zedit.CharPos{Line: li2, Column: ed.LastColumn(li2)}})
	ed.Focus()
	w.ShowAndRun()
}
