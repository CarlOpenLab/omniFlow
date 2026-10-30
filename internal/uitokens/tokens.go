// Package uitokens is where OmniFlow's colors, materials and the
// measurements of its chrome are written down. The desktop window reads
// them; the app is native UI only, so there is no second copy to drift.
package uitokens

import "github.com/egoist/mygo/ui"

// Palette is the chrome's colors, for one appearance. It follows the
// acrylic rules (the mygo-acrylic skill): the window is the system's frosted
// material, the workspace card lets it through, and everything on the card
// or the sidebar lets the card through.
//
// Read it by one rule. The layers (Ground, Card, Popover, CodeBg, and the
// framework's own control faces) are opaque, and are only ever the ground of
// a layer. The washes (RowHover, Chip, WashSelected, Kbd) and the hairlines
// are translucent, and are the only grounds a control or a strip on the card
// or the sidebar takes: an opaque color there would cut a solid block out of
// the frost, invisible at full opacity and plain as the card is turned down.
// TestCardAndSidebarUseWashes in cmd/omniflow holds the line.
type Palette struct {
	Dark bool

	// Layers: opaque.
	//
	// Ground is the window's ground, veiled over the material (GroundVeil)
	// and painted whole where there is none. Card is the workspace card,
	// times the opacity the person chose (CardOpacity). Popover is the
	// dialogs and popovers, and CodeBg the code canvas: bodies, headers and
	// the body editor, which stay solid so code keeps its contrast. Control
	// and ControlHover are the framework's own control faces, which only the
	// dialogs show.
	Ground       ui.Color
	Card         ui.Color
	Popover      ui.Color
	CodeBg       ui.Color
	Control      ui.Color
	ControlHover ui.Color

	// Washes: translucent, light to heavy. A state only climbs this ladder,
	// transparent → RowHover → Chip → WashSelected, and never changes the
	// geometry. Kbd is a shortcut's face.
	RowHover     ui.Color
	Chip         ui.Color
	WashSelected ui.Color
	Kbd          ui.Color
	KbdBorder    ui.Color

	// Hairlines: 1px. Divider and Edge part and outline; EdgeStrong
	// outlines a field and a hovered or chosen control; EdgeHigh rims a
	// field under the pointer and a floating panel.
	Divider    ui.Color
	Edge       ui.Color
	EdgeStrong ui.Color
	EdgeHigh   ui.Color

	// Ink.
	Text      ui.Color
	TextMuted ui.Color
	TextFaint ui.Color

	// Accent and signals. AccentSoft is the chosen row's band; Selection
	// the highlight behind chosen text.
	Accent      ui.Color
	AccentHover ui.Color
	OnAccent    ui.Color
	AccentSoft  ui.Color
	Selection   ui.Color
	Success     ui.Color
	Warning     ui.Color
	Danger      ui.Color
	Info        ui.Color

	// The code canvas's ink: JSON tokens in the body viewer.
	CodeText    ui.Color
	CodeEdge    ui.Color
	CodeKey     ui.Color
	CodeString  ui.Color
	CodeNumber  ui.Color
	CodeLiteral ui.Color
	CodePunct   ui.Color

	// Lift. CardBorder is the light card's hard edge, drawn as a shadow so it
	// does not push the content in as a border would.
	CardBorder  ui.Color
	CardShadow  ui.Color
	CardShadow2 ui.Color
	FloatShadow ui.Color
	Overlay     ui.Color
}

// Light is the light appearance: the prototype's cool, quiet grey-blue,
// with white cards above it.
var Light = Palette{
	Ground:       ui.Hex("#e6ecf5"),
	Card:         ui.Hex("#ffffff"),
	Popover:      ui.Hex("#ffffff"),
	CodeBg:       ui.Hex("#f7f8fb"),
	Control:      ui.Hex("#ffffff"),
	ControlHover: ui.Hex("#f5f6f9"),

	RowHover:     ui.RGBA(0, 0, 0, 0.04),
	Chip:         ui.RGBA(0, 0, 0, 0.045),
	WashSelected: ui.RGBA(0, 0, 0, 0.07),
	Kbd:          ui.RGBA(0, 0, 0, 0.05),
	KbdBorder:    ui.RGBA(0, 0, 0, 0.09),

	Divider:    ui.RGBA(0, 0, 0, 0.06),
	Edge:       ui.RGBA(0, 0, 0, 0.07),
	EdgeStrong: ui.RGBA(0, 0, 0, 0.10),
	EdgeHigh:   ui.RGBA(0, 0, 0, 0.14),

	Text:      ui.Hex("#1d1d1f"),
	TextMuted: ui.Hex("#6e6e78"),
	TextFaint: ui.Hex("#9595a1"),

	Accent:      ui.Hex("#3b6cf6"),
	AccentHover: ui.Hex("#2f5bd8"),
	OnAccent:    ui.Hex("#ffffff"),
	AccentSoft:  ui.RGBA(59, 108, 246, 0.12),
	Selection:   ui.RGBA(59, 108, 246, 0.25),
	Success:     ui.Hex("#22863a"),
	Warning:     ui.Hex("#b08800"),
	Danger:      ui.Hex("#d3302f"),
	Info:        ui.Hex("#2f5bd8"),

	CodeText:    ui.Hex("#26262b"),
	CodeEdge:    ui.RGBA(0, 0, 0, 0.055),
	CodeKey:     ui.Hex("#7b3fc4"),
	CodeString:  ui.Hex("#1c7a45"),
	CodeNumber:  ui.Hex("#b45309"),
	CodeLiteral: ui.Hex("#0f8199"),
	CodePunct:   ui.Hex("#8a8a95"),

	CardBorder:  ui.RGBA(0, 0, 0, 0.07),
	CardShadow:  ui.RGBA(0, 0, 0, 0.07),
	CardShadow2: ui.RGBA(0, 0, 0, 0.035),
	FloatShadow: ui.RGBA(20, 20, 40, 0.14),
	Overlay:     ui.RGBA(12, 12, 28, 0.18),
}

// Dark is the dark appearance.
var Dark = Palette{
	Dark:         true,
	Ground:       ui.Hex("#141418"),
	Card:         ui.Hex("#232328"),
	Popover:      ui.Hex("#26262c"),
	CodeBg:       ui.Hex("#1b1b21"),
	Control:      ui.Hex("#3a3a42"),
	ControlHover: ui.Hex("#45454e"),

	RowHover:     ui.RGBA(255, 255, 255, 0.05),
	Chip:         ui.RGBA(255, 255, 255, 0.07),
	WashSelected: ui.RGBA(255, 255, 255, 0.10),
	Kbd:          ui.RGBA(255, 255, 255, 0.07),
	KbdBorder:    ui.RGBA(255, 255, 255, 0.11),

	Divider:    ui.RGBA(255, 255, 255, 0.07),
	Edge:       ui.RGBA(255, 255, 255, 0.075),
	EdgeStrong: ui.RGBA(255, 255, 255, 0.11),
	EdgeHigh:   ui.RGBA(255, 255, 255, 0.16),

	Text:      ui.Hex("#ececf0"),
	TextMuted: ui.Hex("#a9a9b4"),
	TextFaint: ui.Hex("#8c8c99"),

	Accent:      ui.Hex("#6b93ff"),
	AccentHover: ui.Hex("#7ea2ff"),
	OnAccent:    ui.Hex("#101422"),
	AccentSoft:  ui.RGBA(107, 147, 255, 0.16),
	Selection:   ui.RGBA(107, 147, 255, 0.25),
	Success:     ui.Hex("#4fd17a"),
	Warning:     ui.Hex("#f2c14e"),
	Danger:      ui.Hex("#ff6b64"),
	Info:        ui.Hex("#6b9bff"),

	CodeText:    ui.Hex("#e4e4ea"),
	CodeEdge:    ui.RGBA(255, 255, 255, 0.06),
	CodeKey:     ui.Hex("#c79cf7"),
	CodeString:  ui.Hex("#7fd9a1"),
	CodeNumber:  ui.Hex("#f2c14e"),
	CodeLiteral: ui.Hex("#6fd7e8"),
	CodePunct:   ui.Hex("#8d8d99"),

	CardBorder:  ui.RGBA(255, 255, 255, 0.08),
	CardShadow:  ui.RGBA(0, 0, 0, 0.35),
	CardShadow2: ui.RGBA(0, 0, 0, 0.20),
	FloatShadow: ui.RGBA(0, 0, 0, 0.45),
	Overlay:     ui.RGBA(0, 0, 0, 0.38),
}

// WashKind is where a control rests on the wash ladder.
type WashKind uint8

const (
	// Ghost rests transparent: icon buttons, rows, a segment.
	Ghost WashKind = iota
	// Filled rests on Chip: buttons, fields, a segmented control's track.
	Filled
)

// Wash is a control's ground in this frame. State only climbs the ladder:
//
//	          rest      hovered        pressed / chosen / under way
//	Ghost     clear     RowHover       WashSelected
//	Filled    Chip      WashSelected   WashSelected
func (p *Palette) Wash(k WashKind, hovered, active bool) ui.Color {
	switch {
	case active, hovered && k == Filled:
		return p.WashSelected
	case hovered:
		return p.RowHover
	case k == Filled:
		return p.Chip
	}
	return ui.Transparent
}

// EdgeFor is a filled control's 1px edge: Edge at rest, EdgeStrong hovered
// or active. The border always takes its pixel and only changes color, so
// a state never moves the layout.
func (p *Palette) EdgeFor(hovered, active bool) ui.Color {
	if hovered || active {
		return p.EdgeStrong
	}
	return p.Edge
}

// GroundVeil is how opaque the ground's tint is over the material. The
// material follows the desktop, not the app's appearance: at its brightest
// the veil still holds the sidebar's muted text above 4.5:1.
const GroundVeil = 0.65

// GroundColor is the window's ground in this frame: a veil over the
// material, or the ground itself where there is none.
func (p *Palette) GroundColor(material bool) ui.Color {
	if material {
		return p.Ground.Alpha(GroundVeil)
	}
	return p.Ground
}

// The workspace card's opacity, which the person sets; the card is the one
// layer that has any. Below 30% the text on it has no steady contrast.
const (
	OpacityMin     = 0.30
	OpacityMax     = 1.0
	OpacityDefault = 0.80
)

// NormalizeOpacity reads 0 (never set) as the default, and holds the rest
// within [OpacityMin, OpacityMax].
func NormalizeOpacity(v float64) float64 {
	if v <= 0 {
		return OpacityDefault
	}
	return min(max(v, OpacityMin), OpacityMax)
}

// CardFill is the workspace card's ground at an opacity.
func (p *Palette) CardFill(opacity float64) ui.Color {
	return p.Card.Alpha(float32(NormalizeOpacity(opacity)))
}

// For returns the palette of an appearance.
func For(dark bool) *Palette {
	if dark {
		return &Dark
	}
	return &Light
}

// Method colors, as every HTTP client colors its verbs. The prototype's
// palette, one step deeper in the light so it reads on white.
var (
	MethodGetLight    = ui.Hex("#2f5bd8")
	MethodPostLight   = ui.Hex("#22863a")
	MethodPutLight    = ui.Hex("#b08800")
	MethodDeleteLight = ui.Hex("#d3302f")
	MethodPatchLight  = ui.Hex("#a033c4")

	MethodGetDark    = ui.Hex("#6b9bff")
	MethodPostDark   = ui.Hex("#4fd17a")
	MethodPutDark    = ui.Hex("#f2c14e")
	MethodDeleteDark = ui.Hex("#ff6b64")
	MethodPatchDark  = ui.Hex("#d07cf0")
)

// MethodColor is the color of a verb, muted if the palette is dark or not.
func MethodColor(p *Palette, method string) ui.Color {
	switch method {
	case "GET":
		if p.Dark {
			return MethodGetDark
		}
		return MethodGetLight
	case "POST":
		if p.Dark {
			return MethodPostDark
		}
		return MethodPostLight
	case "PUT":
		if p.Dark {
			return MethodPutDark
		}
		return MethodPutLight
	case "PATCH":
		if p.Dark {
			return MethodPatchDark
		}
		return MethodPatchLight
	case "DELETE":
		if p.Dark {
			return MethodDeleteDark
		}
		return MethodDeleteLight
	}
	return p.TextMuted
}

// StatusColor colors a response status: success, redirection, a client
// mistake and a server one, in the palette's own signals.
func StatusColor(p *Palette, status int) ui.Color {
	switch {
	case status >= 500:
		return p.Danger
	case status >= 400:
		return p.Warning
	case status >= 300:
		return p.Info
	case status >= 200:
		return p.Success
	}
	return p.TextMuted
}

// The measurements of the chrome, in DIPs. BarMid is the line the top
// bar's contents center on; the sidebar's header and the card's line up
// with it, and the traffic lights sit on it.
const (
	BarMid  = 30
	TopBar  = 2 * BarMid
	CardGap = 8
	HeaderH = 44

	SidebarWidth = 272
	MinSidebar   = 232
	MaxSidebar   = 340

	// CardRadius is a floating card's corner: a dialog, a popover. The
	// workspace card takes its radius from the window's own corner; see
	// cmd/omniflow/card_radius.go.
	CardRadius = 14
	RowRadius  = 8
	RowH       = 30
	ToolbarH   = 38

	URLBarH = 38
	TabH    = 34
	FootH   = 40
)

// Material is the window's ground where there is no system material to show:
// a vertical gradient in two stops with a glow from the top right. Over the
// material the ground is Palette.Ground's veil instead.
type Material struct {
	Label string
	Light []string
	Dark  []string
	// LightGlow and DarkGlow tint the top right corner; transparent for none.
	LightGlow ui.Color
	DarkGlow  ui.Color
	// Acrylic lets the window's material show through the ground and the
	// card; the sidebar takes no ground of its own.
	Acrylic bool
}

// Acrylic is the app's one background: the cool, quiet tint the system
// material is read through.
var Acrylic = Material{
	Label: "Acrylic",
	Light: []string{"#e6ecf5", "#edf1f8"},
	Dark:  []string{"#141418", "#1a1a22"},

	LightGlow: ui.RGBA(255, 255, 255, 0.45),
	DarkGlow:  ui.RGBA(255, 255, 255, 0.08),

	Acrylic: true,
}

// Base is the ground's gradient.
func (m Material) Base(dark bool) ui.LinearGradient {
	stops := m.Light
	if dark {
		stops = m.Dark
	}
	out := make([]ui.GradientStop, len(stops))
	for i, h := range stops {
		out[i] = ui.GradientStop{Color: ui.Hex(h), At: float32(i) / float32(max(len(stops)-1, 1))}
	}
	return ui.LinearGradient{Angle: 180, Oklab: true, Stops: out}
}

// Glow is the ground's glow from the top right.
func (m Material) Glow(dark bool) ui.LinearGradient {
	c := m.LightGlow
	if dark {
		c = m.DarkGlow
	}
	return ui.LinearGradient{Angle: 225, Oklab: true, Stops: []ui.GradientStop{
		{Color: c, At: 0},
		{Color: c.Alpha(0), At: 0.55},
	}}
}
