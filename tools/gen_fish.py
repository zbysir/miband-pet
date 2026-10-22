"""小鱼干图标：和猫一样逐像素手绘成字符格子图，按最近邻放大 SCALE 倍。
输出到用到它的插件：server/plugins/agent-cat/assets/fish.png、server/plugins/today/assets/fish.png。

    .venv/bin/python tools/gen_fish.py
"""

import os

from PIL import Image

SCALE = 2

# 头朝左的小鱼干：K 轮廓  F 鱼身  D 鱼身上的烤痕  L 肚子  E 眼睛  W 眼睛高光  . 透明
FISH = [
    "....KKKKKK...KK.",
    "..KKFFFFFFKK.KFK",
    ".KWEFFDFFDFFKKFK",
    "KFEEFFFFFFFFFFK.",
    "KFFFFDFFDFFFFFK.",
    ".KLLLLLLLLLLKKFK",
    "..KKLLLLLLKK.KFK",
    "....KKKKKK...KK.",
]

COLORS = {
    "K": (74, 42, 20, 255),
    "F": (226, 150, 64, 255),
    "D": (176, 98, 38, 255),
    "L": (246, 206, 138, 255),
    "E": (24, 20, 18, 255),
    "W": (255, 255, 255, 255),
}

HERE = os.path.dirname(os.path.abspath(__file__))
OUTS = [os.path.join(HERE, "..", "server", "plugins", name, "assets", "fish.png") for name in ("agent-cat", "today")]


def main():
    w, h = len(FISH[0]), len(FISH)
    img = Image.new("RGBA", (w, h), (0, 0, 0, 0))
    for y, row in enumerate(FISH):
        assert len(row) == w, f"第 {y} 行长度不对"
        for x, ch in enumerate(row):
            if ch != ".":
                img.putpixel((x, y), COLORS[ch])
    img = img.resize((w * SCALE, h * SCALE), Image.NEAREST)
    for out in OUTS:
        os.makedirs(os.path.dirname(out), exist_ok=True)
        img.save(out, optimize=True)
        print("已生成", os.path.normpath(out))


if __name__ == "__main__":
    main()
