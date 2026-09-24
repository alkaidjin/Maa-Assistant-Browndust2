# -*- coding: utf-8 -*-
"""Generate the offline fixture used by agent/rock-picker's integration test.

Renders a 1280x720 "圣石洞穴 资源栏" strip: white bold numbers on a dark panel,
right-aligned and ending at x=1185, one row per cave, at the row geometry the
pipeline uses (see resource/pipeline/HuntingArea.json -> PickLeastRock).

This only exists so the OCR -> argmin -> OverrideNext routing can be exercised
without the game. It is NOT a pixel-accurate copy of the game UI.

Usage:  python gen_fixture.py
Output: rock_bar.png (the readable one), rock_bar_blank.png (nothing readable)
"""
import os

from PIL import Image, ImageDraw, ImageFont

WIDTH, HEIGHT = 1280, 720
# (label, value, roi_y) — keep in sync with the pipeline's rows.
ROWS = [
    ("火", 13010, 56),
    ("水", 9177, 84),
    ("风", 12008, 112),
    ("光", 10722, 140),
    ("暗", 16724, 168),
]
ROI_X, ROI_W, ROI_H = 1100, 95, 20
RIGHT_EDGE = 1185

FONT_CANDIDATES = [
    r"C:\Windows\Fonts\arialbd.ttf",
    r"C:\Windows\Fonts\msyhbd.ttc",
    r"C:\Windows\Fonts\segoeuib.ttf",
]

HERE = os.path.dirname(os.path.abspath(__file__))


def load_font(size):
    for path in FONT_CANDIDATES:
        if os.path.exists(path):
            return ImageFont.truetype(path, size)
    raise SystemExit("no usable bold font found")


def new_panel():
    img = Image.new("RGB", (WIDTH, HEIGHT), (24, 26, 34))
    ImageDraw.Draw(img).rectangle([1000, 30, 1230, 210], fill=(16, 18, 24))
    return img


def draw_rows(img, values, size):
    draw = ImageDraw.Draw(img)
    font = load_font(size)
    for (label, _default, y), value in zip(ROWS, values):
        text = "{:,}".format(value)
        box = draw.textbbox((0, 0), text, font=font)
        w = box[2] - box[0]
        draw.text((RIGHT_EDGE - w, y + 2 - box[1]), text, font=font, fill=(245, 245, 245))
    return img


def main():
    values = [v for _l, v, _y in ROWS]

    img = draw_rows(new_panel(), values, 14)
    img.save(os.path.join(HERE, "rock_bar.png"))

    # Same layout but with an unreadable glyph size, for the "cannot read" path.
    img = draw_rows(new_panel(), values, 4)
    img.save(os.path.join(HERE, "rock_bar_blank.png"))

    # Crop for a quick visual check of what the ROIs actually cover.
    img.crop((1080, 40, 1210, 200)).resize((130 * 3, 160 * 3), Image.NEAREST).save(
        os.path.join(HERE, "rock_bar_crop.png")
    )
    print("wrote rock_bar.png / rock_bar_blank.png / rock_bar_crop.png")


if __name__ == "__main__":
    main()
