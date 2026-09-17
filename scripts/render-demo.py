#!/usr/bin/env python3
"""Assemble a recorded run (jev-turbo explore --record DIR) into an MP4 and a GIF.

    python3 scripts/render-demo.py DIR out/demo [--title "Zürich → London"] [--subtitle "..."]
    python3 scripts/render-demo.py DIR out/still.png --still 3200      # one composed frame at 3.2 s

The page sits on the left in a browser frame. The run sits on the right: the
elapsed clock, one row per action with the sightmap component it used, the
finish check, and the median Jev pick. Frames play at the wall-clock intervals
they were captured at, so the video runs at 1x. Needs ffmpeg and Pillow.
"""
import argparse, json, os, re, shutil, subprocess, sys, tempfile

from PIL import Image, ImageDraw, ImageFont

ap = argparse.ArgumentParser()
ap.add_argument("src")
ap.add_argument("out")
ap.add_argument("--title", default="")
ap.add_argument("--subtitle", default="One goal. Every pick by Jev over a sightmap.")
ap.add_argument("--still", type=int, default=None, help="render one frame at this ms offset to OUT and stop")
ap.add_argument("--width", type=int, default=1200)
args = ap.parse_args()
src = args.src

frames = [json.loads(l) for l in open(os.path.join(src, "frames.jsonl")) if l.strip()]
events = [json.loads(l) for l in open(os.path.join(src, "events.jsonl")) if l.strip()]
if not frames:
    sys.exit("no frames")
run_path = os.path.join(src, "run.json")
run = json.load(open(run_path)) if os.path.exists(run_path) else {}
if "runs" in run:
    run = run["runs"][0]
spec = run.get("spec") or {}
values = spec.get("values") or {}
total_ms = run.get("ms") or frames[-1]["t"]
tail_ms = 700

# sightmap.org palette
BG, TEXT, TEXT2, DIM, BORDER, SUBTLE, RAISED = "#faf8f6", "#1a1714", "#3d3929", "#8a8272", "#e5e0da", "#f0ece8", "#ffffff"
ACCENT, ACCENT_DIM, DARK = "#c9456d", "#f8e6ec", "#1a1a2e"


def font(kind, size):
    paths = {
        "sans": ["~/Library/Fonts/DMSans-Regular.ttf", "/System/Library/Fonts/Supplemental/Arial.ttf", "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"],
        "bold": ["~/Library/Fonts/DMSans-Bold.ttf", "/System/Library/Fonts/Supplemental/Arial Bold.ttf", "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf"],
        "mono": ["~/Library/Fonts/JetBrainsMono-Regular.ttf", "/System/Library/Fonts/Menlo.ttc", "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf"],
        "monobold": ["~/Library/Fonts/JetBrainsMono-Bold.ttf", "/System/Library/Fonts/Menlo.ttc", "/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf"],
    }[kind]
    for p in paths:
        p = os.path.expanduser(p)
        if os.path.exists(p):
            idx = 1 if kind == "monobold" and p.endswith("Menlo.ttc") else 0
            try:
                return ImageFont.truetype(p, size, index=idx)
            except OSError:
                return ImageFont.truetype(p, size)
    return ImageFont.load_default()


F = {k: font(*v) for k, v in {
    "brand": ("bold", 22), "brand2": ("sans", 17), "pill": ("mono", 11), "h1": ("bold", 40), "sub": ("sans", 17),
    "url": ("mono", 12), "label": ("mono", 12), "clock": ("monobold", 54), "clockcap": ("mono", 11),
    "row": ("bold", 16), "rowsub": ("mono", 11), "card": ("bold", 15), "cardline": ("mono", 12),
    "stat": ("monobold", 22), "statcap": ("sans", 12), "foot": ("sans", 12), "footmono": ("mono", 11),
}.items()}

ACTION = re.compile(r'^(filled|clicked|selected|typed|chose|opened) \[(\w+)((?: [^\]]*)?)\](?: with (\w+))?')
PROP = re.compile(r'(\w+)="([^"]*)"')


def human(text):
    """One line for a person, and one line naming the sightmap component."""
    m = ACTION.match(text)
    if not m:
        if text.startswith("waited"):
            return "Results load", "wait"
        if text.startswith("went back"):
            return "Back", "back"
        if text.startswith("scrolled"):
            return "Scroll", "scroll"
        if text.startswith("pressed"):
            return "Enter", "enter"
        return text[:28], ""
    verb, comp, props, key = m.groups()
    props = dict(PROP.findall(props or ""))
    label = None
    if verb == "filled" and key and key in values:
        label = str(values[key])
    elif "label" in props:
        label = props["label"]
    elif "value" in props:
        first = props["value"].split(". ")[0]
        label = re.sub(r"^(Change|Select|Choose|Open|Set) ", "", first).capitalize() if ". " in props["value"] else props["value"]
    else:
        words = re.sub(r"(?<!^)(?=[A-Z])", " ", comp).split()
        if len(words) > 1 and words[-1] in ("Button", "Link", "Field", "Select", "Input", "Tab"):
            words = words[:-1]
        label = " ".join(words)
    if len(label) > 26:
        label = label[:25] + "…"
    return label, comp


rows = []
for ev in events:
    if ev["text"].startswith("stale") or ev["text"] == "done":
        continue
    label, comp = human(ev["text"])
    rows.append({"t": ev["t"], "label": label, "comp": comp, "view": ev.get("view", ""), "step": ev["step"]})
done_at = next((ev["t"] for ev in events if ev["text"] == "done"), None)
if run.get("ok") and done_at is None:
    done_at = total_ms

# finish check lines
def check_lines(dw):
    if not dw:
        return ["Jev's own done answer ≥ 0.85"]
    if "all" in dw:
        out = []
        for c in dw["all"]:
            out += check_lines(c)
        return out
    for k, v in dw.items():
        k = k.replace("_contains", "").replace("_", " ")
        return [f"{k} has {v}" if k in ("url", "text") else f"{k} = {v}"]
    return []


checks = check_lines(spec.get("done_when") or run.get("done_when"))[:3]
picks = [s.get("ms_pick") for s in run.get("steps", []) if s.get("ms_pick")]
median_pick = sorted(picks)[len(picks) // 2] if picks else None
llm_calls = (run.get("picker_stats") or {}).get("llm_calls", 0)

W = args.width
H = round(W * 800 / 1200)
sx = W / 1200  # scale for layout numbers written at 1200 wide


def S(v):
    return round(v * sx)


def rounded(draw, box, r, fill=None, outline=None, width=1):
    draw.rounded_rectangle(box, radius=r, fill=fill, outline=outline, width=width)


def text_w(draw, s, f):
    l, t, r, b = draw.textbbox((0, 0), s, font=f)
    return r - l


def compose(page, t_ms):
    img = Image.new("RGB", (W, H), BG)
    d = ImageDraw.Draw(img)

    # header
    x = S(28)
    d.text((x, S(26)), "jev-turbo", font=F["brand"], fill=TEXT)
    x += text_w(d, "jev-turbo", F["brand"]) + S(12)
    d.text((x, S(30)), "sightmap × TypeSafe Jev", font=F["brand2"], fill=DIM)
    pill = "LIVE SITE · 1× SPEED · 0 LLM CALLS" if not llm_calls else "LIVE SITE · 1× SPEED"
    pw = text_w(d, pill, F["pill"]) + S(28)
    rounded(d, (W - S(28) - pw, S(24), W - S(28), S(24) + S(26)), S(13), fill=ACCENT_DIM)
    d.text((W - S(28) - pw + S(14), S(31)), pill, font=F["pill"], fill=ACCENT)
    title = args.title or (f"{run.get('name', 'run').replace('-', ' → ').title()}. {total_ms / 1000:.1f} seconds.")
    d.text((S(28), S(58)), title, font=F["h1"], fill=TEXT)
    d.text((S(28), S(108)), args.subtitle, font=F["sub"], fill=DIM)

    # browser frame
    fx, fy, fw = S(28), S(146), S(820)
    bar = S(30)
    ph = round(page.height * fw / page.width)
    rounded(d, (fx, fy, fx + fw, fy + bar + ph), S(10), fill=DARK, outline=BORDER)
    for i, c in enumerate(("#ff5f57", "#febc2e", "#28c840")):
        cx = fx + S(16) + i * S(18)
        d.ellipse((cx, fy + S(10), cx + S(10), fy + S(20)), fill=c)
    url = (run.get("steps") or [{}])[0].get("url", "") or ""
    url = re.sub(r"^https?://(www\.)?", "", url).split("?")[0]
    d.text((fx + S(80), fy + S(8)), url[:60], font=F["url"], fill="#9aa3b2")
    scaled = page.resize((fw, ph), Image.LANCZOS)
    mask = Image.new("L", (fw, ph), 255)
    md = ImageDraw.Draw(mask)
    md.rounded_rectangle((0, -S(12), fw, ph), radius=S(10), fill=255)
    img.paste(scaled, (fx, fy + bar), mask)

    # right column
    rx, ry = S(876), S(148)
    d.text((rx, ry), "JEV-TURBO", font=F["label"], fill=ACCENT)
    clock = f"{min(t_ms, done_at or t_ms) / 1000:05.2f}"
    d.text((rx - S(2), ry + S(18)), clock, font=F["clock"], fill=TEXT)
    d.text((rx, ry + S(84)), "SECONDS ELAPSED", font=F["clockcap"], fill=DIM)

    # rows
    y = ry + S(116)
    rh = S(38)
    max_rows = 9
    cur = sum(1 for r in rows if r["t"] <= t_ms)
    start = max(0, min(cur - max_rows + 1, len(rows) - max_rows)) if len(rows) > max_rows else 0
    for r in rows[start:start + max_rows]:
        state = "done" if r["t"] <= t_ms else "todo"
        cx, cy = rx + S(10), y + S(11)
        if state == "done":
            d.ellipse((cx - S(10), cy - S(10), cx + S(10), cy + S(10)), fill=ACCENT)
            d.line((cx - S(5), cy, cx - S(1), cy + S(4), cx + S(5), cy - S(4)), fill="white", width=S(2))
        else:
            d.ellipse((cx - S(10), cy - S(10), cx + S(10), cy + S(10)), fill=SUBTLE, outline=BORDER)
        d.text((rx + S(32), y - S(1)), r["label"], font=F["row"], fill=TEXT if state == "done" else DIM)
        sub = f"{r['view']} › {r['comp']}" if r["view"] and r["comp"] else (r["comp"] or "")
        d.text((rx + S(32), y + S(20)), sub, font=F["rowsub"], fill=ACCENT if state == "done" else DIM)
        y += rh

    # one line of numbers, then the finish check
    sy = ry + S(116) + rh * min(max_rows, len(rows)) + S(4)
    stats = []
    if median_pick:
        stats.append(f"{median_pick} ms median decision")
    stats.append(f"{len(rows)} actions")
    d.text((rx, sy), " · ".join(stats), font=F["cardline"], fill=DIM)
    cy0 = sy + S(26)
    finished = done_at is not None and t_ms >= done_at
    ch = S(30) + S(18) * max(1, len(checks)) + S(12)
    rounded(d, (rx, cy0, W - S(28), cy0 + ch), S(10), fill=ACCENT_DIM if finished else SUBTLE)
    d.text((rx + S(14), cy0 + S(10)), "Goal reached" if finished else "Finish check", font=F["card"], fill=ACCENT if finished else TEXT2)
    yy = cy0 + S(32)
    for c in checks:
        mark = "✓ " if finished else "· "
        d.text((rx + S(14), yy), (mark + c)[:38], font=F["cardline"], fill=TEXT2 if finished else DIM)
        yy += S(18)

    # footer
    py = H - S(44)
    d.rectangle((S(28), py, W - S(28), py + S(3)), fill=BORDER)
    frac = min(1.0, t_ms / max(1, done_at or total_ms))
    d.rectangle((S(28), py, S(28) + round((W - S(56)) * frac), py + S(3)), fill=ACCENT)
    d.text((S(28), py + S(12)), "Jev picks each step from the sightmap's named components. Typed values come from the spec. Original timing, waits included.", font=F["foot"], fill=DIM)
    tag = "github.com/sightmap/jev-turbo"
    d.text((W - S(28) - text_w(d, tag, F["footmono"]), py + S(13)), tag, font=F["footmono"], fill=DIM)
    return img


def frame_at(t_ms):
    cur = frames[0]
    for fr in frames:
        if fr["t"] <= t_ms:
            cur = fr
    return cur


if args.still is not None:
    fr = frame_at(args.still)
    page = Image.open(os.path.join(src, fr["file"])).convert("RGB")
    compose(page, args.still).save(args.out)
    print(args.out)
    sys.exit(0)

tmp = tempfile.mkdtemp(prefix="jev-turbo-demo-")
rendered = []
for i, fr in enumerate(frames):
    page = Image.open(os.path.join(src, fr["file"])).convert("RGB")
    path = os.path.join(tmp, f"r{i:05d}.jpg")
    compose(page, fr["t"]).save(path, quality=90)
    rendered.append(path)

with open(os.path.join(tmp, "list.txt"), "w") as f:
    for i, fr in enumerate(frames):
        nxt = frames[i + 1]["t"] if i + 1 < len(frames) else fr["t"] + tail_ms
        f.write(f"file '{rendered[i]}'\nduration {max(nxt - fr['t'], 16) / 1000:.3f}\n")
    f.write(f"file '{rendered[-1]}'\n")

os.makedirs(os.path.dirname(os.path.abspath(args.out)) or ".", exist_ok=True)
mp4, gif = args.out + ".mp4", args.out + ".gif"
subprocess.check_call(["ffmpeg", "-y", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", os.path.join(tmp, "list.txt"),
                       "-vf", "fps=20,format=yuv420p", "-c:v", "libx264", "-preset", "slow", "-crf", "23", "-movflags", "+faststart", mp4])
palette = os.path.join(tmp, "palette.png")
subprocess.check_call(["ffmpeg", "-y", "-loglevel", "error", "-i", mp4, "-vf", "fps=12,scale=1152:-1:flags=lanczos,palettegen=max_colors=128", palette])
subprocess.check_call(["ffmpeg", "-y", "-loglevel", "error", "-i", mp4, "-i", palette, "-lavfi",
                       "fps=12,scale=1152:-1:flags=lanczos[x];[x][1:v]paletteuse=dither=bayer:bayer_scale=5", "-loop", "0", gif])
shutil.rmtree(tmp, ignore_errors=True)
print(f"{mp4}: {os.path.getsize(mp4) // 1024} KB, {gif}: {os.path.getsize(gif) // 1024} KB, {len(frames)} frames, {len(rows)} actions")
