package zedit

import (
	"image/color"

	"github.com/phrozen/blend"
)

type BlendMode int

const (
	BlendColor BlendMode = iota + 1
	BlendColorBurn
	BlendColorDodge
	BlendDarken
	BlendDarkerColor
	BlendDifference
	BlendDivide
	BlendExclusion
	BlendHardLight
	BlendHardMix
	BlendHue
	BlendLighten
	BlendLighterColor
	BlendLinearBurn
	BlendLinearDodge
	BlendLinearLight
	BlendLuminosity
	BlendMultiply
	BlendOverlay
	BlendPhoenix
	BlendPinLight
	BlendReflex
	BlendSaturation
	BlendScreen
	BlendSoftLight
	BlendSubstract
	BlendVividLight
)

func BlendColors(blending BlendMode, switched bool, c1, c2 color.Color) color.Color {
	if switched {
		tmp := c2
		c2 = c1
		c1 = tmp
	}
	var c color.Color
	switch blending {
	case BlendColorBurn:
		c = blend.ColorBurn(c2, c1)
	case BlendColorDodge:
		c = blend.ColorDodge(c2, c1)
	case BlendDarken:
		c = blend.Darken(c2, c1)
	case BlendDarkerColor:
		c = blend.DarkerColor(c2, c1)
	case BlendDifference:
		c = blend.Difference(c2, c1)
	case BlendDivide:
		c = blend.Divide(c2, c1)
	case BlendExclusion:
		c = blend.Exclusion(c2, c1)
	case BlendHardLight:
		c = blend.HardLight(c2, c1)
	case BlendHardMix:
		c = blend.HardMix(c2, c1)
	case BlendHue:
		c = blend.Hue(c2, c1)
	case BlendLighten:
		c = blend.Lighten(c2, c1)
	case BlendLighterColor:
		c = blend.LighterColor(c2, c1)
	case BlendLinearBurn:
		c = blend.LinearBurn(c2, c1)
	case BlendLinearDodge:
		c = blend.LinearDodge(c2, c1)
	case BlendLinearLight:
		c = blend.LinearLight(c2, c1)
	case BlendLuminosity:
		c = blend.Luminosity(c2, c1)
	case BlendMultiply:
		c = blend.Multiply(c2, c1)
	case BlendOverlay:
		c = blend.Overlay(c2, c1)
	case BlendPhoenix:
		c = blend.Phoenix(c2, c1)
	case BlendPinLight:
		c = blend.PinLight(c2, c1)
	case BlendReflex:
		c = blend.Reflex(c2, c1)
	case BlendSaturation:
		c = blend.Saturation(c2, c1)
	case BlendScreen:
		c = blend.Screen(c2, c1)
	case BlendSoftLight:
		c = blend.SoftLight(c2, c1)
	case BlendSubstract:
		c = blend.Substract(c2, c1)
	case BlendVividLight:
		c = blend.VividLight(c2, c1)
	default:
		c = blend.Color(c2, c1)
	}
	return c
}
