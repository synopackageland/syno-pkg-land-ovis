"""Build the DSM icon candidates from upstream's sheep (its artwork unchanged).

Maintainer tool (needs Pillow); the build never runs it. From the fork root:

    python3 synology/assets/make-icon.py app/assets/ovis-sheep.png <outdir>
    cp <outdir>/b-tile-dusk/icon_*.png synology/assets/icon/

Writes <outdir>/<variant>/icon_<size>.png for every size DSM loads. The package
uses variant b: a rounded dusk-blue tile, padded like DSM's own tiles, with a
fuller tile and no drop shadows at 32 px and below. Variant a (a cutout on
transparency) fades on DSM's white Package Center at 16-24 px; variant c's
green is outside Ovis's palette.
"""
import sys
from pathlib import Path
from PIL import Image, ImageDraw, ImageFilter, ImageChops

SIZES = [16, 24, 32, 48, 64, 72, 256]
W = 1024  # work canvas; downscaled with LANCZOS


def sheep(src, box):
    im = Image.open(src).convert("RGBA")
    im = im.crop(im.getbbox())
    s = box / max(im.size)
    return im.resize((round(im.width * s), round(im.height * s)), Image.LANCZOS)


def centered(canvas, im, dy=0):
    x = (canvas.width - im.width) // 2
    y = (canvas.height - im.height) // 2 + dy
    canvas.alpha_composite(im, (x, y))


def shadow(alpha, offset, blur, opacity, color=(20, 24, 40)):
    a = alpha.point(lambda v: v * opacity)
    sh = Image.new("RGBA", alpha.size, color + (0,))
    sh.putalpha(a)
    sh = sh.filter(ImageFilter.GaussianBlur(blur))
    out = Image.new("RGBA", alpha.size, (0, 0, 0, 0))
    out.alpha_composite(sh, (0, offset))
    return out


def outline(im, width, color):
    a = im.getchannel("A").point(lambda v: 255 if v > 96 else 0)
    grown = a.filter(ImageFilter.MaxFilter(width * 2 + 1)).filter(ImageFilter.GaussianBlur(1.2))
    ring = Image.new("RGBA", im.size, color + (0,))
    ring.putalpha(grown)
    ring.alpha_composite(im)
    return ring


def gradient(size, top, bottom):
    g = Image.new("RGBA", (1, 256))
    for y in range(256):
        t = y / 255
        g.putpixel((0, y), tuple(round(top[i] + (bottom[i] - top[i]) * t) for i in range(3)) + (255,))
    return g.resize(size, Image.BICUBIC)


def rounded_mask(size, box, radius):
    m = Image.new("L", size, 0)
    ImageDraw.Draw(m).rounded_rectangle(box, radius=radius, fill=255)
    return m


def tile(src, top, bottom, rim, margin, radius, sheep_frac, drop=True, sheep_shadow=True):
    c = Image.new("RGBA", (W, W), (0, 0, 0, 0))
    box = (margin, margin, W - margin, W - margin)
    mask = rounded_mask((W, W), box, radius)
    if drop:
        c.alpha_composite(shadow(mask, round(W * 0.016), W * 0.018, 0.40))
    face = gradient((W, W), top, bottom)
    # a soft top highlight: depth without a hard gloss line
    hl = Image.new("L", (W, W), 0)
    ImageDraw.Draw(hl).ellipse((-W * 0.3, -W * 0.85, W * 1.3, W * 0.42), fill=46)
    hl = hl.filter(ImageFilter.GaussianBlur(W * 0.05))
    face = Image.composite(Image.new("RGBA", (W, W), (255, 255, 255, 255)), face, hl)
    t = Image.new("RGBA", (W, W), (0, 0, 0, 0))
    t.paste(face, (0, 0), mask)
    # hairline rim, darker than the tile's bottom, so the edge holds on light and dark
    inner = rounded_mask((W, W), tuple(v + (s * 6) for v, s in zip(box, (1, 1, -1, -1))), radius - 6)
    ring = ImageChops.subtract(mask, inner)
    rim_l = Image.new("RGBA", (W, W), rim + (0,))
    rim_l.putalpha(ring.point(lambda v: v * 0.55))
    t.alpha_composite(rim_l)
    c.alpha_composite(t)
    s = sheep(src, round((W - 2 * margin) * sheep_frac))
    layer = Image.new("RGBA", (W, W), (0, 0, 0, 0))
    centered(layer, s, dy=round(W * 0.012))
    if sheep_shadow:
        c.alpha_composite(shadow(layer.getchannel("A"), round(W * 0.014), W * 0.012, 0.35, color=(10, 14, 30)))
    c.alpha_composite(layer)
    return c


def cutout(src, frac, stroke, drop=True):
    c = Image.new("RGBA", (W, W), (0, 0, 0, 0))
    s = sheep(src, round(W * frac))
    pad = 40
    s = s.crop((-pad, -pad, s.width + pad, s.height + pad))
    s = outline(s, stroke, (201, 178, 148))
    layer = Image.new("RGBA", (W, W), (0, 0, 0, 0))
    centered(layer, s, dy=-round(W * 0.01))
    if drop:
        c.alpha_composite(shadow(layer.getchannel("A"), round(W * 0.022), W * 0.022, 0.45))
    c.alpha_composite(layer)
    return c


def down(im, size):
    return im.resize((size, size), Image.LANCZOS)


VARIANTS = {
    # (a) shaped object on transparency, like DSM's own object icons
    "a-cutout": {
        "big": lambda src: cutout(src, 0.84, 10),
        "small": lambda src: cutout(src, 0.97, 22, drop=False),
    },
    # (b) rounded tile, dusk-blue gradient from the sheep's own navy features
    "b-tile-dusk": {
        "big": lambda src: tile(src, (104, 132, 214), (46, 58, 112), (24, 30, 64), 52, 210, 0.78),
        "small": lambda src: tile(src, (104, 132, 214), (46, 58, 112), (24, 30, 64), 8, 230, 0.86, drop=False, sheep_shadow=False),
    },
    # (c) rounded tile, meadow-green gradient: a sheep on grass
    "c-tile-meadow": {
        "big": lambda src: tile(src, (150, 214, 132), (58, 142, 92), (30, 84, 52), 52, 210, 0.78),
        "small": lambda src: tile(src, (150, 214, 132), (58, 142, 92), (30, 84, 52), 8, 230, 0.86, drop=False, sheep_shadow=False),
    },
}


def main():
    src, outdir = sys.argv[1], Path(sys.argv[2])
    for name, v in VARIANTS.items():
        big, small = v["big"](src), v["small"](src)
        d = outdir / name
        d.mkdir(parents=True, exist_ok=True)
        for size in SIZES:
            down(small if size <= 32 else big, size).save(d / f"icon_{size}.png", optimize=True)


main()
