#!/usr/bin/env python3
"""Assemble a recorded run (jev-turbo explore --record DIR) into an MP4 and a GIF.

    python3 scripts/render-demo.py DIR out/demo        # writes out/demo.mp4 and out/demo.gif

Frames play back at the wall-clock intervals they were captured at, so the
video runs at 1x. With Pillow installed (pip install pillow) each step's action
is burned in as a caption at the bottom and the elapsed time at the top right;
without it the frames are encoded as they are. Needs ffmpeg.
"""
import json, os, shutil, subprocess, sys, tempfile

if len(sys.argv) < 3:
    sys.exit(__doc__)
src, out = sys.argv[1], sys.argv[2]
width = int(sys.argv[3]) if len(sys.argv) > 3 else 960

frames = [json.loads(l) for l in open(os.path.join(src, "frames.jsonl")) if l.strip()]
events = [json.loads(l) for l in open(os.path.join(src, "events.jsonl")) if l.strip()]
if not frames:
    sys.exit("no frames")
run_path = os.path.join(src, "run.json")
run = json.load(open(run_path)) if os.path.exists(run_path) else {}
tail_ms = 700

try:
    from PIL import Image, ImageDraw, ImageFont
except ImportError:  # captions are optional
    Image = None

tmp = tempfile.mkdtemp(prefix="jev-turbo-demo-")


def caption_for(t_ms):
    cur = None
    for ev in events:
        if ev["t"] <= t_ms:
            cur = ev
    return f"{cur['step']}. {cur['text']}" if cur else ""


def font(size):
    for f in ["/System/Library/Fonts/Supplemental/Arial.ttf", "/System/Library/Fonts/Helvetica.ttc",
              "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", "/usr/share/fonts/dejavu/DejaVuSans.ttf"]:
        if os.path.exists(f):
            return ImageFont.truetype(f, size)
    return ImageFont.load_default()


head = f"jev-turbo · Jev picks every step · {len(events)} steps"
if run.get("ms"):
    head += f" · {run['ms'] / 1000:.1f} s"


def box(draw, xy, text, fnt, anchor_right=False, img_w=0):
    x, y = xy
    l, t, r, b = draw.textbbox((0, 0), text, font=fnt)
    w, h = r - l, b - t
    pad = 10
    if anchor_right:
        x = img_w - w - pad * 2 - x
    draw.rectangle([x, y, x + w + pad * 2, y + h + pad * 2], fill=(0, 0, 0, 170))
    draw.text((x + pad - l, y + pad - t), text, font=fnt, fill=(255, 255, 255, 255))


rendered = []
for i, fr in enumerate(frames):
    path = os.path.abspath(os.path.join(src, fr["file"]))
    if Image is not None:
        img = Image.open(path).convert("RGBA")
        if img.width != width:
            img = img.resize((width, round(img.height * width / img.width)), Image.LANCZOS)
        overlay = Image.new("RGBA", img.size, (0, 0, 0, 0))
        draw = ImageDraw.Draw(overlay)
        big, small = font(max(16, width // 44)), font(max(14, width // 52))
        cap = caption_for(fr["t"])
        if cap:
            box(draw, (16, img.height - 16 - (max(16, width // 44) + 20)), cap[:110], big)
        box(draw, (16, 16), head, small)
        box(draw, (16, 16), f"{fr['t'] / 1000:.1f} s", small, anchor_right=True, img_w=img.width)
        img = Image.alpha_composite(img, overlay).convert("RGB")
        path = os.path.join(tmp, f"r{i:05d}.jpg")
        img.save(path, quality=88)
    rendered.append(path)

with open(os.path.join(tmp, "list.txt"), "w") as f:
    for i, fr in enumerate(frames):
        nxt = frames[i + 1]["t"] if i + 1 < len(frames) else fr["t"] + tail_ms
        f.write(f"file '{rendered[i]}'\nduration {max(nxt - fr['t'], 16) / 1000:.3f}\n")
    f.write(f"file '{rendered[-1]}'\n")

os.makedirs(os.path.dirname(os.path.abspath(out)) or ".", exist_ok=True)
mp4, gif = out + ".mp4", out + ".gif"
vf = f"scale={width}:-2:flags=lanczos,fps=20,format=yuv420p"
subprocess.check_call(["ffmpeg", "-y", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", os.path.join(tmp, "list.txt"),
                       "-vf", vf, "-c:v", "libx264", "-preset", "slow", "-crf", "23", "-movflags", "+faststart", mp4])
palette = os.path.join(tmp, "palette.png")
subprocess.check_call(["ffmpeg", "-y", "-loglevel", "error", "-i", mp4, "-vf", "fps=12,scale=800:-1:flags=lanczos,palettegen=max_colors=128", palette])
subprocess.check_call(["ffmpeg", "-y", "-loglevel", "error", "-i", mp4, "-i", palette, "-lavfi",
                       "fps=12,scale=800:-1:flags=lanczos[x];[x][1:v]paletteuse=dither=bayer:bayer_scale=5", "-loop", "0", gif])
shutil.rmtree(tmp, ignore_errors=True)
print(f"{mp4}: {os.path.getsize(mp4) // 1024} KB, {gif}: {os.path.getsize(gif) // 1024} KB, {len(frames)} frames, {len(events)} steps, captions={'yes' if Image else 'no'}")
