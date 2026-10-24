---
name: wristkit-plugin
description: 写或改 wristkit 手环插件（`server/plugins/<名字>/` 或用户插件目录里的 index.jsx / server.js / manifest.json）。新建插件、给插件加界面或功能、修插件在手环上的显示问题、让插件读 Mac 上的数据时用。
---

# 写 wristkit 插件

插件的接口、组件、权限、目录结构以 [docs/plugins.md](../../docs/plugins.md) 为准，先把它通读一遍再动手；这份只补写插件的流程和手环上踩出来的写法。手环实测数据和原因在 [docs/render-spike.md](../../docs/render-spike.md)，碰到性能或排版怪问题时去查。

## 流程

1. **定落点。** 改的是哪个插件、哪一层：
   - 内置插件在 `server/plugins/<名字>/`，打包进 App；用户自己的插件在 `~/Library/Application Support/miband-pet/plugins/<名字>/`。用户没说要内置，新插件放用户插件目录。
   - 能在插件里做完的都在插件里做。要动本体（`render-app/`、`server/internal/`、`server/app/`）时先停下，按 AGENTS.md 说清楚改哪里、为什么插件层做不到，等用户同意。
2. **写。** 照下面「写法」和 docs/plugins.md 的「限制」写。每个文件开头一段注释说清这个插件 / 文件干什么，跟 `server/plugins/agent-cat/` 一样。
3. **自检**，全部通过才算写完：
   - 编译过：`server/app/frontend/node_modules/.bin/esbuild <插件目录>/index.jsx --bundle --external:miband --external:miband/server --jsx-factory=__h --jsx-fragment=__Fragment --target=es2020 --loader:.js=jsx --outfile=/dev/null`（有 `server.js` 的同样再跑一遍）。没有这个 esbuild 就用 `npx esbuild`。
   - 改了内置插件：在 `server/` 下跑 `go test ./internal/plugin/`，它用手环上同一份运行时跑内置插件。改了显示或数据逻辑，给 `host_test.go` 补对应的测试（照 `TestAgentCat*` 写）。
   - View 嵌套不超过 6 层（根算第 1 层），一屏节点几十个以内。
   - 界面文字、manifest 的 `title`、`settings` 的标题和说明都是用户看得懂的话，不出现 server.js、store、publish 这类词。
4. **上手环。**
   - 用户插件：保存后 App 2 秒内自动重新加载，手环自己拿到新代码，什么都不用做。
   - 内置插件：要重新构建 App（`make app`），替换 `/Applications/wristkit.app` 再重启 App。App 会拦下正常退出，要直接结束进程。这一步会断开手环，先问用户。
   - 只有本体的 `render-app/` 改了才要在 App 里点「更新」重装手环上的渲染器。
5. **交代验证。** 你看不到手环，也看不到 App 里的预览。告诉用户去「插件预览」或手环上看哪个界面、点哪里，偶发问题要说清怎么让它出现。

## 写法

手环上 JS 比 Mac 慢上百倍，新建节点约 10ms 一个（9 Pro 上约 30ms），只改文字和颜色很快。写法都围着这两点：

- **结构稳定。** 同一个位置有内容和没内容时画成同一个元素：没内容时用背景色画空框、不放子元素，而不是换成另一个样式完全不同的占位块。会变的样式每次都写同一组属性、只换值，比如不选中时边框用暗色，而不是不写 `borderColor`。9 Pro 上排版属性变了或某一项去掉了，元素要整个重建。
- **常变的状态放进小组件。** 只读这个状态的那一块单独成组件，`useState` / `useStore` 放在它里面，变了只重算这一块。例子：Agent 猫的 `Bubble`（点了换下一个）和 `Tally`（钱数几十秒变一次）。
- **固定尺寸。** 屏幕按 212 宽设计，高度用 `useScreen().height`。会变长的文字给定宽度和 `lines: 1, textOverflow: 'ellipsis'`，没数据时也留出同样大小的位置，布局不跳。
- **数据从 Mac 来就写 server.js。** 读文件、跑命令都在 `server.js` 里，处理成界面直接能用的小对象再 `publish`；手环那边只管画。读不到的数据（用户没装某工具、没开某功能）就不推，界面上那一块隐藏，不报错。
- **接入用户环境要能关、能还原。** 改用户设置、读本机文件这类做成 `settings` 里的开关；按所有用户的环境设计，不按本机。
- **动画用 `Sprite`。** 每帧停两三百毫秒以上，别用定时器一帧帧改界面。
- **打开慢就晚点画。** 第二屏这类一开始看不到的东西，等首屏出来一会儿再画（见 Agent 猫的 `todayReady`）。
