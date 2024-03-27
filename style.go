package zedit

import "image/color"

type EditorStyle interface {
	TextColor() color.Color
	BackgroundColor() color.Color
}

type CustomEditorStyle struct {
	FGColor, BGColor color.Color
}

func (s *CustomEditorStyle) TextColor() color.Color {
	return s.FGColor
}

func (s *CustomEditorStyle) BackgroundColor() color.Color {
	return s.BGColor
}
