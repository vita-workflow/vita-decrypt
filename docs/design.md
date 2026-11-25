# Design

What the Vita Decrypt desktop app looks like, and the rules for changing it. The app is part of the Vita family, so
it follows **Vita's design system**: its color tokens, fonts, radii, and accessibility rules. When they change, this
app follows.

## Look and feel

- **Calm and quiet.** People open this app when they need a file back, sometimes in a stressful moment. Screens do
  one job each, say what will happen in plain words, and keep the next step obvious.
- **Life, not alarm.** Vita's accent is a vine or tree green that stands for life. The app uses it for the primary
  action, the selected screen, links, and focus. Red is only for something that went wrong.
- **Soft, never harsh.** Off-white and charcoal-green neutrals, never pure white or pure black. Cards sit on a
  slightly darker canvas with a thin border rather than a shadow.
- **Reassurance up front.** The Open screen leads with a dark forest-green card: "Works offline", "Open your sealed
  files", "Your private key never leaves this computer." That card and the About dialog's backdrop are the only
  forest surfaces.
- **Botanical identity.** The sidebar's wordmark is the Lucide Sprout icon, "Vita" in Fraunces, and a small
  "DECRYPT". The app icon is the same sprout on the fixed Vita green (`#2E6F40`).

## Color

The palette is `cmd/vita-decrypt-gui/theme.go`. It uses Vita's token values unchanged. Use the token, never
a raw hex value in a screen.

| Token | Light | Dark | Use |
|-------|-------|------|-----|
| `canvas` | `#F4F7F1` | `#101511` | Window background. |
| `surface` | `#FBFCF8` | `#171E19` | Cards, fields, the sidebar, the About dialog. |
| `surfaceAlt` | `#EEF4EA` | `#1F2921` | Hover fills, file rows, read-only values. |
| `foreground` | `#1A3D25` | `#EDF6ED` | Body text. |
| `muted` | `#4A6655` | `#A3C9B0` | Secondary text and captions. |
| `border` | `#D3E2D8` | `#243D2E` | Decorative dividers and card edges only. |
| `borderStrong` | `#6F8A78` | `#5D7A68` | Field and checkbox edges: anything you can focus or type in. |
| `primary` | `#2E6F40` | `#4FBE64` | Primary button fill and the selected sidebar item. |
| `onPrimary` | `#FBFDF9` | `#101511` | Text on `primary`. Dark mode uses dark text on bright green. |
| `primaryHover` | `#256532` | `#62D778` | Primary button hover. |
| `primaryText` | `#2E6F40` | `#4FBE64` | Links and brand-colored text. |
| `ring` | `#2E6F40` | `#4FBE64` | Focus ring: 2 px, drawn outside the control. |
| `forest` | `#1A3D25` | `#143024` | The Open screen's hero card. |
| `forestDeep` | `#143024` | `#0D1F16` | The About dialog's scrim. |
| `forestForeground` | `#EEF4EA` | `#EEF4EA` | Text on forest. |
| `fern` | `#62D778` | `#74E389` | Accents on forest only. |

**Status colors** (`success`, `warning`, `danger`, each with a fill and a border) are Fluent UI's palette values, as
in Vita. Vita's gold means "unread notification" only, so this app, which has no notifications, doesn't use it.

**Blending.** Gio blends translucent colors in linear light, which looks lighter than the same opacity in a browser.
`blend` and `translucent` in `theme.go` reproduce the CSS result, so hover tints and the scrim match Vita in a browser.

## Type

The same two families as Vita, bundled in `cmd/vita-decrypt-gui/fonts` (SIL Open Font License 1.1): **Fraunces**
for display text and **Inter** for everything else. Sizes are in dp.

| Style | Font | Size / line height | Use |
|-------|------|--------------------|-----|
| `styleHero` | Fraunces SemiBold | 32 / 38 | The Open screen's hero heading |
| `styleTitle` | Fraunces SemiBold | 28 / 34 | Screen titles |
| `styleWordmark` | Fraunces Bold | 25 / 25 | "Vita" in the sidebar |
| `styleFingerprint` | Inter SemiBold, tracked | 24 / 32 | Fingerprints shown large |
| `styleSection` | Inter SemiBold | 16 / 22 | Card and section headings |
| `styleBody`, `styleButton`, `styleNavigation` | Inter | 14 / 20 | Body text, buttons, sidebar items |
| `styleSmall` | Inter | 13 / 20 | Help text under fields |
| `styleCaption` | Inter | 12 / 18 | Captions |
| `styleEyebrow` | Inter SemiBold, uppercase, tracked | 12 / 16 | Small labels above headings |
| `stylePill` | Inter SemiBold, uppercase, tracked | 10 / 16 | Pills such as the key type |

Fingerprints are shown in four groups of four, as Vita shows them, so people can compare them a group at a time.

## Shape and spacing

- **Radii** follow Vita: **14** for cards, sidebar items, and badges; **20** for large cards and dialogs (the hero
  card, the About dialog); fully round (999) for pills. Fields, buttons, and messages use **10**.
- **Borders** are 1 px. Shadows aren't used.
- **Window:** opens at 960 × 740 and can't be made smaller than 720 × 560. The sidebar is fixed on the left; the
  content scrolls.

## Icons

[Lucide](https://lucide.dev) icons (ISC license), kept as SVG in `cmd/vita-decrypt-gui/icons` and drawn from their
paths at the stroke width the design asks for. To add one, copy its SVG from Lucide into that folder and refer to it
by file name. Don't draw icons by hand or mix in another icon set.

## Components

- **Buttons:** primary (green fill, one per screen, for the main action), secondary (surface with a border), link
  (green text, for "Choose file…" and "Paste"), and icon buttons.
- **Fields** have a `borderStrong` edge, turn to the focus ring when focused, and show help or an error below.
  Public key fields always offer Paste and "Choose file…", so nobody has to type a key.
- **Messages** (success, warning, danger) always pair an icon with the text (circle-check, triangle-alert,
  circle-x), so meaning never depends on color alone.
- **Copy buttons** change to a check and "Copied" for a moment.
- **Drop zones** on the Open and Seal screens highlight while files are dragged over the window (Windows and macOS).

## Behavior

- **Nothing is overwritten,** and the screens say so before you act: an output that already exists is reported, not
  replaced.
- **Long work shows progress** and keeps the window responsive.
- **Creating a key pair** ends on a screen that shows the public key and fingerprint and won't finish until the person
  ticks that they've backed up the private key.
- **Theme:** light by default, with a light/dark switch in the sidebar that's remembered. Vita also offers
  "follow the system"; this app doesn't yet.

## Accessibility

Vita's rules apply:

- Text meets WCAG AA contrast (4.5 : 1), and field edges, focus rings, and other indicators meet 3 : 1. The token
  values above were chosen to pass, and Vita's own tests check them.
- Everything can be reached and used with the keyboard, with a visible focus ring. Enter submits a form.
- Status is never shown by color alone.

## Changing the design

1. If the change is to a token, font, or radius, change it in Vita's design system first (or confirm it's there), then
   mirror it in `theme.go` or `text.go`. A color that exists only in this app needs a reason in the pull request.
2. Check both themes: `VITA_GUI_SHOTS=DIR go test ./cmd/vita-decrypt-gui` renders every screen in light and dark to
   PNG files, and the pull request should include before and after images of the screens it changes.
3. Update this document in the same pull request.
