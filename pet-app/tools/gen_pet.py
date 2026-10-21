"""用代码画像素风宠物（一只橘猫）的动作帧，输出到 src/common/pet/<状态>_<序号>.png。

在 40x40 的像素网格上画，再按最近邻放大 SCALE 倍，保持像素颗粒感。
状态：work（干活）、alert（等你处理）、celebrate（庆祝）、sleep（睡觉）。
改形象只改这个文件，然后重新跑：.venv/bin/python band-app/tools/gen_pet.py
"""

import os
import random
import sys

from PIL import Image, ImageDraw

GRID = 40
SCALE = 4
OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "src", "common", "pet")

C = {
    "line": (59, 35, 20),
    "fur": (255, 159, 67),
    "stripe": (214, 110, 32),
    "cream": (255, 232, 204),
    "pink": (255, 122, 162),
    "eye": (20, 20, 24),
    "shine": (255, 255, 255),
    "laptop": (120, 128, 140),
    "laptop_dark": (70, 76, 86),
    "screen": (46, 160, 255),
    "screen_dim": (24, 90, 160),
    "sign": (255, 159, 28),
    "sign_text": (255, 255, 255),
    "stick": (150, 100, 60),
    "z": (140, 170, 255),
}
CONFETTI = [(255, 99, 132), (61, 220, 132), (46, 160, 255), (255, 206, 86), (186, 104, 255)]


def px(d, x, y, color):
    d.point((x, y), fill=color)


def rect(d, x0, y0, x1, y1, color):
    d.rectangle((x0, y0, x1, y1), fill=color)


def draw_cat(d, dy=0, eyes="open", arms="down", tail=0, mouth="smile"):
    """坐着的橘猫，身体中心约在 x=20。dy 整体上下偏移。"""
    y = 14 + dy
    # 尾巴（身后右侧，tail 控制摆动）
    for i, (tx, ty) in enumerate([(29, y + 18), (30, y + 17), (31, y + 16), (31 + tail, y + 15), (31 + tail, y + 14)]):
        rect(d, tx, ty, tx + 1, ty + 1, C["stripe"] if i % 2 else C["fur"])
    # 身体
    d.ellipse((11, y + 9, 29, y + 23), fill=C["line"])
    d.ellipse((12, y + 10, 28, y + 22), fill=C["fur"])
    d.ellipse((16, y + 13, 24, y + 22), fill=C["cream"])
    # 耳朵
    for ex, flip in ((12, 1), (28, -1)):
        d.polygon([(ex, y - 6), (ex + 5 * flip, y - 1), (ex, y + 1)], fill=C["line"])
        d.polygon([(ex + flip, y - 4), (ex + 4 * flip, y - 1), (ex + flip, y)], fill=C["fur"])
        px(d, ex + 2 * flip, y - 1, C["pink"])
    # 头
    d.rounded_rectangle((10, y - 3, 30, y + 11), radius=5, fill=C["line"])
    d.rounded_rectangle((11, y - 2, 29, y + 10), radius=4, fill=C["fur"])
    # 额头条纹
    for sx in (18, 20, 22):
        rect(d, sx, y - 2, sx, y, C["stripe"])
    # 口鼻
    d.ellipse((16, y + 5, 24, y + 10), fill=C["cream"])
    rect(d, 19, y + 5, 21, y + 5, C["pink"])
    if mouth == "open":
        rect(d, 19, y + 7, 21, y + 8, C["line"])
    else:
        px(d, 19, y + 7, C["line"])
        px(d, 21, y + 7, C["line"])
        px(d, 20, y + 6, C["line"])
    # 眼睛
    for ex in (14, 24):
        if eyes == "open":
            rect(d, ex, y + 2, ex + 2, y + 4, C["eye"])
            px(d, ex, y + 2, C["shine"])
        elif eyes == "wide":
            rect(d, ex, y + 1, ex + 2, y + 4, C["eye"])
            rect(d, ex, y + 1, ex, y + 2, C["shine"])
        elif eyes == "happy":  # ^ ^
            px(d, ex, y + 4, C["eye"])
            px(d, ex + 1, y + 3, C["eye"])
            px(d, ex + 2, y + 4, C["eye"])
        elif eyes == "closed":
            rect(d, ex, y + 3, ex + 2, y + 3, C["eye"])
        elif eyes == "focus":  # 半睁，盯屏幕
            rect(d, ex, y + 3, ex + 2, y + 4, C["eye"])
    # 腮红
    px(d, 13, y + 6, C["pink"])
    px(d, 27, y + 6, C["pink"])
    # 手
    if arms == "down":
        for ax in (14, 24):
            rect(d, ax, y + 19, ax + 2, y + 21, C["cream"])
    elif arms == "up":
        for ax, ex in ((9, 8), (29, 30)):
            rect(d, ax, y + 9, ax + 2, y + 13, C["fur"])
            rect(d, ex, y + 6, ex + 2, y + 8, C["cream"])
    elif isinstance(arms, tuple):  # 打字：(左手抬高, 右手抬高)
        for ax, raised in zip((14, 24), arms):
            ty = y + 17 - (1 if raised else 0)
            rect(d, ax, ty, ax + 2, ty + 2, C["cream"])


def draw_laptop(d, glow):
    # 笔记本放在猫面前偏下，露出敲键盘的爪子
    rect(d, 11, 33, 29, 34, C["laptop_dark"])
    rect(d, 12, 35, 28, 37, C["laptop"])
    rect(d, 13, 35, 27, 36, C["screen"] if glow else C["screen_dim"])
    rect(d, 9, 38, 31, 39, C["laptop_dark"])


def draw_sign(d, dy, wobble):
    x = 30 + wobble
    y = 2 + dy
    rect(d, x + 2, y + 8, x + 3, y + 20, C["stick"])
    d.rounded_rectangle((x - 2, y, x + 7, y + 9), radius=2, fill=C["line"])
    d.rounded_rectangle((x - 1, y + 1, x + 6, y + 8), radius=1, fill=C["sign"])
    rect(d, x + 2, y + 2, x + 3, y + 5, C["sign_text"])
    rect(d, x + 2, y + 7, x + 3, y + 7, C["sign_text"])


def draw_z(d, x, y, size):
    for i in range(size):
        px(d, x + i, y, C["z"])
        px(d, x + i, y + size - 1, C["z"])
        px(d, x + size - 1 - i, y + i, C["z"])


def frame(draw_fn):
    img = Image.new("RGBA", (GRID, GRID), (0, 0, 0, 0))
    draw_fn(ImageDraw.Draw(img))
    return img.resize((GRID * SCALE, GRID * SCALE), Image.NEAREST)


def work_frames():
    out = []
    for i in range(6):
        def f(d, i=i):
            draw_cat(d, dy=1 + (1 if i in (2, 3) else 0), eyes="focus", arms=(i % 2 == 0, i % 2 == 1), tail=i % 2)
            draw_laptop(d, glow=i % 3 != 2)
        out.append(frame(f))
    return out


def alert_frames():
    hops = [0, -3, -5, -3, 0, 0]
    out = []
    for i, dy in enumerate(hops):
        def f(d, i=i, dy=dy):
            draw_cat(d, dy=dy, eyes="wide", arms="down", tail=1 if i % 2 else -1, mouth="open")
            draw_sign(d, dy, wobble=1 if i % 2 else 0)
        out.append(frame(f))
    return out


def celebrate_frames():
    rnd = random.Random(7)
    out = []
    for i in range(6):
        bits = [(rnd.choice([rnd.randrange(1, 9), rnd.randrange(31, 38)]) if j % 2 else rnd.randrange(2, 37), rnd.randrange(0, 7), rnd.choice(CONFETTI)) for j in range(14)]
        def f(d, i=i, bits=bits):
            draw_cat(d, dy=-2 if i % 2 else 0, eyes="happy", arms="up" if i % 2 == 0 else "down", tail=1, mouth="open")
            for x, y, c in bits:
                fy = y + (i * 3) % 9  # 彩纸往下飘，只在头顶上方和身体两侧
                rect(d, x, fy, x + 1, fy, c)
        out.append(frame(f))
    return out


def sleep_frames():
    out = []
    for i in range(8):
        def f(d, i=i):
            draw_cat(d, dy=1 if i % 4 in (1, 2) else 0, eyes="closed", arms="down", tail=0, mouth="smile")
            # Z 从小到大往上飘
            k = i % 4
            draw_z(d, 30, 12 - k * 2, 3)
            if k >= 2:
                draw_z(d, 33, 6 - (k - 2) * 2, 4)
        out.append(frame(f))
    return out


STATES = {
    "work": work_frames,
    "alert": alert_frames,
    "celebrate": celebrate_frames,
    "sleep": sleep_frames,
}


def main():
    os.makedirs(OUT, exist_ok=True)
    for name in os.listdir(OUT):
        if name.endswith(".png"):
            os.remove(os.path.join(OUT, name))
    sheet_rows = []
    for state, fn in STATES.items():
        frames = fn()
        for i, img in enumerate(frames):
            img.save(os.path.join(OUT, f"{state}_{i}.png"), optimize=True)
        sheet_rows.append(frames)
        print(f"{state}: {len(frames)} 帧")
    # 预览图：每行一个状态，深色背景，方便肉眼检查
    if len(sys.argv) > 1:
        w = max(len(r) for r in sheet_rows) * GRID * SCALE
        sheet = Image.new("RGBA", (w, len(sheet_rows) * GRID * SCALE), (10, 12, 18, 255))
        for r, frames in enumerate(sheet_rows):
            for c, img in enumerate(frames):
                sheet.alpha_composite(img, (c * GRID * SCALE, r * GRID * SCALE))
        sheet.save(sys.argv[1])
        print("预览:", sys.argv[1])


if __name__ == "__main__":
    main()
