package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"github.com/rasteric/zedit-fyne"
)

func main() {
	a := app.New()
	w := a.NewWindow("Example Editor")
	ed := zedit.NewZedit(80, 40)
	w.SetContent(ed)
	w.Resize(fyne.NewSize(480, 360))
	w.ShowAndRun()
}
