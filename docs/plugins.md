# 写插件

手环上显示的界面都是插件：Agent 猫是内置插件，你也可以自己写。手环上一次显示一个插件，在 App 的「插件」卡片里点「切换」换。插件用 JSX 写，写法和 React 一样。

插件**在手环上跑**：Mac 把插件编译成 JS，连同图片推到手环上的渲染器（`render-app`）；手环缓存下来，在本地执行。点击、`useState`、翻页都在手环上处理，不经过 Mac，反应很快；Mac 断开时插件照样能点，只是 Mac 推来的数据不再更新。不用重装手环上的应用，保存文件就生效。

内置插件的源码在 `server/plugins/` 下，可以当例子看。

插件只管手环上的一页，每个插件都有界面（没有只在后台跑的插件）。插件之间不共用数据：想要别的插件的功能，把它的代码复制一份过来。

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
- `order`：在插件列表里的顺序，小的在前。Agent 猫是 0，不写默认 100。
- `permissions`：`server.js` 要用的能力，见下面「电脑上跑的部分：server.js」。
- `settings`：插件的配置项，见下面「配置」。
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
| `Pager` | 翻页：每个子元素是一页，手环上是原生的 swiper，翻页跟着手指走。`<Pager vertical index={page} onChange={setPage}>`，`vertical` 默认是纵向；不传 `index` 时它自己记住翻到哪页。**只能是插件界面的最外层**，每一页要和屏幕一样大（`useScreen()`）。例子见 Agent 猫插件 |
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
| `useConfig()` | 插件的配置（见下面「配置」），用户在 App 里改了会重新渲染 |
| `asset(path)` | 插件目录里的 png（路径相对插件目录，如 `assets/a.png`）。图片按内容哈希推到手环，推过就不再推 |
| `vibrate('short' \| 'long')` | 手环震动（只在这一页正显示时有效） |
| `call(name, ...args)` | 让本插件的 `server.js` 做事（交给它 `on(name, fn)` 注册的函数）。不等结果：`server.js` 做完 `publish` 新数据，`useStore` 读到 |
| `storage.get(key)` / `storage.set(key, value)` | 插件自己的持久化数据，存在手环上（离开应用时写入） |
| `setTimeout` `setInterval` `console.log` | 照常用；`console` 的输出会发回 Mac，在 App 的日志里 |

`useStore(name)` 读的是本插件 `server.js` 里 `publish(name, …)` 推的数据（插件本身在手环上，读不到 Mac 上的文件和命令，要读就写 `server.js`）。读不到别的插件推的。另外有两项是基座给所有插件的：

| 名字 | 内容 |
|---|---|
| `screen` | 同 `useScreen()` |
| `band` | `{ connected, foreground }`：手环连着没有，手环上的 wristkit 开在前台没有 |

## 配置

插件在 `manifest.json` 的 `settings` 里声明配置项，App 的「插件」卡片照着画出设置表单；用户改了，值推给手环（`useConfig()`）和 server.js（`getConfig()`），存在 App 的配置文件里。

```json
"settings": [
  {
    "key": "quota",
    "title": "首页显示哪个额度",
    "description": "猫的标题下面那根进度条",
    "type": "select",
    "options": [{ "value": "auto", "label": "用得最多的" }, { "value": "codex", "label": "Codex" }],
    "default": "auto"
  }
]
```

`type` 可以是 `select`（要有 `options`）、`bool`、`number`、`text`。`useConfig()` 拿到的是 `{ quota: 'auto' }` 这样的对象，没改过的项是 `default`。

想在手环上直接切换的东西（比如点一下换个显示方式），用 `useState`、`storage` 自己做就行，不用声明成配置。

## 电脑上跑的部分：server.js

插件的界面在手环上跑，读不到 Mac 上的文件、也不能执行命令。要这些就在插件目录里再放一个 `server.js`，它在 Mac 上跑（App 里），和手环上的界面配合。手环上的界面像网页前端，`server.js` 像它的后端；它不是网络服务，不开端口：

```js
// server.js
import { on, publish, notifyMac } from 'miband/server'

on('hello', (who) => {
  notifyMac('手环上点了一下', `来自 ${who}`) // Mac 右上角弹通知
  publish('lastClick', Date.now()) // 手环上 useStore('lastClick') 读到
})
```

```jsx
// index.jsx
import { Col, Text, useStore, call } from 'miband'

export default function Hello() {
  const last = useStore('lastClick')
  return (
    <Col padding={16}>
      <Text onClick={() => call('hello', '手环')}>点我通知 Mac</Text>
      <Text style={{ fontSize: 11, color: '#8a8f98' }}>{last ? new Date(last).toLocaleTimeString() : '还没点过'}</Text>
    </Col>
  )
}
```

```json
// manifest.json
{ "name": "hello", "title": "你好", "permissions": { "notify": true } }
```

`miband/server` 里有：

| 名字 | 说明 |
|---|---|
| `on(name, fn)` | 手环上 `call(name, ...args)` 时调用 `fn(...args)` |
| `publish(name, value)` | 推数据给手环，本插件里 `useStore(name)` 读到 |
| `getStore(name)` / `onStore(name, fn)` | 读本插件 `publish` 过的数据和 `band` / 数据变了时调用 |
| `getConfig()` / `onConfig(fn)` | 插件的配置 / 用户改了配置时调用 |
| `storage.get(key)` / `storage.set(key, value)` | Mac 这边的持久化数据（`~/Library/Application Support/miband-pet/plugin-data/<name>.json`） |
| `notifyMac(title, body)` / `notifyBand(title, body)` | Mac 弹通知 / 手环发通知（震动）。要 `notify` 权限 |
| `wakeBand()` | 把手环点亮并显示本插件（打开手环上的 wristkit，不震动）。要 `notify` 权限；手环没连上时抛异常 |
| `readFile(path, { tail })` / `readDir(path)` / `stat(path)` | 读文件（`tail` 只读最后多少字节，适合大日志）。只能读 `read` 权限里列的目录 |
| `exec(cmd, args)` | 执行命令，返回 `{ code, stdout, stderr }`，15 秒超时。只能执行 `exec` 权限里列的命令；找不到命令时抛异常（从访达打开的 App 的 PATH 很短，所以还会找 `~/.local/bin`、`/opt/homebrew/bin`、`/usr/local/bin` 这些地方） |
| `home` | 用户目录 |
| `setTimeout` `setInterval` `console.log` | 照常用，`console` 输出在 App 的日志里 |

App 有时会让插件马上刷新数据（比如打开「显示 Claude Code 额度」之后），会调用所有插件 server.js 里 `on('refresh', fn)` 注册的函数；读外部数据的插件注册一下。

**权限**：`manifest.json` 的 `permissions` 里声明 `notify`（发通知、点亮手环）、`read`（能读的目录，可以用 `~`）、`exec`（能执行的命令名，如 `["herdr"]`）。用户插件要了权限的，要在 App 的「插件」里点「允许」，`server.js` 才会跑；权限改了要重新允许。没要权限的 `server.js` 直接跑。内置插件不用问。

例子：Agent 猫插件的 `server/plugins/agent-cat/herdr.js` 每 3 秒跑一次 `herdr agent list`，推 agent 状态、记每天完成了几个、跑完和卡住时提醒；`server.js` 读 Claude Code 和 Codex 的额度，`publish('usage', …)` 给手环。

## 在 Mac 上预览和调试

App 里的「插件预览」用和手环上同一份运行时、同一份编译好的代码和数据，把界面画在窗口里：点一下、右键当长按、触控板上下滑当上下滑，`call()` 会真的交给 `server.js` 处理。改了插件文件，预览自动刷新。不用每次都看手环。

预览里的布局按手环的规则来（只有 flex，div 默认横排），但毕竟是浏览器画的，字体、抗锯齿和手环上会有点差别，最后还是在手环上看一眼。

## 限制

这些限制来自手环，实测数据见 [render-spike.md](render-spike.md)：

- **server.js 跑在 Mac 上，界面跑在手环上**，两边只能通过 `call` 和 `publish` 传数据（经蓝牙，几十毫秒）。
- **View 最多嵌套 6 层**（根算第 1 层）。超过会显示出错页。
- **新建节点很慢，约 10ms 一个**；结构不变、只改文字和样式很快。所以一页控制在几十个节点以内，长列表分页（`useSwipe` 翻页），不要每次渲染都换 `key`。
- **手环的 JS 比 Mac 慢上百倍**（10 万次循环约 0.3 秒）。渲染函数里别做重计算。
- 支持到 ES2020 的语法；只能 `import` `miband` 和插件目录里的文件。
- 没有滚动容器。动画用 `Sprite` 在手环本地播，不要用定时器一帧帧改界面。

## 出错了怎么办

编译失败、在手环上运行时抛异常，手环上这一页会显示「<title> 出错了」和错误信息，App 的「插件」卡片里也能看到（手环把错误发回 Mac）。改好保存后自动重新编译，手环拿到新代码就换上。

## 原理

- Mac（`server/internal/plugin`）：用 esbuild 把插件打成一个 JS 文件，按内容哈希登记代码和图片；手环打开时发插件列表（名字、代码哈希、图片哈希）和数据，手环缺什么再要什么。App 里切换插件时发一条 `show`。
- 手环（`render-app`）：`src/common/runtime.js` 是 hooks 运行时，把插件渲染出的元素树转成节点树；页面模板（`tools/gen_template.py` 生成）照着节点树画。插件代码用 `new Function` 加载。
- Mac 上的测试（`server/internal/plugin/host_test.go`）用 goja 加载同一份 `runtime.js` 跑内置插件，不用真手环。
