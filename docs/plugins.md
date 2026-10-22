# 写插件

手环上显示的界面都是插件：Agent 猫、Agents 列表是内置插件，你也可以自己写。手环上一次显示一个插件，在 App 的「插件」卡片里点「切换」换。插件用 JSX 写，写法和 React 一样。

插件**在手环上跑**：Mac 把插件编译成 JS，连同图片推到手环上的渲染器（`render-app`）；手环缓存下来，在本地执行。点击、`useState`、翻页都在手环上处理，不经过 Mac，反应很快；Mac 断开时插件照样能点，只是数据（agents 等）不再更新。不用重装手环上的应用，保存文件就生效。

内置插件的源码在 `server/plugins/` 下，可以当例子看。

## 放在哪

```
~/Library/Application Support/miband-pet/plugins/
  my-plugin/
    manifest.json
    index.jsx
    assets/        # 可选，png 图片
```

App 里「插件」卡片上有「打开插件目录」。插件目录里的文件一改，2 秒内自动重新加载。和内置插件同名（`name` 相同）的用户插件会替换内置插件。

`manifest.json`：

```json
{
  "name": "my-plugin",
  "title": "我的插件",
  "order": 100
}
```

- `name`：唯一名字。
- `title`：显示名，出错页和 App 里会用到。
- `order`：在插件列表里的顺序，小的在前；排第一的是提醒时切回的那个。Agent 猫是 0，Agents 是 10，不写默认 100。
- `main`：入口文件，默认 `index.jsx`。

## 最小的插件

```jsx
import { Col, Row, Text, Spacer, useState, useScreen } from 'miband'

export default function Counter() {
  const [n, setN] = useState(0)
  const screen = useScreen()
  return (
    <Col gap={8} padding={12} style={{ width: 212, height: screen.height, backgroundColor: '#000000' }}>
      <Row align="center">
        <Text style={{ fontSize: 18, fontWeight: 'bold' }}>计数</Text>
        <Spacer />
        <Text style={{ color: '#8a8f98' }}>{n}</Text>
      </Row>
      <Text onClick={() => setN(n + 1)} style={{ padding: [8, 12], borderRadius: 10, backgroundColor: '#2ea0ff' }}>
        点我 +1
      </Text>
    </Col>
  )
}
```

插件 `export default` 一个组件函数。写法和 React 一样：函数组件 + hooks，`key` 用法也一样。只能 `import 'miband'` 和插件目录里的文件（`./xxx.jsx`、`./data.json`），不能用 npm 包。

## 组件

| 组件 | 说明 |
|---|---|
| `View` | 容器（手环上的 `div`）。只支持 flex 布局，默认横排（`flexDirection: 'row'`） |
| `Row` / `Col` | 横排 / 竖排的 `View`，多几个简写属性：`gap`、`justify`（`start` `end` `center` `between` `around`）、`align`（`start` `end` `center` `stretch`）、`padding` |
| `Spacer` | 占满剩下的空间，把后面的东西顶到另一头 |
| `Text` | 文字。默认白色 14px。单行省略：`{ width: 120, lines: 1, textOverflow: 'ellipsis' }` |
| `Image` | 图片：`<Image src={asset('assets/icon.png')} style={{ width: 24, height: 24 }} />` |
| `Sprite` | 逐帧动画，在手环本地循环播放：`<Sprite frames={[asset('a.png'), asset('b.png')]} duration={800} style={{ width: 80, height: 80 }} />`。换动画时给它换个 `key`，动画才会从头播 |

字符串直接写在 `View` 里会自动包成 `Text`。

### 样式

`style` 可以是对象（驼峰写法，数字自动加 `px`）、CSS 字符串，或者它们的数组（后面的覆盖前面的）：

```jsx
<View style={[{ flexDirection: 'column', padding: [4, 8] }, active && { backgroundColor: '#16191f' }]} />
```

能用的就是手环快应用支持的 CSS：`width` `height`（px 或百分比）、`padding` `margin`、`flex` 系列、`justify-content` `align-items`、`background-color`、`border-*`、`border-radius`、`opacity`、`position: absolute` + `top/left`，文字的 `color` `font-size` `font-weight` `lines` `text-overflow` `text-align` `line-height`。不支持 `gap`（`Row`/`Col` 的 `gap` 会换成子元素的 margin）。

手环屏幕按 212 宽来设计，高度用 `useScreen().height`（手环 10 Pro 是 303）。

### 事件

- `onClick`、`onLongPress`：任何组件都能加。里外两层都能点时只触发最里面的。
- 滑动：右滑是退出应用；左、上、下滑交给插件，用 `useSwipe(dir => …)` 接（`dir` 是 `left` / `up` / `down`）。

## Hooks 和 API

都从 `'miband'` 导入。

| 名字 | 说明 |
|---|---|
| `useState` `useEffect` `useRef` `useMemo` `useCallback` | 和 React 一样 |
| `useInterval(fn, ms)` | 定时执行，`ms` 为 `null` 时停 |
| `useStore(name)` | 读 Mac 推进来的数据，数据变了自动重新渲染（见下表） |
| `useSwipe(fn)` | 左滑、上下滑 |
| `useScreen()` | `{ width, height }` |
| `asset(path)` | 插件目录里的 png（路径相对插件目录，如 `assets/a.png`）。图片按内容哈希推到手环，推过就不再推 |
| `vibrate('short' \| 'long')` | 手环震动（只在这一页正显示时有效） |
| `call(name, ...args)` | 让 Mac 做事，比如 `call('feed')` 喂一条小鱼干。不等结果：Mac 做完会更新对应的数据（`useStore`）。现在有的：`feed` |
| `storage.get(key)` / `storage.set(key, value)` | 插件自己的持久化数据，存在手环上（离开应用时写入） |
| `setTimeout` `setInterval` `console.log` | 照常用；`console` 的输出会发回 Mac，在 App 的日志里 |

`useStore` 读的是 Mac 推过来的数据（插件本身在手环上，读不到 Mac 上的文件和命令）：

| 名字 | 内容 |
|---|---|
| `agents` | herdr 的 agent 列表：`[{ id, agent, project, title, status, since }]`，`status` 是 `blocked` / `working` / `done` / `idle` / `unknown`；`since` 是进入当前状态的时间（毫秒，Mac 开始观察之前就是这个状态的为 0） |
| `settings` | `{ pet, impatientMin }`：App 里选的猫（`orange` / `gray` / `white`），卡住超过几分钟算太久（0 表示不管） |
| `stats` | 小鱼干：`{ today: { done, fed }, fishLeft, fedTotal, days: [{ date, weekday, done }] }`。agent 每跑完一个任务（working → done/idle）得一条，Mac 记账，`days` 是最近 7 天 |
| `usage` | 额度：`[{ agent, windows: [{ name, used, resetsAt }], updated }]`。`agent` 是 `claude` / `codex`，`name` 如 `5h` / `7d`，`used` 是百分比，`resetsAt` 是 Unix 秒。读不到的不出现（Claude Code 要在 App 里打开「显示 Claude Code 额度」，只有 Pro/Max 有） |
| `screen` | 同 `useScreen()` |

## 限制

这些限制来自手环，实测数据见 [render-spike.md](render-spike.md)：

- **View 最多嵌套 6 层**（根算第 1 层）。超过会显示出错页。
- **新建节点很慢，约 10ms 一个**；结构不变、只改文字和样式很快。所以一页控制在几十个节点以内，长列表分页（参考 `server/plugins/agents`），不要每次渲染都换 `key`。
- **手环的 JS 比 Mac 慢上百倍**（10 万次循环约 0.3 秒）。渲染函数里别做重计算。
- 支持到 ES2020 的语法；只能 `import` `miband` 和插件目录里的文件。
- 没有滚动容器。动画用 `Sprite` 在手环本地播，不要用定时器一帧帧改界面。

## 出错了怎么办

编译失败、在手环上运行时抛异常，手环上这一页会显示「<title> 出错了」和错误信息，App 的「插件」卡片里也能看到（手环把错误发回 Mac）。改好保存后自动重新编译，手环拿到新代码就换上。

## 原理

- Mac（`server/internal/plugin`）：用 esbuild 把插件打成一个 JS 文件，按内容哈希登记代码和图片；手环打开时发插件列表（名字、代码哈希、图片哈希）和数据，手环缺什么再要什么。App 里切换插件时发一条 `show`。
- 手环（`render-app`）：`src/common/runtime.js` 是 hooks 运行时，把插件渲染出的元素树转成节点树；页面模板（`tools/gen_template.py` 生成）照着节点树画。插件代码用 `new Function` 加载。
- Mac 上的测试（`server/internal/plugin/host_test.go`）用 goja 加载同一份 `runtime.js` 跑内置插件，不用真手环。
