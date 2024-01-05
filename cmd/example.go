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
	ed := zedit.NewZGrid(80, 40)
	ed.ShowLineNumbers = true
	s := ""
	for i := 0; i < 1000; i++ {
		s += lorem.Sentence(5, 11)
		s += "\n"
	}
	ed.SetText(s)
	w.SetContent(ed.Content())
	// w.Canvas().Focus(ed)
	// w.Resize(fyne.NewSize(480, 360))

	w.ShowAndRun()
}
