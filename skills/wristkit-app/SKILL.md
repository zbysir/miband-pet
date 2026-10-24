---
name: wristkit-app
description: 写或改 wristkit 手环 app（`server/apps/<名字>/` 或用户 app 目录里的 index.ux / server.js / manifest.json）。新建 app、给 app 加界面或功能、修 app 在手环上的显示问题、让 app 读 Mac 上的数据时用。
---

# 写 wristkit app

每个 app 是手环上一个单独的快应用：`index.ux`（快应用原生写法，像 Vue）跑在手环上，可选的 `server.js` 跑在 Mac 上、把数据推给手环。接口、目录结构、限制以 [docs/apps.md](../../docs/apps.md) 为准，先把它通读一遍再动手；这份只补写 app 的流程和手环上踩出来的写法。手环实测数据在 [docs/render-spike.md](../../docs/render-spike.md)。

## 流程

1. **定落点。** 改的是哪个 app：
   - 内置 app 在 `server/apps/<名字>/`，打包进 App；用户自己的 app 在 `~/Library/Application Support/miband-pet/apps/<名字>/`。用户没说要内置，新 app 放用户 app 目录。
   - 能在 app 里做完的都在 app 里做。要动本体（`server/internal/`、`server/app/`、`server/cmd/`）时先停下，按 AGENTS.md 说清楚改哪里、为什么 app 里做不到，等用户同意。
2. **写。** 照下面「写法」和 docs/apps.md 写。每个文件开头一段注释说清这个 app / 文件干什么，跟 `server/apps/agent-cat/` 一样。
3. **自检**，全部通过才算写完：
   - 编译过：在 `server/` 下 `go run ./cmd/miband pack <app 目录> /tmp/x.rpk`，没报错。模板写错、表达式写错、import 了不支持的东西都会在这里报出来。
   - 改了内置 app：在 `server/` 下跑 `go test ./internal/bandapp/`，它在模拟的手环里跑编好的安装包（`TestAgentCat*` 是例子），改了显示或数据逻辑就照着补测试。
   - 一页元素几十个以内；界面文字、manifest 的 `title`、`settings` 的标题和说明都是用户看得懂的话，不出现 server.js、store、publish 这类词。
4. **上手环。**
   - 用户 app：保存后 App 左边的预览自动更新；装到手环要用户在 App 里点「装到手环」/「更新」（约 10 秒，会断开手环上正开着的这个 app），先问用户。
   - 内置 app：要重新构建 App（`make app`）、替换 `/Applications/wristkit.app` 再重启 App（App 会拦下正常退出，要直接结束进程，这会断开手环，先问用户），然后在 App 里更新这个 app。
5. **交代验证。** 你看不到手环。告诉用户去 App 的预览或手环上看哪个界面、点哪里，偶发问题要说清怎么让它出现。

## 写法

手环上新建元素慢（10 Pro 约 10ms 一个，9 Pro 约 30ms），改已有元素的文字和颜色很快；手环的 JS 比 Mac 慢上百倍。写法都围着这两点：

- **排版写在 class 里。** 宽高、边距、方向这些写在 `<style>` 的 class 里，只有会变的值（颜色、进度条宽度）写进 `style="…{{}}…"`。
- **结构稳定。** 同一个位置有内容和没内容，优先画同一个元素、只换文字和颜色；`if` 包的东西小一点。列表用 `for` 加 `tid`，长列表分页。
- **数据从 Mac 来就写 server.js。** 读文件、跑命令、重计算都在 `server.js` 里，处理成界面直接能用的小对象再 `publish`；页面的 `onStore` 只把它放进数据。读不到的数据（用户没装某工具、没开某功能）就不推，界面上那一块隐藏，不报错。
- **接入用户环境要能关、能还原。** 改用户设置、读本机文件这类做成 `settings` 里的开关；按所有用户的环境设计，不按本机。
- **逐帧动画用 `image-animator`**（见 Agent 猫的 `startAnim`）：`duration` 是每帧停多久；换了帧列表等约 300ms 再 `start()`；`onHide` 时 `stop()`。每帧停两三百毫秒以上。
- **屏幕按 212 宽设计**，高度用 `@system.device` 的 `getInfo` 算（Agent 猫的 `pageH`）。会变长的文字给定宽度和 `lines: 1; text-overflow: ellipsis`。
