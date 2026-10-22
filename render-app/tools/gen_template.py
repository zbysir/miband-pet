"""生成 src/pages/index/index.ux 里的 <template>：把 Mac 发来的界面树逐层画出来。

快应用的模板是编译时定死的，不能递归（递归组件能用，但自定义组件在手环上内存开销大、也更慢，
见 docs/render-spike.md），所以在页面模板里把每一层都写出来，最多 DEPTH 层 view 嵌套。

    python3 render-app/tools/gen_template.py

节点字段（插件运行时 src/common/runtime.js 生成）：
    t  类型：v=view（div）  x=text  m=image  a=sprite（image-animator）
       w=pager（swiper，子节点是一页一页）。只能是根节点：每层都多一种容器的话模板会成倍变大
    i  节点 id，同时用作元素 id 和事件回传
    s  整串 style
    c  子节点（只有 view 有）
    x  文字    u  图片地址    f  动画帧 [{src}]    d  动画时长
    e  事件位：1=click  2=longpress  4=change（pager 翻页）
    vt pager 是不是纵向    ix pager 当前第几页
"""

import os
import re

DEPTH = 6  # view 最多嵌套几层（含根节点）

UX = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "src", "pages", "index", "index.ux")


def events(n):
    return f'@click="tap({n}.i, {n}.e)" @longpress="hold({n}.i, {n}.e)"'


def leaves(n, ind):
    """非 view 的节点：text / image / sprite，用 elif 接在 view 后面"""
    return [
        f'{ind}<text elif="{{{{{n}.t === \'x\'}}}}" id="{{{{{n}.i}}}}" style="{{{{{n}.s}}}}" {events(n)}>{{{{{n}.x}}}}</text>',
        f'{ind}<image elif="{{{{{n}.t === \'m\'}}}}" id="{{{{{n}.i}}}}" style="{{{{{n}.s}}}}" src="{{{{{n}.u}}}}" {events(n)}></image>',
        f'{ind}<image-animator elif="{{{{{n}.t === \'a\'}}}}" id="{{{{{n}.i}}}}" style="{{{{{n}.s}}}}" images="{{{{{n}.f}}}}" duration="{{{{{n}.d}}}}"></image-animator>',
    ]


def swiper_attrs(n):
    # 原生 swiper：翻页跟着手指走；不循环、不显示自带的小圆点。
    # index 不绑定：绑定的话翻页 → 插件更新页码 → 写回 index → swiper 再翻一次，会来回跳。
    # 插件要换页时由页面调 swipeTo（见 index.ux 的 syncPager）
    return (f'id="{{{{{n}.i}}}}" style="{{{{{n}.s}}}}" vertical="{{{{{n}.vt}}}}" '
            f'loop="false" indicator="false" @change="pageChange({n}.i, {n}.e)"')


def level(parent, depth, ind):
    """parent 的子节点；depth 是子节点所在的层（根是第 0 层）"""
    n = f"n{depth}"
    # tid：按节点 id 复用元素；id 变了（比如猫换了动作）就新建元素，动画从头播
    out = [f'{ind}<block for="{{{{{n} in {parent}.c}}}}" tid="i">']
    inner = ind + "  "
    if depth < DEPTH - 1:
        out.append(f'{inner}<div if="{{{{{n}.t === \'v\'}}}}" id="{{{{{n}.i}}}}" style="{{{{{n}.s}}}}" {events(n)}>')
        out += level(n, depth + 1, inner + "  ")
        out.append(f"{inner}</div>")
    else:
        # 最深一层的 view 不再往下画子节点
        out.append(f'{inner}<div if="{{{{{n}.t === \'v\'}}}}" id="{{{{{n}.i}}}}" style="{{{{{n}.s}}}}" {events(n)}></div>')
    out += leaves(n, inner)
    out.append(f"{ind}</block>")
    return out


def template():
    lines = [
        "<template>",
        "  <!-- tools/gen_template.py 生成，不要手改 -->",
        '  <div class="page" style="height: {{pageH}}px" @swipe="onSwipe">',
        f'    <div if="{{{{root && root.t !== \'w\'}}}}" id="{{{{root.i}}}}" style="{{{{root.s}}}}" {events("root")}>',
    ]
    lines += level("root", 1, "      ")
    lines += ["    </div>", f'    <swiper elif="{{{{root}}}}" {swiper_attrs("root")}>']
    lines += level("root", 1, "      ")
    lines += [
        "    </swiper>",
        '    <text if="{{!root}}" class="empty">{{connected ? \'加载中…\' : \'等待 Mac 连接…\'}}</text>',
        '    <text if="{{root && !connected}}" class="offline">未连接 Mac</text>',
        "  </div>",
        "</template>",
    ]
    return "\n".join(lines) + "\n"


def main():
    with open(UX) as f:
        src = f.read()
    src, n = re.subn(r"<template>.*?</template>\n", lambda _: template(), src, count=1, flags=re.S)
    if n != 1:
        raise SystemExit("index.ux 里没找到 <template>")
    with open(UX, "w") as f:
        f.write(src)
    print("已生成", UX)


if __name__ == "__main__":
    main()
