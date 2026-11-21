# SPDX-License-Identifier: Apache-2.0
"""Headless test of the editor window UI (internal/editor/ui/index.html).

Fakes the Go bindings the way go-webview2 exposes them, then checks the
empty state, the markup tools, Draw & Hold, crop, undo, save and settings.

    pip install playwright && python -m playwright install chromium
    python test/editor_test.py [--shots DIR]
"""
import base64, functools, http.server, json, pathlib, struct, sys, threading, zlib

from playwright.sync_api import sync_playwright

ROOT = pathlib.Path(__file__).resolve().parents[1]
PAGE = (ROOT / "internal" / "editor" / "ui" / "index.html").as_uri()


def png(w, h):
    rows = b""
    for y in range(h):
        rows += b"\x00" + b"".join(bytes([250, 250, 252]) if not (60 < y < 90 and 40 < x < w - 40) else bytes([200, 210, 225]) for x in range(w))
    chunk = lambda t, d: struct.pack(">I", len(d)) + t + d + struct.pack(">I", zlib.crc32(t + d) & 0xFFFFFFFF)
    return b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0)) + chunk(b"IDAT", zlib.compress(rows)) + chunk(b"IEND", b"")


IMAGE = {"dataUrl": "data:image/png;base64," + base64.b64encode(png(560, 330)).decode(), "name": "Screenshot", "path": "C:\\x.png", "width": 560, "height": 330}
SETTINGS = {"autoCopyChanges": True, "autoSaveScreenshots": True, "askToSaveEdited": True, "multipleWindows": False, "borderEnabled": False,
            "borderColor": "#000000", "borderThickness": 1, "openEditorAfterSnip": False, "theme": "light", "mode": "rectangle", "delay": 0, "saveFolder": ""}

STUB = """
window.__calls = [];
const rec = (n, ret) => (...a) => { window.__calls.push(n); return Promise.resolve(typeof ret === 'function' ? ret(...a) : ret); };
window.stLoad = rec('stLoad', () => window.__load);
window.stSaveSettings = rec('stSaveSettings', (j) => JSON.parse(j));
window.stCopy = rec('stCopy', null); window.stEditInPaint = rec('stEditInPaint', null);
window.stSetDirty = rec('stSetDirty', null); window.stHasImage = rec('stHasImage', null);
window.stOpenWindowsSettings = rec('stOpenWindowsSettings', null); window.stShowInFolder = rec('stShowInFolder', null); window.stClose = rec('stClose', null);
const asyncRec = (n, res) => (id, ...a) => { window.__calls.push(n); setTimeout(() => window.stDone(id, typeof res === 'function' ? res(...a) : res, ''), 20); return Promise.resolve(); };
window.stNewSnip = asyncRec('stNewSnip', () => ({ image: window.__image }));
window.stSaveAs = asyncRec('stSaveAs', 'C:\\\\Pictures\\\\Screenshots\\\\a.png');
window.stOpenFile = asyncRec('stOpenFile', null); window.stPickFolder = asyncRec('stPickFolder', 'D:\\\\Shots');
window.stSaveVideo = asyncRec('stSaveVideo', (...a) => { window.__saveVideo = a; return 'C:\\\\Videos\\\\v.mp4'; });
window.stShareImage = asyncRec('stShareImage', (u) => { window.__shareImage = u; return null; });
window.stVisualSearch = asyncRec('stVisualSearch', (u) => { window.__search = u; return null; });
window.stShareVideo = asyncRec('stShareVideo', (...a) => { window.__shareVideo = a; return null; });
window.__gifFrames = 0;
window.stCopyText = rec('stCopyText', (t) => { window.__copied = t; return null; });
window.stOcr = asyncRec('stOcr', () => ({ angle: 0, lines: [
  { text: "Name Phone", words: [{ text: "Name", x: 40, y: 100, w: 50, h: 14 }, { text: "Phone", x: 240, y: 100, w: 55, h: 14 }] },
  { text: "Anna +46 70 123 45 67", words: [{ text: "Anna", x: 40, y: 130, w: 45, h: 14 }, { text: "+46", x: 240, y: 130, w: 30, h: 14 }, { text: "70", x: 275, y: 130, w: 18, h: 14 }, { text: "123", x: 297, y: 130, w: 28, h: 14 }, { text: "45", x: 329, y: 130, w: 18, h: 14 }, { text: "67", x: 351, y: 130, w: 18, h: 14 }] },
  { text: "Mail anna@example.com", words: [{ text: "Mail", x: 40, y: 160, w: 40, h: 14 }, { text: "anna@example.com", x: 240, y: 160, w: 130, h: 14 }] } ] }));
window.stGifBegin = rec('stGifBegin', 'g1'); window.stGifFrame = rec('stGifFrame', () => { window.__gifFrames++; return null; });
window.stGifFinish = asyncRec('stGifFinish', (id, action) => action === 'copy' ? '' : 'C:\\\\Videos\\\\v.gif');
"""


class CORS(http.server.SimpleHTTPRequestHandler):
    def end_headers(self):
        self.send_header("Access-Control-Allow-Origin", "*")
        super().end_headers()

    def log_message(self, *a):
        pass


def video_tests(b, shots, errors, page):
    srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0), functools.partial(CORS, directory=str(ROOT / "test")))
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    url = "http://127.0.0.1:%d/sample.mp4" % srv.server_address[1]
    s = dict(SETTINGS); s["kind"] = "record"
    pg = page(1172, 620, {"settings": s, "version": "1.0.0",
                          "video": {"url": url, "name": "Screen Recording", "path": "C:\\r.mp4", "duration": 0, "unsaved": True}})
    pg.wait_for_function("S.video && S.video.duration > 5", timeout=15000)
    # An unsaved recording asks before closing.
    pg.evaluate("void stRequestClose()")
    assert "not been saved" in pg.inner_text("#dlgText")
    pg.click("[data-r=cancel]")
    assert pg.is_visible("#btnGif") and pg.is_visible("#btnTrim") and not pg.is_visible("#tools") and pg.evaluate("$('segRec').classList.contains('on')")
    # mode menu offers only recording modes
    pg.click("#btnMode")
    assert pg.locator("[data-mode]").count() == 2
    pg.keyboard.press("Escape")
    # trim: drag the start handle to ~1/3
    pg.click("#btnTrim")
    tb = pg.locator("#trimTrack").bounding_box(); ha = pg.locator("#trimA").bounding_box()
    pg.mouse.move(ha["x"] + 5, ha["y"] + 10); pg.mouse.down(); pg.mouse.move(tb["x"] + tb["width"] / 3, ha["y"] + 10, steps=5); pg.mouse.up()
    st = pg.evaluate("S.video.start")
    assert 1.5 < st < 2.5, st
    if shots: pg.screenshot(path=str(shots / "video.png"))
    pg.keyboard.press("Control+s")
    pg.wait_for_function("window.__saveVideo")
    pg.wait_for_function("!S.video.unsaved")
    a = pg.evaluate("window.__saveVideo")
    assert a[0] == "C:\\r.mp4" and 1.5 < a[1] < 2.5 and a[2] > 5.5, a
    # Share sends the recording and the trim range.
    assert pg.is_enabled("#btnShareVideo")
    pg.click("#btnShareVideo")
    pg.wait_for_function("window.__shareVideo")
    a = pg.evaluate("window.__shareVideo")
    assert a[0] == "C:\\r.mp4" and 1.5 < a[1] < 2.5, a
    # GIF export (low quality, copy)
    pg.click("#btnGif"); pg.click("[data-q=low]")
    if shots: pg.screenshot(path=str(shots / "gif-flyout.png"))
    pg.click("#gifCopy")
    pg.wait_for_function("window.__calls.includes('stGifFinish')", timeout=60000)
    n = pg.evaluate("window.__gifFrames")
    assert 35 <= n <= 45, n  # ~4 s × 10 fps
    # switching back to snip
    pg.click("#segSnip")
    assert pg.evaluate("S.settings.kind") == "snip"
    pg.close()
    srv.shutdown()


def main():
    shots = pathlib.Path(sys.argv[sys.argv.index("--shots") + 1]) if "--shots" in sys.argv else None
    errors = []
    with sync_playwright() as p:
        b = p.chromium.launch(channel="chrome") if pathlib.Path("/usr/bin/google-chrome").exists() else p.chromium.launch()

        def page(w, h, load, dark=False):
            pg = b.new_page(viewport={"width": w, "height": h}, color_scheme="dark" if dark else "light")
            pg.set_default_timeout(8000)
            pg.on("pageerror", lambda e: errors.append(str(e)))
            pg.on("console", lambda m: errors.append(m.text) if m.type == "error" else None)
            pg.add_init_script(STUB + "window.__load=%s; window.__image=%s;" % (json.dumps(load), json.dumps(IMAGE)))
            pg.goto(PAGE)
            pg.wait_for_function("window.__calls.includes('stLoad')")
            return pg

        # 1. Empty state, Print Screen available.
        pg = page(496, 197, {"settings": SETTINGS, "version": "1.0.0"})
        assert "Alt + S" in pg.inner_text("#emptyLine")
        assert pg.is_visible("#btnNew") and pg.is_visible("#btnTextExtractor") and not pg.is_visible("#tools")
        if shots: pg.screenshot(path=str(shots / "empty.png"))
        # Mode and delay dropdowns.
        pg.click("#btnMode")
        assert pg.locator("[data-mode]").nth(1).get_attribute("data-mode") == "square"
        pg.click("[data-mode=window]")
        assert pg.evaluate("S.settings.mode") == "window"
        pg.click("#btnDelay"); pg.click("[data-d='5']")
        assert pg.inner_text("#delayText") == "5s"

        # Text Extractor button (empty window): copied text shows a toast; not copied shows a dialog.
        pg.evaluate("window.stNewSnip = (id) => { window.__calls.push('stNewSnip'); setTimeout(() => window.stDone(id, { text: { text: 'Hello OCR', copied: true } }, ''), 10); return Promise.resolve(); }")
        pg.click("#btnTextExtractor")
        pg.wait_for_function("document.getElementById('toast').textContent.includes('Text copied')")
        pg.evaluate("window.stNewSnip = (id) => { setTimeout(() => window.stDone(id, { text: { text: 'Line 1\\nLine 2', copied: false } }, ''), 10); return Promise.resolve(); }")
        pg.click("#btnTextExtractor")
        pg.wait_for_function("document.getElementById('scrim').classList.contains('show')")
        assert pg.input_value("#dlgTA") == "Line 1\nLine 2"
        pg.click("[data-r=copy]")
        pg.wait_for_function("window.__copied === 'Line 1\\nLine 2'")
        pg.close()

        # 2. Empty state when Win+Alt+S is taken by another app; record kind shows Win+Alt+V.
        pg = page(496, 197, {"settings": SETTINGS, "shortcuts": {"snip": False, "record": True}, "version": "1.0.0"})
        assert "New" in pg.inner_text("#emptyLine") and "another app" in pg.inner_text("#emptyHint")
        pg.click("#segRec")
        assert "Alt + V" in pg.inner_text("#emptyLine") and "recording" in pg.inner_text("#emptyLine")
        pg.close()

        # 3. Markup.
        pg = page(1172, 520, {"settings": SETTINGS, "version": "1.0.0"})
        pg.click("#btnNew")
        pg.wait_for_function("S.img && S.w === 560")
        assert pg.is_visible("#tools") and pg.is_visible("#btnPaint") and not pg.is_visible("#btnTextExtractor")
        box = pg.locator("#view").bounding_box()
        at = lambda fx, fy: (box["x"] + box["width"] * fx, box["y"] + box["height"] * fy)
        def drag(a, z, steps=10, hold=0):
            pg.mouse.move(*a); pg.mouse.down(); pg.mouse.move(*z, steps=steps)
            if hold: pg.wait_for_timeout(hold)
            pg.mouse.up()

        drag(at(.1, .2), at(.4, .3))  # free pen stroke
        assert pg.evaluate("S.objects.length") == 1 and pg.evaluate("S.objects[0].type") == "stroke"
        # Draw & Hold: a straight stroke held still snaps to a line shape.
        drag(at(.1, .5), at(.45, .52), steps=12, hold=900)
        assert pg.evaluate("S.objects[1].type") == "shape" and pg.evaluate("S.objects[1].kind") == "line", pg.evaluate("S.objects[1]")
        # Draw & Hold rectangle.
        pg.mouse.move(*at(.55, .55)); pg.mouse.down()
        for pt in [(.85, .55), (.85, .9), (.55, .9), (.55, .56)]:
            pg.mouse.move(*at(*pt), steps=8)
        pg.wait_for_timeout(900); pg.mouse.up()
        assert pg.evaluate("S.objects[2].kind") == "rect", pg.evaluate("S.objects[2]")
        # Move the selected shape.
        before = pg.evaluate("S.objects[2].x1")
        drag(at(.55, .7), at(.6, .7), steps=4)
        assert pg.evaluate("S.objects[2].x1") > before

        # Highlighter selected shows its flyout on second click.
        pg.click("[data-tool=highlighter]")
        assert pg.evaluate("S.tool") == "highlighter"
        if shots: pg.screenshot(path=str(shots / "markup.png"), clip={"x": 0, "y": 0, "width": 1172, "height": 387})
        pg.click("[data-tool=highlighter]")
        assert pg.is_visible("#flyout")
        pg.click("[data-c='#26e600']")
        assert pg.evaluate("S.hl.color") == "#26e600"
        if shots: pg.screenshot(path=str(shots / "highlighter-flyout.png"))
        pg.keyboard.press("Escape")

        # Eraser removes the first stroke; undo brings it back.
        pg.click("[data-tool=eraser]")
        n = pg.evaluate("S.objects.length")
        drag(at(.25, .25), at(.25, .26), steps=2)
        assert pg.evaluate("S.objects.length") == n - 1
        pg.keyboard.press("Control+z")
        assert pg.evaluate("S.objects.length") == n

        # Shapes: arrow, then text and emoji.
        pg.click("[data-tool=shapes]")
        pg.click("[data-k=arrow]")
        pg.keyboard.press("Escape")
        drag(at(.2, .8), at(.4, .65))
        assert pg.evaluate("S.objects[S.objects.length-1].kind") == "arrow"
        pg.click("[data-tool=shapes]"); pg.click("#fEmojiBtn"); pg.click("[data-e='👍']")
        assert pg.evaluate("S.objects[S.objects.length-1].type") == "emoji"
        pg.click("[data-tool=shapes]"); pg.click("[data-k=text]"); pg.keyboard.press("Escape")
        pg.mouse.click(*at(.6, .15)); pg.keyboard.type("Hello")
        pg.mouse.click(*at(.02, .98))
        assert pg.evaluate("S.objects.some(o => o.type === 'text' && o.text === 'Hello')"), pg.evaluate("JSON.stringify([S.objects.map(o=>[o.type,o.kind,o.text]), S.sel, S.editingText, S.tool, S.shape.kind, document.activeElement.id])")

        # Ruler snaps a pen stroke to its edge.
        pg.click("#btnMore"); pg.click("#mRuler")
        assert pg.evaluate("!!S.ruler")
        pg.click("[data-tool=pen]")
        e = pg.evaluate("(() => { const e = rulerEdge(S.ruler); return [e.a, e.b]; })()")
        r = pg.locator("#view").bounding_box(); cw = pg.evaluate("currentCrop()")
        to_css = lambda q: (r["x"] + (q["x"] - cw["x"]) * r["width"] / cw["w"], r["y"] + (q["y"] - cw["y"]) * r["height"] / cw["h"] - 6)
        a, z = e
        mid1 = {"x": a["x"] + (z["x"] - a["x"]) * .3, "y": a["y"]}; mid2 = {"x": a["x"] + (z["x"] - a["x"]) * .6, "y": a["y"]}
        drag(to_css(mid1), (to_css(mid2)[0], to_css(mid2)[1] + 8), steps=6)
        last = pg.evaluate("S.objects[S.objects.length-1]")
        assert last["type"] == "stroke" and len(last["points"]) == 2, last
        pg.click("#btnMore"); pg.click("#mRuler")

        # Crop with handles, then apply.
        pg.click("[data-tool=crop]")
        assert pg.is_visible("#cropBar")
        box = pg.locator("#view").bounding_box()
        if shots: pg.screenshot(path=str(shots / "crop-start.png"), clip={"x": 0, "y": 0, "width": 1172, "height": 520})
        # 1) draw a new area by dragging inside the full-image frame (away from the handles)
        pg.mouse.move(box["x"] + box["width"] * .3, box["y"] + box["height"] * .3); pg.mouse.down()
        pg.mouse.move(box["x"] + box["width"] * .7, box["y"] + box["height"] * .8, steps=6); pg.mouse.up()
        c = pg.evaluate("S.cropping")
        assert 150 <= c["x"] <= 190 and 200 <= c["w"] <= 250, c
        # 2) move it by dragging inside
        pg.mouse.move(box["x"] + box["width"] * .5, box["y"] + box["height"] * .55); pg.mouse.down()
        pg.mouse.move(box["x"] + box["width"] * .45, box["y"] + box["height"] * .55, steps=4); pg.mouse.up()
        c2 = pg.evaluate("S.cropping")
        assert c2["x"] < c["x"] and abs(c2["w"] - c["w"]) < 1, (c, c2)
        # 3) resize with the top-left handle
        hx = box["x"] + (c2["x"] + 6) * box["width"] / 560; hy = box["y"] + (c2["y"] + 6) * box["height"] / 330
        pg.mouse.move(hx, hy); pg.mouse.down(); pg.mouse.move(hx - 20, hy - 20, steps=4); pg.mouse.up()
        c3 = pg.evaluate("S.cropping")
        assert c3["x"] < c2["x"] and c3["w"] > c2["w"], (c2, c3)
        if shots: pg.screenshot(path=str(shots / "crop-area.png"), clip={"x": 0, "y": 0, "width": 1172, "height": 520})
        # 4) Shift + corner handle keeps it square; Shift + drawing a new area too
        sc = lambda c, fx, fy: (box["x"] + (c["x"] + c["w"] * fx) * box["width"] / 560, box["y"] + (c["y"] + c["h"] * fy) * box["height"] / 330)
        hx, hy = sc(c3, 1, 1); hx -= 6 * box["width"] / 560; hy -= 6 * box["height"] / 330
        pg.keyboard.down("Shift"); pg.mouse.move(hx, hy); pg.mouse.down(); pg.mouse.move(hx + 40, hy + 5, steps=4); pg.mouse.up(); pg.keyboard.up("Shift")
        c4 = pg.evaluate("S.cropping")
        assert abs(c4["w"] - c4["h"]) < 0.5 and abs(c4["x"] - c3["x"]) < 0.5 and abs(c4["y"] - c3["y"]) < 0.5, c4
        pg.keyboard.press("Escape"); pg.click("[data-tool=crop]")
        pg.keyboard.down("Shift"); pg.mouse.move(*sc({"x": 0, "y": 0, "w": 560, "h": 330}, .3, .3)); pg.mouse.down()
        pg.mouse.move(*sc({"x": 0, "y": 0, "w": 560, "h": 330}, .9, .5), steps=5); pg.mouse.up(); pg.keyboard.up("Shift")
        c5 = pg.evaluate("S.cropping")
        assert abs(c5["w"] - c5["h"]) < 0.5 and c5["y"] + c5["h"] <= 330.01, c5
        # Shift + side handle: square, centered on the old area
        nx, ny = sc(c5, 1, .5); nx -= 6 * box["width"] / 560
        pg.keyboard.down("Shift"); pg.mouse.move(nx, ny); pg.mouse.down(); pg.mouse.move(nx - 30, ny, steps=4); pg.mouse.up(); pg.keyboard.up("Shift")
        c6 = pg.evaluate("S.cropping")
        assert abs(c6["w"] - c6["h"]) < 0.5 and c6["w"] < c5["w"], (c5, c6)
        c3 = c6
        pg.click("#cropApply")
        c = pg.evaluate("S.crop")
        assert c and abs(c["x"] - round(c3["x"])) <= 1 and c["w"] < 560, c
        raw = base64.b64decode(pg.evaluate("exportPNG()").split(",", 1)[1])
        assert struct.unpack(">II", raw[16:24]) == (c["w"], c["h"])

        # Save, copy, Paint.
        pg.keyboard.press("Control+s")
        pg.wait_for_function("document.getElementById('toast').textContent.includes('a.png')")
        assert pg.evaluate("S.hIndex === S.savedIndex")
        pg.click("#btnCopy"); pg.click("#btnPaint")
        # Share sends the marked-up image.
        assert pg.is_enabled("#btnShare")
        pg.click("#btnShare")
        pg.wait_for_function("window.__shareImage")
        assert pg.evaluate("window.__shareImage.startsWith('data:image/png')")
        # Visual Search sends the marked-up image; the tooltip names the engine.
        assert pg.is_visible("#btnSearch")
        pg.hover("#btnSearch")
        assert pg.get_attribute("#btnSearch", "title") == "Visual Search with Bing"
        pg.evaluate("S.settings.visualSearch = 'google'")
        pg.mouse.move(0, 0); pg.hover("#btnSearch")
        assert pg.get_attribute("#btnSearch", "title") == "Visual Search with Google Lens"
        pg.evaluate("S.settings.visualSearch = 'bing'")
        pg.click("#btnSearch")
        pg.wait_for_function("window.__search")
        assert pg.evaluate("window.__search.startsWith('data:image/png')")
        # Quick markup's Copy from the New button only shows a notice.
        pg.evaluate("window.__keepImg = S.img; window.stNewSnip = (id) => { setTimeout(() => window.stDone(id, { notice: 'Snip copied to clipboard' }, ''), 10); return Promise.resolve(); }")
        pg.click("#btnNew")
        pg.wait_for_function("document.getElementById('toast').textContent.includes('Snip copied')")
        assert pg.evaluate("S.img === window.__keepImg")
        for need in ["stSaveAs", "stCopy", "stEditInPaint", "stSetDirty", "stNewSnip"]:
            assert need in pg.evaluate("window.__calls"), need

        # Text actions (OCR): copy all, table, select + copy, quick redact, redact selection.
        pg.keyboard.press("Control+z"); pg.wait_for_timeout(50)
        while pg.evaluate("S.crop !== null"):
            pg.keyboard.press("Control+z")
        pg.click("#btnTextActions")
        pg.wait_for_function("S.ocr && S.ocr.words.length === 10")
        assert pg.is_visible("#taBar")
        if shots: pg.screenshot(path=str(shots / "text-actions.png"), clip={"x": 0, "y": 0, "width": 1172, "height": 520})
        pg.click("#taCopyAll")
        assert pg.evaluate("window.__copied") == "Name Phone\r\nAnna +46 70 123 45 67\r\nMail anna@example.com"
        pg.click("#taTable")
        assert pg.evaluate("window.__copied") == "Name\tPhone\r\nAnna\t+46 70 123 45 67\r\nMail\tanna@example.com", pg.evaluate("window.__copied")
        box = pg.locator("#view").bounding_box()
        tc = lambda x, y: (box["x"] + x * box["width"] / 560, box["y"] + y * box["height"] / 330)
        pg.mouse.move(*tc(60, 137)); pg.mouse.down(); pg.mouse.move(*tc(300, 137), steps=5); pg.mouse.up()
        pg.keyboard.press("Control+c")
        assert pg.evaluate("window.__copied") == "Anna +46 70 123", pg.evaluate("window.__copied")
        n0 = pg.evaluate("S.objects.filter(o => o.type === 'redact').length")
        pg.click("#taRedact")
        assert pg.evaluate("S.objects.filter(o => o.type === 'redact').length") == n0 + 2
        r = pg.evaluate("S.objects.filter(o => o.type === 'redact').map(o => [Math.round(o.x), Math.round(o.w)])")
        assert [238, 133] in r and [238, 134] in r, r  # phone number (5 words) and email
        pg.mouse.move(*tc(60, 107)); pg.mouse.down(); pg.mouse.up()
        pg.mouse.click(*tc(60, 107), button="right")
        pg.click("#cmRedact")
        assert pg.evaluate("S.objects.filter(o => o.type === 'redact').length") == n0 + 3
        # redactions are in the exported image (black pixels where "Name" was)
        px = pg.evaluate("(() => { const c = document.createElement('canvas'); c.width = 560; c.height = 330; const g = c.getContext('2d'); renderInto(g, currentCrop()); return Array.from(g.getImageData(60, 106, 1, 1).data); })()")
        assert px[:3] == [0, 0, 0], px
        if shots: pg.screenshot(path=str(shots / "redacted.png"), clip={"x": 0, "y": 0, "width": 1172, "height": 520})
        pg.click("#taRedactMenu"); pg.click("#rqRemove")
        assert pg.evaluate("S.objects.filter(o => o.type === 'redact').length") == 0
        pg.keyboard.press("Escape")
        assert not pg.is_visible("#taBar")

        # Ask to save on close.
        drag(at(.3, .3), at(.5, .4))
        pg.evaluate("void stRequestClose()")
        assert pg.is_visible("#scrim")
        pg.click("[data-r=discard]")
        pg.wait_for_function("window.__calls.includes('stClose')")

        # Settings page.
        pg.click("#btnMore"); pg.click("#mSettings")
        assert pg.is_visible("#settings")
        pg.click("input[data-key=multipleWindows]", force=True)
        assert pg.evaluate("S.settings.multipleWindows") is True
        # Visual Search engine: Bing by default, Google Lens can be picked.
        assert pg.input_value("#setSearch") == "bing"
        pg.select_option("#setSearch", "google")
        assert pg.evaluate("S.settings.visualSearch") == "google"
        if shots: pg.screenshot(path=str(shots / "settings.png"))
        pg.click("#setBack")
        pg.close()

        # 4. Recording playback, trim, save and GIF export (needs Chrome for H.264).
        video_tests(b if not shots else b, shots, errors, page)

        # 5. Dark theme markup screenshot.
        if shots:
            d = dict(SETTINGS); d["theme"] = "dark"
            pg = page(1172, 520, {"settings": d, "version": "1.0.0", "image": IMAGE}, dark=True)
            pg.wait_for_function("S.img")
            pg.screenshot(path=str(shots / "markup-dark.png"), clip={"x": 0, "y": 0, "width": 1172, "height": 387})
            pg.close()
        b.close()
    if errors:
        raise SystemExit("page errors: " + "; ".join(errors))
    print("editor test: OK")


if __name__ == "__main__":
    main()
