package main

import (
	"fmt"

	"fyne.io/fyne/v2/app"
	lorem "github.com/drhodes/golorem"
	"github.com/rasteric/zedit-fyne"
)

func main() {
	a := app.New()
	w := a.NewWindow("Example")
	// w.SetFixedSize(true)
	ed := zedit.NewZedit(80, 40)
	s := ""
	for i := 0; i < 10000; i++ {
		s += fmt.Sprintf("%05d %v", i+1, lorem.Sentence(5, 11))
		s += "\n"
	}
	ed.SetText(s)
	w.SetContent(ed.Content())
	// w.Resize(fyne.NewSize(480, 360))
	w.ShowAndRun()
}
