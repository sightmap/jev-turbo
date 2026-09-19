#!/usr/bin/env python3
"""Assemble one or more recorded runs (jev-turbo explore --record DIR) into an MP4 and a GIF.

    python3 scripts/render-demo.py DIR out/demo [--title "Zürich → London"] [--subtitle "..."]
    python3 scripts/render-demo.py DIR out/still.png --still 3200      # one composed frame at 3.2 s

    python3 scripts/render-demo.py --compare "without a map=DIR1" "with a map=DIR2" out/demo [--title ...] [--subtitle ...]
    python3 scripts/render-demo.py --compare "without a map=DIR1" "with a map=DIR2" out/still.png --still 1:3200

The page sits on the left in a browser frame. The run sits on the right: the
elapsed clock, one row per action with the sightmap component it used (or the
sightkick tool it ran) and how many candidates it chose among, the finish
check, and the median Jev pick. Frames play at the wall-clock intervals they
were captured at, so the video runs at 1x.

In --compare mode, any number of "LABEL=DIR" acts render back to back into one
video, each with a pill in the top right showing its position and label. Every
act but the last holds 1.2 s on its final frame before the next begins; the
last holds 1.5 s. The default --title summarizes step counts per act. Needs
ffmpeg and Pillow.
"""
import argparse, json, os, re, shutil, subprocess, sys, tempfile

from PIL import Image, ImageDraw, ImageFont

ap = argparse.ArgumentParser()
ap.add_argument("--compare", action="store_true", help="render several recordings as acts of one video")
ap.add_argument("rest", nargs="+", help="SRC OUT, or with --compare: LABEL=DIR ... OUT")
ap.add_argument("--title", default="")
ap.add_argument("--subtitle", default="One goal. Every pick by Jev over a sightmap.")
ap.add_argument("--still", default=None, help="render one frame and stop: MS, or with --compare: ACT_INDEX:MS")
ap.add_argument("--width", type=int, default=1200)
args = ap.parse_args()

if args.compare:
    if len(args.rest) < 2:
        ap.error("--compare needs at least one LABEL=DIR act and an OUT path")
    *act_args, out = args.rest
    acts_spec = []
    for a in act_args:
        if "=" not in a:
            ap.error(f"expected LABEL=DIR, got {a!r}")
        label, _, d = a.partition("=")
        acts_spec.append((label, d))
else:
    if len(args.rest) != 2:
        ap.error("expected SRC OUT")
    src, out = args.rest
    acts_spec = [(None, src)]

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
    "brand": ("bold", 22), "brand2": ("sans", 17), "pill": ("mono", 11), "sub": ("sans", 17),
    "url": ("mono", 12), "label": ("mono", 12), "clock": ("monobold", 54), "clockcap": ("mono", 11),
    "row": ("bold", 16), "rowsub": ("mono", 11), "card": ("bold", 15), "cardline": ("mono", 12),
    "stat": ("monobold", 22), "statcap": ("sans", 12), "foot": ("sans", 12), "footmono": ("mono", 11),
}.items()}

H1_SIZE = 40

VERBS = "filled|clicked|selected|typed|chose|opened"
ACTION = re.compile(r'^(%s) \[(\w+)((?: [^\]]*)?)\](?: with (\w+))?' % VERBS)
RAW = re.compile(r'^(%s) ([a-z]+) "(.*)"(?: with (\w+))?$' % VERBS)
PROP = re.compile(r'(\w+)="([^"]*)"')
TOOL_TEXT = re.compile(r'^ran tool (\w+)\(|^tool (\w+) failed:')

MAX_LABEL = 26


def shorten(label):
    return label if len(label) <= MAX_LABEL else label[:MAX_LABEL - 1] + "…"


def human(text, values):
    """A line for a person, the sightmap component, and the raw role when there is none.

    With a map an action reads `clicked [BagLink count="..."]`. Without one it
    reads `clicked link "Shopping bag, 1 items"`: there is no component, so the
    row carries the control's own name and, under it, the role the raw tree
    gave it.
    """
    m = ACTION.match(text)
    if m:
        verb, comp, props, key = m.groups()
        props = dict(PROP.findall(props or ""))
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
        return shorten(label), comp, ""
    m = RAW.match(text)
    if m:
        verb, role, name, key = m.groups()
        label = str(values[key]) if verb == "filled" and key and key in values else name.replace('\\"', '"')
        return shorten(label), "", role
    if text.startswith("waited"):
        return "Results load", "wait", ""
    if text.startswith("went back"):
        return "Back", "back", ""
    if text.startswith("scrolled"):
        return "Scroll", "scroll", ""
    if text.startswith("pressed"):
        return "Enter", "enter", ""
    return shorten(text), "", ""


def tool_name(text):
    m = TOOL_TEXT.match(text)
    return (m.group(1) or m.group(2)) if m else None


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


def title_case(name, sep=" "):
    return name.replace("-", sep).title()


def load(src):
    """Load one recording directory into everything compose() needs to draw it."""
    frames = [json.loads(l) for l in open(os.path.join(src, "frames.jsonl")) if l.strip()]
    events = [json.loads(l) for l in open(os.path.join(src, "events.jsonl")) if l.strip()]
    if not frames:
        sys.exit(f"no frames: {src}")
    run_path = os.path.join(src, "run.json")
    run = json.load(open(run_path)) if os.path.exists(run_path) else {}
    if "runs" in run:
        run = run["runs"][0]
    spec = run.get("spec") or {}
    values = spec.get("values") or {}
    total_ms = run.get("ms") or frames[-1]["t"]
    steps_by_n = {s.get("step"): s for s in run.get("steps", [])}

    rows = []
    for ev in events:
        if ev["text"].startswith("stale") or ev["text"] == "done":
            continue
        step = steps_by_n.get(ev["step"], {})
        tool = step.get("tool") or ""
        if tool:
            label = tool_name(ev["text"]) or tool
            sub = f"tool › {tool}"
        else:
            label, comp, role = human(ev["text"], values)
            if comp:
                sub = f"{ev.get('view', '')} › {comp}" if ev.get("view") else comp
            else:
                sub = role
        rows.append({"t": ev["t"], "label": label, "sub": sub, "candidates": step.get("candidates")})

    done_at = next((ev["t"] for ev in events if ev["text"] == "done"), None)
    ok = bool(run.get("ok"))
    if ok and done_at is None:
        done_at = total_ms

    checks = check_lines(spec.get("done_when") or run.get("done_when"))[:3]
    picks = [s.get("ms_pick") for s in run.get("steps", []) if s.get("ms_pick")]
    median_pick = sorted(picks)[len(picks) // 2] if picks else None

    return {
        "src": src, "frames": frames, "timeline": timeline(frames, done_at or total_ms), "run": run, "rows": rows,
        "capture_end": frames[-1]["t"],
        "done_at": done_at, "total_ms": total_ms, "checks": checks, "median_pick": median_pick,
        "ok": ok, "name": run.get("name") or "", "n_steps": len(run.get("steps", [])),
        "url": (run.get("steps") or [{}])[0].get("url", "") or "",
        "llm_calls": (run.get("picker_stats") or {}).get("llm_calls", 0),
    }


HELD_STEP_MS = 500  # cadence of the held tail below


def timeline(frames, end_ms):
    """(t, file, stale) per composed frame, one entry per captured frame.

    Frame capture can stop before a run does: the page the run then leaves
    stops yielding screenshots and no later frame is written. The act still
    has to end where the run ended, so the last captured page is held, at 1x,
    to the run's end, and every held entry is marked stale so the frame can
    say the page stopped updating.
    """
    out = [(fr["t"], fr["file"], False) for fr in frames]
    last_t, last_file = frames[-1]["t"], frames[-1]["file"]
    t = last_t + HELD_STEP_MS
    while t < end_ms:
        out.append((t, last_file, True))
        t += HELD_STEP_MS
    if end_ms > last_t:
        out.append((end_ms, last_file, True))
    return out


def single_title(rec):
    secs = f"{rec['total_ms'] / 1000:.1f} seconds."
    return f"{title_case(rec['name'], ' → ')}. {secs}" if rec["name"] else secs


def compare_title(name, acts):
    """acts: [(label, rec), ...] in play order."""
    clauses = []
    for i, (label, rec) in enumerate(acts):
        clause = f"{rec['n_steps']} steps {label}" if i == 0 else f"{rec['n_steps']} {label}"
        if not rec["ok"]:
            clause += " (not reached)"
        clauses.append(clause)
    body = f"{', '.join(clauses)}."
    return f"{title_case(name)}. {body}" if name else body


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


def clip(draw, s, f, max_w):
    """s, trimmed with an ellipsis until it draws no wider than max_w."""
    if not s or text_w(draw, s, f) <= max_w:
        return s
    while s and text_w(draw, s + "…", f) > max_w:
        s = s[:-1]
    return s + "…"


def fit_font(text, kind, max_w, start_size, min_size=20):
    """The largest size of kind, down to min_size, that fits text in max_w.

    A single-dir title is short and always fits at start_size; a --compare
    title lists steps per act and can run long, so it shrinks to fit instead
    of running off the frame.
    """
    probe = ImageDraw.Draw(Image.new("RGB", (1, 1)))
    for size in range(start_size, min_size, -1):
        f = font(kind, size)
        if text_w(probe, text, f) <= max_w:
            return f
    return font(kind, min_size)


def compose(rec, page, t_ms, act, stale=False):
    """One frame: the page on the left, the run's progress on the right.

    act is None in single-dir mode, or {"label", "index", "count", "accent"}
    when playing one act of a --compare video. stale marks a frame drawn after
    frame capture stopped, where the page image is the last one captured and
    only the clock and the rows are still moving.
    """
    img = Image.new("RGB", (W, H), BG)
    d = ImageDraw.Draw(img)

    # header
    x = S(28)
    d.text((x, S(26)), "jev-turbo", font=F["brand"], fill=TEXT)
    x += text_w(d, "jev-turbo", F["brand"]) + S(12)
    d.text((x, S(30)), "sightmap × TypeSafe Jev", font=F["brand2"], fill=DIM)
    if act is None:
        pill = "LIVE SITE · 1× SPEED · 0 LLM CALLS" if not rec["llm_calls"] else "LIVE SITE · 1× SPEED"
        pill_fill, pill_text = ACCENT_DIM, ACCENT
    else:
        pill = f"{act['index'] + 1}/{act['count']} · {act['label'].upper()}"
        pill_fill, pill_text = (ACCENT_DIM, ACCENT) if act["accent"] else (SUBTLE, DIM)
    pw = text_w(d, pill, F["pill"]) + S(28)
    rounded(d, (W - S(28) - pw, S(24), W - S(28), S(24) + S(26)), S(13), fill=pill_fill)
    d.text((W - S(28) - pw + S(14), S(31)), pill, font=F["pill"], fill=pill_text)
    d.text((S(28), S(58)), TITLE, font=TITLE_FONT, fill=TEXT)
    d.text((S(28), S(108)), SUBTITLE, font=F["sub"], fill=DIM)

    # browser frame
    fx, fy, fw = S(28), S(146), S(820)
    bar = S(30)
    ph = round(page.height * fw / page.width)
    rounded(d, (fx, fy, fx + fw, fy + bar + ph), S(10), fill=DARK, outline=BORDER)
    for i, c in enumerate(("#ff5f57", "#febc2e", "#28c840")):
        cx = fx + S(16) + i * S(18)
        d.ellipse((cx, fy + S(10), cx + S(10), fy + S(20)), fill=c)
    url = re.sub(r"^https?://(www\.)?", "", rec["url"]).split("?")[0]
    d.text((fx + S(80), fy + S(8)), url[:60], font=F["url"], fill="#9aa3b2")
    if stale:
        note = f"last captured page · {rec['capture_end'] / 1000:.1f} s"
        d.text((fx + fw - S(14) - text_w(d, note, F["url"]), fy + S(8)), note, font=F["url"], fill="#6f7787")
    scaled = page.resize((fw, ph), Image.LANCZOS)
    mask = Image.new("L", (fw, ph), 255)
    md = ImageDraw.Draw(mask)
    md.rounded_rectangle((0, -S(12), fw, ph), radius=S(10), fill=255)
    img.paste(scaled, (fx, fy + bar), mask)

    # right column
    rx, ry = S(876), S(148)
    d.text((rx, ry), "JEV-TURBO", font=F["label"], fill=ACCENT)
    clock = f"{min(t_ms, rec['done_at'] or t_ms) / 1000:05.2f}"
    d.text((rx - S(2), ry + S(18)), clock, font=F["clock"], fill=TEXT)
    d.text((rx, ry + S(84)), "SECONDS ELAPSED", font=F["clockcap"], fill=DIM)
    cap = "choices"
    d.text((W - S(28) - text_w(d, cap, F["label"]), ry + S(84)), cap, font=F["label"], fill=DIM)

    # rows
    rows = rec["rows"]
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
        ntxt = str(r["candidates"]) if r["candidates"] is not None else ""
        nw = text_w(d, ntxt, F["rowsub"]) if ntxt else 0
        lx = rx + S(32)
        avail = W - S(28) - nw - S(12) - lx
        d.text((lx, y - S(1)), clip(d, r["label"], F["row"], avail), font=F["row"], fill=TEXT if state == "done" else DIM)
        d.text((lx, y + S(20)), clip(d, r["sub"], F["rowsub"], avail), font=F["rowsub"], fill=ACCENT if state == "done" else DIM)
        if ntxt:
            d.text((W - S(28) - nw, y + S(4)), ntxt, font=F["rowsub"], fill=DIM)
        y += rh

    # one line of numbers, then the finish check
    sy = ry + S(116) + rh * min(max_rows, len(rows)) + S(4)
    stats = []
    if rec["median_pick"]:
        stats.append(f"{rec['median_pick']} ms median decision")
    stats.append(f"{len(rows)} actions")
    d.text((rx, sy), " · ".join(stats), font=F["cardline"], fill=DIM)
    cy0 = sy + S(26)
    if not rec["ok"]:
        ch = S(30) + S(18) + S(12)
        rounded(d, (rx, cy0, W - S(28), cy0 + ch), S(10), fill=SUBTLE)
        d.text((rx + S(14), cy0 + S(10)), f"Not reached · {rec['n_steps']} steps", font=F["card"], fill=TEXT2)
    else:
        checks = rec["checks"]
        finished = rec["done_at"] is not None and t_ms >= rec["done_at"]
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
    frac = min(1.0, t_ms / max(1, rec["done_at"] or rec["total_ms"]))
    d.rectangle((S(28), py, S(28) + round((W - S(56)) * frac), py + S(3)), fill=ACCENT)
    d.text((S(28), py + S(12)), "Jev picks each step from the sightmap's named components. Typed values come from the spec. Original timing, waits included.", font=F["foot"], fill=DIM)
    tag = "github.com/sightmap/jev-turbo"
    d.text((W - S(28) - text_w(d, tag, F["footmono"]), py + S(13)), tag, font=F["footmono"], fill=DIM)
    return img


def frame_at(rec, t_ms):
    cur = rec["timeline"][0]
    for entry in rec["timeline"]:
        if entry[0] <= t_ms:
            cur = entry
    return cur


def render_frames(rec, act, hold_ms, tmp, start_idx):
    """Compose every frame of one recording to tmp, holding hold_ms on the last.

    Returns (paths, durations_ms, next_start_idx).
    """
    paths, cache = [], {}
    for t, file, stale in rec["timeline"]:
        if file not in cache:
            cache = {file: Image.open(os.path.join(rec["src"], file)).convert("RGB")}
        path = os.path.join(tmp, f"r{start_idx + len(paths):05d}.jpg")
        compose(rec, cache[file], t, act, stale).save(path, quality=90)
        paths.append(path)
    ts = [t for t, _, _ in rec["timeline"]]
    durations = [max(ts[i + 1] - ts[i], 16) for i in range(len(ts) - 1)] + [hold_ms]
    return paths, durations, start_idx + len(paths)


TAIL_MS = 700  # single-dir mode's hold on the last frame, unchanged from before --compare

n_acts = len(acts_spec)
acts = []
for i, (label, d) in enumerate(acts_spec):
    rec = load(d)
    if label is None:
        act_dict, hold_ms = None, TAIL_MS
    else:
        act_dict = {"label": label, "index": i, "count": n_acts, "accent": i != 0}
        hold_ms = 1500 if i == n_acts - 1 else 1200
    acts.append((rec, act_dict, hold_ms))

if args.compare:
    TITLE = args.title or compare_title(acts[0][0]["name"], [(a["label"], r) for r, a, _ in acts])
else:
    TITLE = args.title or single_title(acts[0][0])
SUBTITLE = args.subtitle
TITLE_FONT = fit_font(TITLE, "bold", W - S(56), H1_SIZE)

if args.still is not None:
    if args.compare:
        if ":" not in args.still:
            ap.error("--compare --still needs ACT_INDEX:MS")
        idx_s, _, ms_s = args.still.partition(":")
        idx, ms = int(idx_s), int(ms_s)
    else:
        idx, ms = 0, int(args.still)
    rec, act_dict, _ = acts[idx]
    _, file, stale = frame_at(rec, ms)
    page = Image.open(os.path.join(rec["src"], file)).convert("RGB")
    compose(rec, page, ms, act_dict, stale).save(out)
    print(out)
    sys.exit(0)

tmp = tempfile.mkdtemp(prefix="jev-turbo-demo-")
all_paths, all_durations, idx = [], [], 0
for rec, act_dict, hold_ms in acts:
    paths, durations, idx = render_frames(rec, act_dict, hold_ms, tmp, idx)
    all_paths += paths
    all_durations += durations

with open(os.path.join(tmp, "list.txt"), "w") as f:
    for path, dur in zip(all_paths, all_durations):
        f.write(f"file '{path}'\nduration {dur / 1000:.3f}\n")
    f.write(f"file '{all_paths[-1]}'\n")

os.makedirs(os.path.dirname(os.path.abspath(out)) or ".", exist_ok=True)
mp4, gif = out + ".mp4", out + ".gif"
subprocess.check_call(["ffmpeg", "-y", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", os.path.join(tmp, "list.txt"),
                       "-vf", "fps=20,format=yuv420p", "-c:v", "libx264", "-preset", "slow", "-crf", "23", "-movflags", "+faststart", mp4])
# 5 fps and 24 colors, so a demo this long still fits in a README GIF under
# 4 MB; the MP4 is 20 fps and full color. (With act 2 now fully captured
# instead of mostly a held frame, the video has much more real motion for
# its length, so it needs a lower fps/color budget than a mostly-static cut
# of the same duration would.)
palette = os.path.join(tmp, "palette.png")
subprocess.check_call(["ffmpeg", "-y", "-loglevel", "error", "-i", mp4, "-vf", "fps=5,scale=1152:-1:flags=lanczos,palettegen=max_colors=24", palette])
subprocess.check_call(["ffmpeg", "-y", "-loglevel", "error", "-i", mp4, "-i", palette, "-lavfi",
                       "fps=5,scale=1152:-1:flags=lanczos[x];[x][1:v]paletteuse=dither=bayer:bayer_scale=5", "-loop", "0", gif])
shutil.rmtree(tmp, ignore_errors=True)
total_frames = sum(len(rec["timeline"]) for rec, _, _ in acts)
total_rows = sum(len(rec["rows"]) for rec, _, _ in acts)
print(f"{mp4}: {os.path.getsize(mp4) // 1024} KB, {gif}: {os.path.getsize(gif) // 1024} KB, {total_frames} frames, {total_rows} actions")
