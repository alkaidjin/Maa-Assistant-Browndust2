# -*- coding: utf-8 -*-
"""从源图生成多尺寸 .ico（用于 mxu.exe / 启动器图标）。

用法:
    python tools/make_icon.py <源图路径>            # 只出预览图 cache/_icon_preview.png
    python tools/make_icon.py <源图路径> --final    # 额外写出 mxu.ico / mxu_icon.png 到项目根

所有路径都相对脚本自身解析（ROOT = tools/ 的上一级），换目录、换机器都不用改代码。
依赖 Pillow（本仓库 managed venv 里已装；系统 Python 需 pip install pillow）。
"""
import os
import sys

try:
    from PIL import Image, ImageDraw
except ImportError:
    sys.exit("需要 Pillow：pip install pillow（或使用本仓库的 managed venv）")

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CACHE = os.path.join(ROOT, "cache")

# 裁剪框：聚焦角色头部/面部，避开左侧蓝色人物与右下角水印（针对 1000x1000 量级素材）
BOX = (322, 38, 678, 394)      # 356x356
RADIUS_RATIO = 0.16            # 圆角比例（0 = 直角）


def rounded(img, ratio):
    w, h = img.size
    r = int(min(w, h) * ratio)
    if r <= 0:
        return img
    mask = Image.new("L", (w, h), 0)
    d = ImageDraw.Draw(mask)
    d.rounded_rectangle([0, 0, w - 1, h - 1], radius=r, fill=255)
    out = img.convert("RGBA")
    out.putalpha(mask)
    return out


def main():
    argv = sys.argv[1:]
    final = "--final" in argv
    srcs = [a for a in argv if not a.startswith("-")]
    if not srcs:
        sys.exit("[!] 缺少源图路径。\n\n" + __doc__.strip())
    src = os.path.abspath(srcs[0])
    if not os.path.isfile(src):
        sys.exit("[!] 找不到源图: %s" % src)

    os.makedirs(CACHE, exist_ok=True)
    im = Image.open(src).convert("RGB")
    print("SRC     ", src, im.size)
    crop = im.crop(BOX)

    # 预览：256 圆角
    prev = rounded(crop.resize((256, 256), Image.LANCZOS), RADIUS_RATIO)
    prev_path = os.path.join(CACHE, "_icon_preview.png")
    prev.save(prev_path)
    print("PREVIEW ", prev_path, "BOX", BOX)

    if final:
        sizes = [16, 20, 24, 32, 40, 48, 64, 96, 128, 256]
        # 基准 256 圆角图，逐尺寸重采样（保持圆角干净）
        base = rounded(crop.resize((256, 256), Image.LANCZOS), RADIUS_RATIO)
        ico = os.path.join(ROOT, "mxu.ico")
        base.save(ico, format="ICO", sizes=[(s, s) for s in sizes])
        print("ICO     ", ico, os.path.getsize(ico))
        png = os.path.join(ROOT, "mxu_icon.png")
        base.save(png)
        print("PNG     ", png)


if __name__ == "__main__":
    main()
