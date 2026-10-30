package uitokens

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The washes are translucent and climb from light to heavy; the layers are
// opaque. Either broken, a control cuts a solid block out of the frost, or
// hovering one makes it lighter than at rest.
func TestWashLadder(t *testing.T) {
	for _, p := range []*Palette{&Light, &Dark} {
		ladder := []struct {
			name string
			c    ui.Color
		}{{"RowHover", p.RowHover}, {"Chip", p.Chip}, {"WashSelected", p.WashSelected}}
		for i, w := range ladder {
			if w.c.A == 0 || w.c.A == 255 {
				t.Errorf("dark=%v: %s has alpha %d; a wash is translucent", p.Dark, w.name, w.c.A)
			}
			if i > 0 && w.c.A < ladder[i-1].c.A {
				t.Errorf("dark=%v: %s is lighter than %s", p.Dark, w.name, ladder[i-1].name)
			}
		}
		for name, c := range map[string]ui.Color{
			"Ground": p.Ground, "Card": p.Card, "Popover": p.Popover, "CodeBg": p.CodeBg,
		} {
			if c.A != 255 {
				t.Errorf("dark=%v: the layer %s has alpha %d; layers are opaque", p.Dark, name, c.A)
			}
		}
		for name, c := range map[string]ui.Color{
			"Divider": p.Divider, "Edge": p.Edge, "EdgeStrong": p.EdgeStrong, "EdgeHigh": p.EdgeHigh,
			"Kbd": p.Kbd, "AccentSoft": p.AccentSoft, "Selection": p.Selection,
		} {
			if c.A == 255 {
				t.Errorf("dark=%v: %s is opaque; it is drawn over the card", p.Dark, name)
			}
		}
		if p.Wash(Ghost, false, false) != ui.Transparent || p.Wash(Ghost, true, false) != p.RowHover ||
			p.Wash(Filled, false, false) != p.Chip || p.Wash(Filled, true, false) != p.WashSelected ||
			p.Wash(Ghost, false, true) != p.WashSelected {
			t.Errorf("dark=%v: Wash does not climb the ladder", p.Dark)
		}
	}
}

// Over the material the ground is a veil, and dark over the brightest
// material (about #898c8d, measured on a light desktop) it still holds the
// muted text at 4.5:1.
func TestGroundVeil(t *testing.T) {
	p := &Dark
	if p.GroundColor(false) != p.Ground {
		t.Fatal("without a material the ground is painted whole")
	}
	veil := p.GroundColor(true)
	if veil != p.Ground.Alpha(GroundVeil) {
		t.Fatalf("over a material the ground is Ground × %.2f", GroundVeil)
	}
	if r := contrast(veil.Over(ui.Hex("#898c8d")), p.TextMuted); r < 4.5 {
		t.Errorf("muted text over the veiled bright material is %.1f:1, under 4.5:1", r)
	}
}

func TestOpacity(t *testing.T) {
	for in, want := range map[float64]float64{0: OpacityDefault, -1: OpacityDefault, 0.1: OpacityMin, 2: OpacityMax, 0.55: 0.55} {
		if got := NormalizeOpacity(in); got != want {
			t.Errorf("NormalizeOpacity(%v) = %v, want %v", in, got, want)
		}
	}
	if got := Light.CardFill(1); got != Light.Card {
		t.Errorf("the card at 100%% is %v, want the card itself", got)
	}
}

func contrast(a, b ui.Color) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(c ui.Color) float64 {
	ch := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c.R) + 0.7152*ch(c.G) + 0.0722*ch(c.B)
}
