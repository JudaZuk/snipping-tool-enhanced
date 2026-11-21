# SPDX-License-Identifier: Apache-2.0
"""Checks the Visual Search upload pages in Chromium: each page must post the
snip to its engine in the right form field. Requests are intercepted, nothing
is sent. Usage: python test/visualsearch_test.py <dir with bing.html, google.html, snip.jpg>"""
import base64, pathlib, sys
from playwright.sync_api import sync_playwright

d = pathlib.Path(sys.argv[1])
jpg = (d / "snip.jpg").read_bytes()
expect = {
    "bing.html": ("www.bing.com/images/search", b'name="imageBin"', base64.b64encode(jpg)),
    "google.html": ("lens.google.com/v3/upload", b'name="encoded_image"; filename="snip.jpg"', jpg),
}
with sync_playwright() as p:
    b = p.chromium.launch()
    for page_name, (host, field, payload) in expect.items():
        pg = b.new_page()
        got = {}
        def handle(route, request, got=got):
            got["url"], got["method"], got["body"] = request.url, request.method, request.post_data_buffer
            got["ctype"] = request.headers.get("content-type", "")
            route.fulfill(status=200, body="ok")
        pg.route("https://**/*", handle)
        pg.goto((d / page_name).as_uri())
        pg.wait_for_function("document.body.innerText.includes('ok')", timeout=10000)
        assert host in got["url"], got.get("url")
        assert got["method"] == "POST" and got["ctype"].startswith("multipart/form-data"), got
        assert field in got["body"], page_name + ": field missing"
        assert payload in got["body"], page_name + ": image missing"
        if page_name == "google.html":
            assert b"Content-Type: image/jpeg" in got["body"]
        pg.close()
    b.close()
print("visual search pages: OK")
