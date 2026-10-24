# wristkit

<p align="center"><img src="docs/pet.gif" alt="Agent 猫的四种状态：有 agent 卡住时举牌，刚跑完一个时庆祝，有 agent 在跑时敲笔记本，都空闲时睡觉" width="760"></p>

小米手环上的插件基座。Mac 通过蓝牙直连手环（不经过手机），手环上只装一个应用，上面显示的都是插件：插件用 JSX 写，保存就推到手环上，不用重装手环上的应用。

自带的插件把 AI agent（[herdr](https://herdr.dev) 管理的 Claude Code、Codex 等）的状态放到手腕上：

- **Agent 猫**：一只像素猫代表所有 agent 的整体状态。有 agent 卡住了它举牌，卡太久就着急；跑完一个就庆祝，有在跑的就敲笔记本，都空闲时睡觉。标题下面是 Claude Code / Codex 的额度，右下角的小鱼干是今天完成了几个任务。往上翻是「今日」：最近 7 天的完成数和全部额度。
- **提醒**：agent 跑完、卡住时手环发通知震一下，或者只亮屏；有 agent 等你回答时还可以让手环一直亮着。

<p align="center"><img src="docs/skins.png" alt="猫有三种：橘猫、灰猫（中华田园猫）、白猫（美短）" width="600"></p>

> 非小米官方项目。用的是逆向出来的非官方协议（移植自 Gadgetbridge、OronBox），风险自负。许可证 AGPL-3.0，致谢见 [NOTICE.md](NOTICE.md)。

## 需要什么

- 一台 Mac。
- 小米手环。实测可用的是**小米手环 10 Pro**、**小米手环 9 Pro**；其它跑 Xiaomi Vela 的手环 / 手表理论上可以，没试过。
- 第一次绑定时借用一下手机（小米运动健康），之后就不用了。
- 想看 agent 状态的话，装 [herdr](https://herdr.dev)。

## 开始使用

到 [Releases](https://github.com/zbysir/wristkit/releases) 下载最新的 `wristkit-*-mac.zip`，解压后把 `wristkit.app` 拖进「应用程序」。Intel 和 Apple 芯片的 Mac 都能用。

App 没有经过 Apple 公证（要付费的开发者账号），第一次打开时系统会拦下，提示无法验证开发者：

1. 先双击打开一次，在弹窗里点「完成」。
2. 打开「系统设置 → 隐私与安全性」，拉到「安全性」那里，会看到 wristkit 被阻止的提示，点旁边的「仍要打开」，再输入密码确认。

之后就能正常打开。每个版本都用同一张证书签名，更新后不用重新允许蓝牙。也可以自己构建，见下面「开发」。

第一次打开会进入**连接向导**：

1. 借用手机，用小米运动健康绑定一次手环（手环的密钥是绑定时生成的，之后就不用手机了）。
2. 在 App 里扫码登录同一个小米账号，App 自动拿到手环的密钥和地址。
3. 在手机的**系统蓝牙**里对手环「取消配对」（不要在小米运动健康里解绑，解绑后密钥会作废），再在手环上进入「连接新手机」停在二维码界面。App 会自动连上手环，两边弹出配对确认时点允许。
4. 一键把「wristkit」装到手环上。

之后打开手环上的「wristkit」就能看到猫。换手环或手环重新绑定过，在 App 里点「换手环 / 重新绑定」重走一遍。

## 日常使用

- **App 窗口**：左边是手环预览，和手环上跑的是同一份插件、同一份数据，可以直接点、拖动翻页（右键当长按）；「在手环上显示」把手环切到这个插件，「亮屏」把手环点亮。右边是设置：当前插件的设置、菜单栏的猫、插件列表。
- **菜单栏**：关掉窗口后 App 留在菜单栏继续工作，从菜单栏「退出」才会真正退出。菜单栏的猫是 App 自带的，只是个图标，和手环上的猫分开选。
- **提醒**：在 Agent 猫的设置里。agent 跑完、卡住（可选开始跑）时发通知（手环震动）或者只亮屏（把手环点亮、切到猫，不震动）；还可以设「卡住几分钟算太久」（猫着急，再提醒一次）和「有 agent 等你回答时保持亮屏」。
- **Claude Code 额度**：Codex 的额度直接从它的会话日志读，不用设置。Claude Code 只把额度告诉状态栏命令，要在 App 里打开「显示 Claude Code 额度」：App 会把 Claude Code 的状态栏换成一个小脚本，先存一份数据（按会话各存一份，Agent 猫用来算每天花了多少钱）再交给你原来的状态栏命令，状态栏显示不变；关掉会原样还原，改之前会备份 `~/.claude/settings.json`。只有 Pro / Max 订阅才有额度。
- **数据放在哪**：`~/Library/Application Support/miband-pet/`。`config.json` 和 `session.json` 是配置和小米账号登录态（权限 600），`plugin-data/` 是插件在 Mac 上存的数据（比如 Agent 猫记的每天完成了几个任务），`plugins/` 是你自己的插件。

> 这个项目以前叫 miband pet。为了不影响已经在用的人，数据目录（`miband-pet`）、App 标识（`dev.bysir.miband-pet`）、手环上应用的包名（`dev.bysir.mibandpet`）都沿用原来的。

## 写插件

插件是一个文件夹，放进插件目录（App 的「插件」里点「打开插件目录」），保存就出现在手环上：

```jsx
// index.jsx
import { Col, Text, useState } from 'miband'

export default function Counter() {
  const [n, setN] = useState(0)
  return (
    <Col padding={16}>
      <Text style={{ fontSize: 18 }} onClick={() => setN(n + 1)}>点了 {n} 下</Text>
    </Col>
  )
}
```

```json
// manifest.json
{ "name": "counter", "title": "计数" }
```

写法和 React 一样（函数组件 + hooks），布局是 flex。插件的界面在手环上跑，点击、翻页都在手环本地处理；要读 Mac 上的文件、执行命令、发通知、点亮手环，就再写一部分在 Mac 上跑的代码，和手环上的界面互相传数据。Agent 猫读 herdr、提醒都是这么做的，本体不认识 herdr。插件可以声明配置项，App 会照着画出设置表单。想改内置插件的话，在 App 里点它的「复制出来改」。

完整说明见 [docs/plugins.md](docs/plugins.md)。

### 为什么这样写插件

小米手环的应用（快应用）正常的写法是 `.ux` 文件（类似 Vue），要用小米的 aiot-toolkit 编译、打包签名成安装包，再装到手环上。wristkit 换了一种做法：手环上只装一个通用的渲染器，插件用 JSX 写，App 编译好推过去，渲染器在手环上执行。这样：

- **不需要开发环境**：不用装 Node、小米的打包工具或 IDE，有个文本编辑器和 wristkit App 就够了。编译（esbuild）内置在 App 里。
- **不用打包、不用装**：保存文件，2 秒内手环上就变了。只推变了的代码和图片，按内容去重，推过的不再推。
- **手环上只有一个应用**：加多少个插件都还是这一个，插件之间切换是在应用里换页面，不用退出再打开别的应用；渲染器装一次，以后写插件不用再碰手环上的应用。
- **App 里能直接预览**：App 窗口里跑的是同一份插件、同一份数据，能点、能翻页，不用每改一次都看手环。
- **写法熟悉**：函数组件 + hooks，和 React 一样；布局是 flex。

代价是比手写快应用慢：插件要先在手环上算出界面（组件重算、转成界面树、和上次比较），再交给快应用去画，多了一层；一页的节点数、嵌套层数也有上限。

在小米手环 9 Pro 上，把 Agent 猫用 `.ux` 原生手写一遍做对照，从点下去到界面更新完：

| 操作 | wristkit 插件 | 原生 `.ux` |
|---|---|---|
| 点气泡换一个 agent（改几个文字和一个边框颜色） | 约 110ms | 约 30ms |
| 今日页切换小鱼干 / 钱数（7 根柱子和几个颜色一起变） | 约 440ms | 约 270ms |

差距主要在手环上的 JS：原生写法改数据只要几毫秒，插件要几十到两百多毫秒。插件运行时只重算变了的组件、没变的节点直接复用，所以组件拆得越细越快：点气泡只重算气泡，就比整页重算快一倍多。数据和优化过程见 [docs/render-spike.md](docs/render-spike.md)。

## 原理

```
 Mac（wristkit App）                                     手环（wristkit 应用）
 ┌─────────────────────────────────────┐   经典蓝牙   ┌─────────────────────────────┐
 │ 插件宿主：编译插件（esbuild），     │ ───────────▶ │ 通用渲染器：用 new Function │
 │ 按哈希推代码和图片，推数据          │  代码 / 数据  │ 加载插件，hooks 运行时算出  │
 │ 插件在 Mac 上跑的部分（goja）       │ ◀─────────── │ 界面树，照着画；点击、翻页  │
 │ 连手环、菜单栏                      │  调用 / 日志  │ 在手环本地处理              │
 └─────────────────────────────────────┘              └─────────────────────────────┘
```

- 手环上的快应用模板是编译时定死的，没法动态装组件，所以渲染器的模板把 6 层嵌套都写了出来，界面树的每个节点带一整串 style。
- 手环的 JS 引擎能执行动态下发的代码（支持到 ES2020），所以插件逻辑放在手环上跑，点击不用走蓝牙往返。
- 这些能不能做、有哪些限制（单条消息不超过 16KB、新建节点约 10ms 一个、JS 比 Mac 慢上百倍），是先在真机上做实验量出来的，见 [docs/render-spike.md](docs/render-spike.md)。

## 开发

```sh
make app        # 构建 macOS App（含手环上的应用）：server/app/build/bin/wristkit.app
make cli        # 构建命令行版 server/miband
make quickapps  # 只打包手环上的应用 render-app/dist/*.rpk
make test       # 跑 Go 测试（插件测试会在 goja 里跑手环上的同一份运行时）
```

改代码前先看 [AGENTS.md](AGENTS.md)：改动分「本体」和「插件」两层，能在插件里做的就在插件里做。

### 目录

- `server/app/`：macOS App。Wails v2 + React + shadcn/ui，界面跑在系统自带的 WKWebView 里。
- `server/internal/`
  - `plugin/`：插件宿主（编译插件、推代码图片和数据、插件在 Mac 上跑的部分、权限、配置、热更新）。
  - `service/`：常驻服务（连手环、跑插件）。App 和命令行共用。
  - `xiaomi/`：小米手环协议（SPP v2 帧、鉴权、AES-CCM/CTR、停等发送、通知、快应用安装和消息）。`go test` 里有个模拟手环做端到端测试，加密结果用 `testdata/vectors.json` 和原 Python 实现对照。
  - `bt/`：经典蓝牙 RFCOMM 和 BLE（cgo 调 macOS 的蓝牙框架）。
  - `usage/`：接管 Claude Code 状态栏拿额度的开关。
  - `micloud/`：扫码登录小米账号、拉手环密钥。
- `server/plugins/`：内置插件（Agent 猫：手环上的界面 + Mac 上读 herdr、额度、提醒），打包进 App。
- `render-app/`：手环上的渲染器。运行时在 `src/common/runtime.js`；页面模板由 `tools/gen_template.py` 生成，节点树格式和消息协议写在这两个文件开头。
- `tools/gen_pet.py`、`tools/gen_fish.py`：逐像素手绘猫和小鱼干（怎么画见 [docs/pixel-cat.md](docs/pixel-cat.md)）。
- `spike/`、`server/cmd/spike/`：真机实验的代码（[docs/render-spike.md](docs/render-spike.md)）。
- `*.py`：最早的 Python 实现，留作参考（快应用模式推的是改成插件之前的旧格式）。

### 签名

macOS 按「App 标识 + 签名证书」记蓝牙授权。不签名的话每次构建签名都变，每次都会重新弹蓝牙授权。`make app` 会用钥匙串里名为 `miband pet local signing` 的证书签名（名字沿用改名之前的），建一次即可（自己构建的和 Releases 里下载的证书不同，两者之间换着用要重新允许一次蓝牙）：

```sh
openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 3650 -nodes \
  -subj "/CN=miband pet local signing" -addext "keyUsage=critical,digitalSignature" \
  -addext "extendedKeyUsage=critical,codeSigning" -addext "basicConstraints=critical,CA:false"
openssl pkcs12 -export -legacy -inkey key.pem -in cert.pem -out cert.p12 -passout pass:tmp
security import cert.p12 -k ~/Library/Keychains/login.keychain-db -P tmp -T /usr/bin/codesign
rm key.pem cert.pem cert.p12
```

### 发版

推一个 `v` 开头的 tag（如 `git tag v0.1.0 && git push origin v0.1.0`），GitHub Actions（`.github/workflows/release.yml`）会构建 Intel / Apple 芯片通用的 App，用固定的自签证书「wristkit release signing」签名，打成 zip 挂到 Release 上。本机也能跑同样的构建：`make release VERSION=0.1.0 SIGN_IDENTITY="wristkit release signing"`。

证书和私钥在仓库的 Secrets 里（`MACOS_CERT_P12` 是 base64 编码的 .p12，`MACOS_CERT_PASSWORD` 是它的密码）。**一直用这同一张证书**：换了证书，所有用户更新后都要重新允许蓝牙。

### 命令行

App 和命令行版不要同时运行：手环同一时间只接受一个连接。

```sh
server/miband app                 # 常驻：跑插件、推给手环
server/miband notify 标题 正文     # 发一条通知
server/miband apps                # 列出手环上的快应用
server/miband launch [包名]        # 把快应用打开到前台（默认 wristkit），手环会亮屏
server/miband key                 # 扫码登录小米账号，拉手环的密钥和地址写进 ./config.json
server/miband install render-app/dist/dev.bysir.mibandpet.debug.*.rpk
```

配置读 `./config.json`（或用 `-config` 指定；App 的配置在 `~/Library/Application Support/miband-pet/config.json`）：`auth_key`、`address` 是手环的密钥和地址；`plugin_config` 是插件的配置，比如 `{"agent-cat": {"alert": "wake"}}`（提醒方式 `notify` / `wake` / `off`）。

### 踩过的坑

- **经典蓝牙要用异步的 `openRFCOMMChannelAsync`**。手环会把 Mac 当成自己的手机主动连上来，这时同步的 `openRFCOMMChannelSync` 会误报 `kIOReturnError`（通道其实已经连上），看起来就像「要重启手环才能连」。也不能在派发到主队列的块里等这个回调（回调同样走主队列，会死锁）。
- 通道没关就退出会让手环那边卡住，所以退出前一定关通道。
- 手环 10 系列不回应 SPP v1 的版本探测包，超时后直接开 v2 会话。
- 给快应用发消息要带包名和签名指纹（从 `GET_INSTALLED_LIST` 拿）；快应用打开时会来问「手机端在不在」，要回 `SYNC_PHONE_APP_STATUS=CONNECTED`，否则 interconnect 不会触发 onopen。
- 装快应用：`PREPARE_INSTALL_APP` → MASS prepare → 分片走明文的 mass 通道，靠 SPP v2 的 ACK 确认 → 等 `REPORT_INSTALL_RESULT`。
- 账号密码登录会被小米的二次验证拦下，所以改用扫码登录；小米运动健康 3.59 版的日志里已经没有密钥了。
