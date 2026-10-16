#!/usr/bin/env python3
"""Convert a captured ANSI screen (from the TUI tests) into a standalone HTML
page that looks like a terminal window, for docs screenshots.

    python3 hack/ansi2html.py screen.ans "vlx demo" > screen.html
"""
import html
import re
import sys

BG = "#0f1117"
FG = "#e9e6ee"
SGR = re.compile(r"\x1b\[([0-9;]*)m")
OTHER = re.compile(r"\x1b\][^\x07]*\x07|\x1b\[[0-9;?]*[A-Za-z]")


def convert(text):
    out = []
    state = {"fg": None, "bg": None, "bold": False, "italic": False, "faint": False, "ul": False}

    def span(chunk):
        if not chunk:
            return
        style = []
        if state["fg"]:
            style.append("color:" + state["fg"])
        if state["bg"]:
            style.append("background:" + state["bg"])
        if state["bold"]:
            style.append("font-weight:700")
        if state["italic"]:
            style.append("font-style:italic")
        if state["faint"]:
            style.append("opacity:.6")
        if state["ul"]:
            style.append("text-decoration:underline")
        esc = html.escape(chunk)
        out.append(f'<span style="{";".join(style)}">{esc}</span>' if style else esc)

    pos = 0
    for m in SGR.finditer(text):
        span(OTHER.sub("", text[pos:m.start()]))
        pos = m.end()
        codes = [int(c) if c else 0 for c in m.group(1).split(";")] if m.group(1) else [0]
        i = 0
        while i < len(codes):
            c = codes[i]
            if c == 0:
                state.update(fg=None, bg=None, bold=False, italic=False, faint=False, ul=False)
            elif c == 1:
                state["bold"] = True
            elif c == 2:
                state["faint"] = True
            elif c == 3:
                state["italic"] = True
            elif c == 4:
                state["ul"] = True
            elif c == 22:
                state["bold"] = state["faint"] = False
            elif c == 23:
                state["italic"] = False
            elif c == 24:
                state["ul"] = False
            elif c == 39:
                state["fg"] = None
            elif c == 49:
                state["bg"] = None
            elif c in (38, 48) and i + 4 < len(codes) + 0 and codes[i + 1] == 2:
                r, g, b = codes[i + 2:i + 5]
                state["fg" if c == 38 else "bg"] = f"rgb({r},{g},{b})"
                i += 4
            elif c in (38, 48) and codes[i + 1] == 5:
                i += 2
            i += 1
    span(OTHER.sub("", text[pos:]))
    return "".join(out)


def page(body, title):
    return f"""<!doctype html><html><head><meta charset="utf-8"><style>
html,body{{margin:0;background:transparent}}
.wrap{{padding:40px;display:inline-block;background:linear-gradient(135deg,#2a1d4a 0%,#12141c 55%,#0b2a33 100%)}}
.win{{background:{BG};border-radius:12px;box-shadow:0 30px 80px rgba(0,0,0,.55),0 0 0 1px rgba(255,255,255,.08);overflow:hidden}}
.bar{{height:38px;display:flex;align-items:center;padding:0 16px;gap:8px;background:#171a23;border-bottom:1px solid #262a36}}
.dot{{width:12px;height:12px;border-radius:50%}}
.title{{flex:1;text-align:center;color:#8a8496;font:500 13px Inter,sans-serif;margin-right:52px}}
pre{{margin:0;padding:14px 18px 18px;color:{FG};font:15px/1.17 'DejaVu Sans Mono','JetBrains Mono',monospace;white-space:pre}}
</style></head><body><div class="wrap"><div class="win">
<div class="bar"><span class="dot" style="background:#ff5f57"></span><span class="dot" style="background:#febc2e"></span><span class="dot" style="background:#28c840"></span><span class="title">{html.escape(title)}</span></div>
<pre>{body}</pre></div></div></body></html>"""


if __name__ == "__main__":
    with open(sys.argv[1], encoding="utf-8") as f:
        text = f.read()
    title = sys.argv[2] if len(sys.argv) > 2 else "vlx"
    sys.stdout.write(page(convert(text), title))
