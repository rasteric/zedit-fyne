package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
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
	ed.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl},
		func(z *zedit.Editor) {
			dialog.ShowFileSave(func(uri fyne.URIWriteCloser, err error) {
				if uri == nil || err != nil {
					return
				}
				defer uri.Close()
				if err := z.Save(uri); err != nil {
					dialog.ShowError(err, w)
				}
			}, w)
		})
	ed.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierControl},
		func(z *zedit.Editor) {
			dialog.ShowFileOpen(func(uri fyne.URIReadCloser, err error) {
				if uri == nil || err != nil {
					return
				}
				defer uri.Close()
				if err := z.Load(uri); err != nil {
					dialog.ShowError(err, w)
				}
			}, w)
		})
	ed.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.KeyL, Modifier: fyne.KeyModifierControl},
		func(z *zedit.Editor) {
			dialog.ShowFileOpen(func(uri fyne.URIReadCloser, err error) {
				if uri == nil || err != nil {
					return
				}
				defer uri.Close()
				if err := z.LoadText(uri); err != nil {
					dialog.ShowError(err, w)
				}
			}, w)
		})
	ed.Focus()
	w.ShowAndRun()
}
