"""Render the Upgaming mark to a 1-bit thermal asset.

Quick Look stretches the artwork, so the paths are flattened and filled here
instead. The file only uses M, L, V, C and Z, all absolute.
"""
import re, sys, pathlib
from PIL import Image, ImageDraw, ImageChops

NUM = r'-?\d*\.?\d+(?:[eE][-+]?\d+)?'


def subpaths(d):
    toks = re.findall(r'[A-Za-z]|' + NUM, d)
    i, cur, start, out, sp = 0, (0.0, 0.0), (0.0, 0.0), [], []

    def num():
        nonlocal i
        v = float(toks[i]); i += 1
        return v

    def more():
        return i < len(toks) and not toks[i].isalpha()

    def cubic(p0, p1, p2, p3, steps=24):
        pts = []
        for s in range(1, steps + 1):
            t = s / steps; u = 1 - t
            pts.append((
                u*u*u*p0[0] + 3*u*u*t*p1[0] + 3*u*t*t*p2[0] + t*t*t*p3[0],
                u*u*u*p0[1] + 3*u*u*t*p1[1] + 3*u*t*t*p2[1] + t*t*t*p3[1]))
        return pts

    while i < len(toks):
        cmd = toks[i]; i += 1
        while True:
            if cmd == 'M':
                if len(sp) > 2:
                    out.append(sp)
                cur = (num(), num()); start = cur; sp = [cur]
            elif cmd == 'L':
                cur = (num(), num()); sp.append(cur)
            elif cmd == 'V':
                cur = (cur[0], num()); sp.append(cur)
            elif cmd == 'H':
                cur = (num(), cur[1]); sp.append(cur)
            elif cmd == 'C':
                p1 = (num(), num()); p2 = (num(), num()); p3 = (num(), num())
                sp.extend(cubic(cur, p1, p2, p3)); cur = p3
            elif cmd == 'Z':
                sp.append(start); cur = start
                if len(sp) > 2:
                    out.append(sp)
                sp = [cur]
                break
            else:
                raise SystemExit(f'unsupported path command {cmd!r}')
            if not more():
                break
    if len(sp) > 2:
        out.append(sp)
    return out


def render(svg_path, height, ss=8):
    svg = pathlib.Path(svg_path).read_text()
    vx, vy, vw, vh = (float(v) for v in re.search(r'viewBox="([^"]+)"', svg).group(1).split())

    width = round(height * vw / vh)
    W, H = width * ss, height * ss
    sx, sy = W / vw, H / vh

    union = Image.new('1', (W, H), 0)
    for d in re.findall(r'\sd="([^"]+)"', svg):
        # fill-rule is evenodd, so subpaths cut holes in each other.
        acc = Image.new('1', (W, H), 0)
        for sp in subpaths(d):
            layer = Image.new('1', (W, H), 0)
            ImageDraw.Draw(layer).polygon(
                [((x - vx) * sx, (y - vy) * sy) for x, y in sp], fill=1)
            acc = ImageChops.logical_xor(acc, layer)
        union = ImageChops.logical_or(union, acc)

    # Downsample the supersampled coverage, then commit to one ink.
    grey = union.convert('L').resize((width, height), Image.LANCZOS)
    out = Image.new('L', (width, height), 255)
    out.paste(0, (0, 0), grey.point(lambda v: 255 if v > 110 else 0))
    return out.convert('1'), width


img, w = render(sys.argv[1], int(sys.argv[2]))
img.save(sys.argv[3])
print(f'{sys.argv[3]} {w}x{sys.argv[2]} ({w/12:.1f} columns)')
