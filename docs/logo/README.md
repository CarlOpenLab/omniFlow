# OmniFlow icon

Built with the `design-playful-app-icons` system (text-only rules, no reference images).

## Brief

- **Object** — a flow conduit with an inspection porthole.
- **Action** — traffic passes through and is watched.
- **Emotion** — curious, clever, mildly mischievous.
- **Lane** — `soft_3d_object_character`.
- **Hook** — the porthole doubles as an eye: the proxy "sees" every exchange.

## Directions considered

Scores below are the pre-render triad from the skill (product meaning / silhouette / originality).

1. **Inspector's Eye** — a soft-white conduit crosses the tile as a gentle S; a large
   porthole sits at the point of inspection, its off-centre yellow pupil reading as a
   curious eye. Soft 3D object-character. **19 / 19 / 9 = 47/50. Selected.**
2. **Turnstile** — a chunky three-arm turnstile, one arm frozen mid-swing, a single
   lime packet slipping through. Minimal dimensional glyph. 16 / 18 / 6 = 40/50.
3. **Relay** — a rounded router mascot with two loop antennas, a heavy ink contour and
   a face; one arrow-packet passes through a slot in its body. Flat outlined mascot.
   14 / 17 / 6 = 37/50.

Why the first wins: it says *flow* and *inspection* with one object, has the strongest
black silhouette (band + circle), keeps one focal feature at 32 px, and avoids the
router/turnstile clichés that could stand for any network utility.

## Final prompt

Use this when regenerating with an image model. The shipped asset was constructed as
vector geometry (see `icon.svg`), not generated from this prompt; the prompt is the
reproducible text specification.

```text
Square app icon for macOS representing an HTTP workbench where requests are composed
and captured traffic is inspected. One original soft-white flow conduit crosses the
tile as a gentle S, about 200 px wide on a 1024 px canvas, with a large porthole at its
inflection: a 400 px soft-white rim, a cool-grey bezel, deep ink glass, and one
off-centre clear-yellow pupil with a small white specular. The pupil makes the porthole
read as a curious eye watching the flow. The conduit and porthole are built from one
broad rounded band and one circle, filling about 75% of the square, with the band
cropped by the left and right edges through the straight part of the icon tile. Soft
plastic and glass, broad upper-left light, a short diffuse contact shadow under the
conduit and a soft shadow under the porthole. Palette: cobalt #1865EE dominant, sky
#3FA1F9 to violet #4237C6 background gradient, soft white #F8F6F5 neutral, ink #1C212C,
clear yellow #FDDA34 as the single bright accent. Readable at 32 px: the silhouette is
a band with a bold circle, the face is one pupil. No text, no thin strokes, no tiny
decorative details, no multiple subjects, no hard chrome, no excessive glow, no
photorealism, no existing logo, no known character, no artist imitation.
```

## Palette

| Role | Value |
| --- | --- |
| Background gradient | `#3FA1F9` → `#1865EE` → `#4237C6` |
| Conduit (dominant light) | `#FFFFFF` → `#FBFAF8` → `#E8EDF5` → `#C9D5E6` |
| Porthole rim / bezel | `#FFFFFF` / `#BCCBE0` |
| Glass | `#34415A` → `#1C212C` → `#0B111C` |
| Accent (pupil) | `#FDDA34` with `#FFF6C2` specular |
| Shadow | `#0A2A6B` at low opacity |

## Files

- `icon.svg` — master geometry (1024 × 1024 viewBox), editable.
- `icon-1024.png` — production master, sRGB, alpha outside the macOS rounded tile.
- `icon-180.png`, `icon-60.png`, `icon-32.png` — actual-size checks.
- `preview.png` — light/dark launchers, grayscale, palette.
- `../../resources/icon.png` — the copy the app build reads via `mygo.json`.

Rebuild to see it in the Dock:

```sh
sh scripts/build.sh
open out/darwin-arm64/OmniFlow.app
```

## Quality report

```text
Lane: soft 3D object-character
Metaphor: flow conduit + inspect + curious
Palette: blue_trust + clear-yellow accent
Signature hook: porthole-as-eye
Score: 92/100
Small-size result: at 32 px the band + circle silhouette and the yellow pupil survive;
  the S curvature collapses, which is acceptable because it is not the hook.
Production status: ready (macOS 1024 sRGB raster; opaque tile, transparent corners).
```

- 1024 — one material/light model, broad highlights, soft contact shadows, no artefacts.
- 180 — band, rim and gaze read; the porthole/eye dual reading is clearest here.
- 60 — band, eye and pupil read; the band's shadow still separates it from the tile.
- 32 — silhouette and pupil read; all decorative gradient detail is gone.
- Grayscale — white band/rim vs ink glass vs cobalt background: strong separation.
- Black silhouette — band plus circle; one subject, no ambiguous appendages.
- Light / dark launchers — the tile separates from both; the yellow accent survives.
- Edge check — the band exits through the straight left/right edges, so no rounded
  corner is notched; verified at 1024 and in the 32 px downsample.
