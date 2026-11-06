package main

import (
	"github.com/egoist/mygo/ui"
)

// Icons are Lucide-shaped 16pt glyphs in the current text color.
var (
	iconCompose = svg(`<path d="M2.5 8.5 13.5 2.5l-3 11-3.2-4.2L2.5 8.5z"/><path d="M13.5 2.5 7.3 9.3"/>`)
	iconInspect = svg(`<path d="M1.5 8.5h3l2-5 3 10 2-5h3"/>`)
	iconPlus    = svg(`<path d="M8 3v10M3 8h10"/>`)
	iconTrash   = svg(`<path d="M2.5 4.5h11M6 4.5V2.8h4v1.7M4 4.5l.7 9h6.6l.7-9"/>`)
	iconGear    = svg24(`<path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/>`)
	iconMore    = svg24(`<circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/>`)
	iconFile    = svg24(`<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/>`)
	iconSun     = svg24(`<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2m-7.07-17.07 1.41 1.41m11.32 11.32 1.41 1.41M2 12h2m16 0h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"/>`)
	iconMoon    = svg24(`<path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/>`)
	iconMonitor = svg24(`<rect width="20" height="14" x="2" y="3" rx="2"/><path d="M8 21h8M12 17v4"/>`)
	iconValid   = svg24(`<circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/>`)
	iconCancel  = svg24(`<circle cx="12" cy="12" r="10"/><path d="m15 9-6 6M9 9l6 6"/>`)
	iconLayers  = svg24(`<path d="m12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83z"/><path d="m22 17.65-9.17 4.16a2 2 0 0 1-1.66 0L2 17.65"/><path d="m22 12.65-9.17 4.16a2 2 0 0 1-1.66 0L2 12.65"/>`)
	iconImport  = svg24(`<path d="M12 3v12"/><path d="m8 11 4 4 4-4"/><path d="M8 5H4a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-4"/>`)
	iconExport  = svg24(`<path d="M12 15V3"/><path d="m8 7 4-4 4 4"/><path d="M8 9H4a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-8a2 2 0 0 0-2-2h-4"/>`)
	iconSend    = svg(`<path d="M14 2 7.2 8.8M14 2 9.7 14l-2.5-5.2L2 6.3 14 2z"/>`)
	iconSave    = svg(`<path d="M2.5 2.5h9L14 5v8.5h-11.5z"/><path d="M5 2.5V6h6V2.5M5 13.5V9h6v4.5"/>`)
	iconCopy    = svg(`<rect x="5.5" y="5.5" width="8" height="8" rx="1.4"/><path d="M10.5 5.5v-1a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v4a2 2 0 0 0 2 2h1"/>`)
	iconCheck   = svg(`<path d="m3 8.5 3.2 3L13 4.5"/>`)
	iconX       = svg(`<path d="m4 4 8 8M12 4l-8 8"/>`)
	iconChevron = svg(`<path d="m4.5 6 3.5 3.5L11.5 6"/>`)
	iconRight   = svg(`<path d="m6 4.5 3.5 3.5L6 11.5"/>`)
	iconLock    = svg(`<rect x="3" y="7" width="10" height="7" rx="1.5"/><path d="M5.5 7V5a2.5 2.5 0 0 1 5 0v2"/>`)
	iconGlobe   = svg(`<circle cx="8" cy="8" r="6"/><path d="M2 8h12M8 2c1.7 1.7 2.5 3.7 2.5 6S9.7 12.3 8 14C6.3 12.3 5.5 10.3 5.5 8S6.3 3.7 8 2z"/>`)
	iconDown    = svg(`<path d="M8 2v8.5M4.5 7.5 8 11l3.5-3.5M2.5 13.5h11"/>`)
	iconFolder  = svg(`<path d="M2 4.5A1.5 1.5 0 0 1 3.5 3h2.2l1.4 1.6h5.4A1.5 1.5 0 0 1 14 6.1v6.4a1.5 1.5 0 0 1-1.5 1.5h-9A1.5 1.5 0 0 1 2 12.5z"/>`)
	iconKey     = svg(`<circle cx="5.5" cy="10.5" r="2.5"/><path d="m7.4 8.6 5-5M10.2 3.8l1.5 1.5M11.7 2.3l1.5 1.5"/>`)
	iconSpark   = svg(`<path d="m8 1.5 1.6 4.9 4.9 1.6-4.9 1.6L8 14.5l-1.6-4.9L1.5 8l4.9-1.6z"/>`)
	iconServer  = svg(`<rect x="2" y="2.5" width="12" height="4.5" rx="1.3"/><rect x="2" y="9" width="12" height="4.5" rx="1.3"/><path d="M4.5 4.7h.01M4.5 11.2h.01"/>`)
	iconShield  = svg(`<path d="M8 1.8 13 3.5v4.2c0 3-2.1 5.3-5 6.5-2.9-1.2-5-3.5-5-6.5V3.5z"/><path d="m5.5 8 1.8 1.8 3.4-3.4"/>`)
	iconCode    = svg(`<path d="m5 4-4 4 4 4M11 4l4 4-4 4M9.5 2.5l-3 11"/>`)
	iconWrap    = svg24(`<path d="M3 6h18"/><path d="M3 12h15a3 3 0 1 1 0 6h-5"/><path d="m16 15-3 3 3 3"/><path d="M3 18h7"/>`)
	iconFilter  = svg(`<path d="M2 3h12L9 8.5V13l-2-1V8.5z"/>`)
	iconSearch  = svg(`<circle cx="7" cy="7" r="4.5"/><path d="m10.4 10.4 3.6 3.6"/>`)
	iconPlay    = svg(`<path d="M5 3.2 12.5 8 5 12.8z"/>`)
	iconStop    = svg(`<rect x="4" y="4" width="8" height="8" rx="1.2"/>`)
	iconWarning = svg(`<path d="M8 2 14.5 13.5h-13z"/><path d="M8 6v3.5M8 11.7h.01"/>`)
)

func svg(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

// svg24 draws a glyph written on Lucide's own 24-unit grid in the same
// 16-unit frame, at the same stroke as the others.
func svg24(body string) *ui.SVG {
	return svg(`<g transform="scale(0.6666667)" stroke-width="2.025">` + body + `</g>`)
}
