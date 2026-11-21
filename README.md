# Snipping Tool Enhanced

A screen capture and screen recording tool for Windows 11, written in Go. Licensed under Apache-2.0.

One executable: `SnippingToolEnhanced.exe`.

## Implemented in this version

| Feature | Status |
| --- | --- |
| Square, rectangle, freeform, window and full-screen snips | Done. Freeform areas outside the shape are transparent in PNG and white on the clipboard |
| Capture bar | Done: snip / record toggle, snipping-area dropdown, color picker, close. Shift + ←/→ switches mode, Esc cancels |
| Delay 3 / 5 / 10 s | Done (delay dropdown, Alt + D) |
| Color picker | Done: magnifier, HEX / RGB / HSL (Tab switches format), scroll or Ctrl + / Ctrl − zooms, click copies |
| Shortcuts | Win + Alt + S (snip), Win + Alt + V (record), Print Screen (see below) |
| Clipboard + auto-save to *Pictures\Screenshots* | Done |
| Recordings | Saved only when you press Save, as in Windows 11. You're asked to save before closing. Turning on *Automatically save original screen recordings* saves every recording to *Videos\Screen Recordings* |
| "Mark up and share" notification, or open the editor directly | Done |
| Multiple windows setting | Done |
| Border, ask to save edited screenshots | Done |
| Editor: pen, highlighter (colors and size), eraser (whole strokes, *Erase all ink*) | Done |
| Draw & Hold (line, arrow, rectangle, oval snap; move and resize) | Done |
| Shapes with outline and fill, text box (font, size, color, highlight), emoji | Done (in the Shapes menu) |
| Ruler, protractor, touch writing | Done (in the "…" menu; scroll rotates, Shift + scroll by 15°) |
| Crop with handles (Shift keeps it square), undo / redo, zoom (Ctrl + scroll, Ctrl + 0 / 1) | Done |
| Save as PNG / JPEG / GIF, copy, Edit in Paint, print, open file | Done |
| Settings page, light / dark / system theme | Done |
| Screen recording | Done: rectangle or window area, 3-second countdown, Start, Pause/Resume, Stop, Delete, microphone and system audio toggles. Output is MP4 (H.264/AAC, about 30 fps), saved to *Videos\Screen Recordings* |
| Text Extractor | Done: capture bar or app window button, select an area, its text is copied. Settings: *Automatically copy text*, *Remove line breaks* |
| Text Actions | Done: select and copy text (Ctrl+A, Ctrl+C), *Copy all text*, *Copy as table* (tab-separated), *Quick redact* (email addresses, phone numbers), redact selected text (also via right-click), *Remove all redactions*. Uses the on-device Windows OCR engine |
| Playback window | Done: play, trim, Save as, GIF export (first 30 s, low or high quality, Export or Copy) |
| Group Policy *Do not allow Snipping Tool to run* (`DisableSnippingTool`) | Honored |

## Not built yet

These features are not in this version. Their buttons are shown grayed out:

- Share
- Quick markup
- Perfect screenshot
- Visual Search
- The `ms-screenclip:` protocol
- Edit in Clipchamp
- HDR settings

Text recognition needs an OCR language. Most Windows 11 installs have one for the display language; others can be added in *Settings > Time & language > Language & region*.

## Keyboard shortcuts

| Shortcut | Action |
| --- | --- |
| Win + Alt + S | Start a snip |
| Win + Alt + V | Start a screen recording |
| Print Screen | Start a snip (only if *Settings > Accessibility > Keyboard > Use the Print screen key to open screen capture* is turned off) |

Windows reserves **Win + Shift + S** and **Win + Shift + R** for its own Snipping Tool, and the Xbox Game Bar uses **Win + Alt + R**, so other apps can't use those.

The shortcuts keep working after you close the window, because a small background part of the app stays running to listen for them. **Settings > Shortcuts** shows whether each shortcut is available.

Editor shortcuts: Ctrl + N (new snip), Alt + N (new snip in last mode), Alt + M (mode), Alt + D (delay), Ctrl + C, Ctrl + S (Save as), Ctrl + O, Ctrl + P, Ctrl + Z / Ctrl + Y, Delete (remove the selected shape), Esc.

## Problems

If the app closes unexpectedly, the details are written to `%LOCALAPPDATA%\SnippingToolEnhanced\crash.log`. Please include that file when reporting a problem.

## Build

Requires Go 1.22 or later. Building on Windows:

```powershell
.\build.ps1 -Run
```

The output goes to `dist\amd64\SnippingToolEnhanced.exe` and `dist\arm64\SnippingToolEnhanced.exe`. To build x64 only, run `.\build.ps1 -Arch amd64` (or `make build ARCH=amd64`).

The first build for each architecture compiles Go's standard library and all dependencies, which can take several minutes, longer if antivirus scans Go's build cache. The build prints each package as it compiles. Later builds reuse the cache and take seconds.

From Linux, macOS or WSL, run `make build`. The first build creates `go.sum`; commit it. The build also regenerates the app icon, version info and manifest (`cmd/snippingtoolenhanced/rsrc_windows_*.syso`, `assets/SnippingToolEnhanced.ico`) with `go run -tags genres tools/genres/main.go <version>`.

The editor uses the Microsoft Edge WebView2 Runtime, which is included with Windows 11.

## Test

- `go test ./...` tests the capture, imaging, settings and rendering logic.
- `python test/editor_test.py` drives the editor in headless Chromium. It needs `pip install playwright` and `python -m playwright install chromium`.

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
