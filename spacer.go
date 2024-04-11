package zedit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// FixedSpacer is a fyne widget that can be used a fixed-size dummy element that is not displayed.
// This is used in a hack to make an invisible widget for the scrollbar
// of the size of the number of rows in the editor widget, which is fixed
// size internally and only changes the display of lines when they are scrolled
// in and out of the view.
type FixedSpacer struct {
	widget.BaseWidget
	size fyne.Size
}

// NewFixedSpacer creates a spacer of the given fized size. It will not change its size
// except when ChangeSize is called.
func NewFixedSpacer(size fyne.Size) *FixedSpacer {
	s := FixedSpacer{size: size}
	return &s
}

// Size returns the size of the spacer.
func (s *FixedSpacer) Size() fyne.Size {
	return s.size
}

// MinSize returns the minimum size of the spacer, which is the same as its size.
func (s *FixedSpacer) MinSize() fyne.Size {
	return s.size
}

// ChangeSize can be used to change the spacer's size, so it reports this size to
// widgets that embed it such as the scrollbar.
func (s *FixedSpacer) ChangeSize(size fyne.Size) {
	s.size = size
}

// SetHeight sets the height of the spacer only. This is is used when the spacer
// is used as a dummy for a scrollbar.
func (s *FixedSpacer) SetHeight(height float32) {
	if s != nil {
		s.size = fyne.Size{Width: s.size.Width, Height: height}
	}
}

// CreateRenderer creates the fixed spacer renderer.
func (s *FixedSpacer) CreateRenderer() fyne.WidgetRenderer {
	return &FixedSpacerRenderer{s}
}

// FixedSpacerRenderer is a renderer for fixed spacer.
type FixedSpacerRenderer struct {
	spacer *FixedSpacer
}

// Destroy destroys the renderer.
func (r *FixedSpacerRenderer) Destroy() {}

// Layout is the renderer layout procedure, which does nothing (a spacer is invisible).
func (r *FixedSpacerRenderer) Layout(size fyne.Size) {}

// MinSize returns the spacer's minimum size.
func (r *FixedSpacerRenderer) MinSize() fyne.Size {
	return r.spacer.size
}

// Objects is needed for a renderer, but returns an empty array of CanvasObject for a spacer.
func (r *FixedSpacerRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{}
}

// Refresh does nothing, since a fixed spacer is not displayed by itself.
func (r *FixedSpacerRenderer) Refresh() {}
