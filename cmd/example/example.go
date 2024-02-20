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
	ed := zedit.NewZGrid(80, 40, w.Canvas())
	ed.ShowLineNumbers = true
	ed.AddEmacsShortcuts()
	s := ""
	for i := 0; i < 100; i++ {
		s += lorem.Sentence(5, 15)
		s += "\n"
	}
	s = s[:len(s)-1]
	ed.SetText(s)
	w.SetContent(ed)
	w.Canvas().Focus(ed)
	// w.Resize(fyne.NewSize(480, 360))

	w.ShowAndRun()
}
