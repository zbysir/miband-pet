# miband pet

让小米手环 10 Pro 变成 AI agent 的状态宠物：Mac 通过蓝牙直连手环（不经过手机），把 [herdr](https://herdr.dev) 管理的 agent 状态推到手环上的快应用里。

- **Agent 猫**（`pet-app/`）：一只像素橘猫代表所有 agent 的整体状态。有 agent 卡住了它举牌，跑完一个就庆祝，有在跑的就敲笔记本，都空闲时睡觉。
- **Agents 列表**（`band-app/`）：按状态排序，列出每个 agent 的项目和任务。
- **通知**：agent 开始跑、跑完、卡住时，手环震动提醒。

> 非小米官方项目。用的是逆向出来的非官方协议（移植自 Gadgetbridge、OronBox），风险自负。许可证 AGPL-3.0，致谢见 [NOTICE.md](NOTICE.md)。

## 后端服务（Go，server/）

日常用的是 Go 写的 `server/miband`，蓝牙也在里面（cgo 调 macOS IOBluetooth 开经典蓝牙 RFCOMM）：

```sh
cd server && go build -o miband ./cmd/miband && cd ..
server/miband app                 # 常驻：推 agent 列表给快应用；agent 跑完/卡住时发通知
server/miband notify 标题 正文     # 发一条通知
server/miband apps                # 列出手环上的快应用
server/miband launch [包名]        # 把快应用打开到前台（默认宠物版），手环会亮屏
server/miband install pet-app/dist/dev.bysir.herdrpet.debug.1.0.0.rpk
```

配置读 `./config.json`（或用 `-config` 指定）。agent 开始跑、跑完、卡住时会提醒手环：

- `"alert"`：提醒方式。`"auto"`（默认）：手环上快应用正在显示时只亮屏、不震动，否则发通知震动；`"wake"` 总是把宠物快应用打开到前台（亮屏不震动）；`"notify"` 总是发通知；`"both"` 两者都要。
  快应用在前台时会给 Mac 发 `hello`，离开时（右滑退出、熄屏后回到表盘）发 `bye`，Mac 据此判断。
- `"notify_start": false`：开始跑时不提醒，只在跑完、卡住时提醒。
- `"notify": false`：完全不提醒（快应用里的数据照常更新）。

- `internal/bt/`：RFCOMM 桥接。IOBluetooth 的回调只投递到主线程，所以 `main` 里调 `bt.RunMain`，主线程常驻 run loop。
- `internal/xiaomi/`：协议（SPP v2 帧、鉴权、AES-CCM/CTR、停等发送、通知、快应用安装和消息）。`go test ./...` 里有个模拟手环做端到端测试，加密结果用 `testdata/vectors.json` 和原 Python 实现对照。
- `internal/herdr/`：读 `herdr agent list`、算状态变化事件。

踩过的坑：
- **要用异步的 `openRFCOMMChannelAsync`**。手环会把 Mac 当成自己的手机主动连上来，这时同步的 `openRFCOMMChannelSync` 会误报 `kIOReturnError`（系统日志里通道其实已经连上），看起来就像“要重启手环才能连”。
- 也不能在派发到主队列的块里等这个回调（回调同样走主队列，会死锁）；结果通过 cgo 回调通知 Go。
- 通道没关就退出会让手环那边卡住，所以程序退出前一定关通道。

Python 版（`main.py` 等）是最早的实现，留作参考；`get_key.py` 和 `pet-app/tools/gen_pet.py` 是一次性工具，仍用 Python。

## 准备（以手环 10 Pro 为例，实测可用）

1. 手机上用小米运动健康扫手环的绑定二维码，正常绑定一次。这是唯一需要手机的一步。
2. 拉 key：`.venv/bin/python get_key.py`。它会弹出二维码，用手机扫码登录小米账号，然后从小米健康云拉设备的 `encrypt_key`，连同 MAC 写进 `config.json`。登录态缓存在 `.xiaomi_session.json`，下次不用再扫码。
3. 让手环脱离手机：在手机**系统蓝牙**里取消配对手环，再关掉手机蓝牙。不要在 App 里解绑，解绑后 key 会作废。
4. 首次接管：手环停在「连接新设备」的二维码界面时会广播 BLE，跑 `transport: "ble2"` 连一次，鉴权成功后 Mac 会和手环完成系统层配对。
5. 之后把 `transport` 改成 `spp`（经典蓝牙），日常都走这个。连不上时重启一次手环。

几个小知识：
- 手环 10 系列不回应 SPP v1 的版本探测包，代码超时后会直接开 v2 会话。
- 账号密码登录（huami-token）会被小米的二次验证拦下，所以 `get_key.py` 改用扫码登录。
- 小米运动健康 3.59 版的日志里已经没有 key 了。

## 使用

```sh
python3 -m venv .venv && .venv/bin/pip install bleak cryptography protobuf pyobjc-framework-IOBluetooth \
    git+https://github.com/argrento/huami-token   # get_key.py 要用
protoc -Iproto --python_out=. proto/xiaomi.proto proto/vela.proto   # 改了 proto 才需要重新生成

.venv/bin/python main.py scan                 # 找手环，地址可以填进 config.json 的 address，省掉每次扫描
.venv/bin/python main.py notify 你好 测试      # 发一条通知试试
.venv/bin/python main.py watch                # 常驻，监听 herdr
```

`watch` 每 3 秒跑一次 `herdr agent list`，agent 从 working 变成 done/idle，或者变成 blocked 时推送通知；断线后每 15 秒重连一次，没发出去的通知等重连后补发。

第一次运行时 macOS 会请求蓝牙权限，授权对象是你用的终端 App。

## 手环快应用（band-app/）

手环端是 Vela JS 快应用，界面在 `band-app/src/pages/index/index.ux`，用 `@system.interconnect` 收 Mac 推来的 JSON：

- Mac → 快应用：`{"t":"agents","ts":毫秒,"agents":[{id, agent, project, title, status}]}`
- 快应用 → Mac：打开时发 `{"t":"hello"}`，Mac 收到后立刻推一次全量

```sh
cd band-app && npm install && npx aiot build      # 打包成 dist/*.rpk
cd .. && .venv/bin/python main.py install         # 装到手环并打开（重装同包名会覆盖）
.venv/bin/python main.py app                      # 常驻：herdr 有变化就推给快应用
```

安装和消息协议参考 OronBox（`vela.py` 文件头有说明）：先 `PREPARE_INSTALL_APP`，再 MASS prepare，分片走明文的 mass 通道，靠 SPP v2 的 ACK 做确认，最后等 `REPORT_INSTALL_RESULT`。给快应用发消息要带包名和签名指纹，指纹从 `GET_INSTALLED_LIST` 拿；快应用打开时会来问「手机端在不在」，要回 `SYNC_PHONE_APP_STATUS=CONNECTED`，否则 interconnect 不会触发 onopen。

## 宠物版（pet-app/）

另一个快应用「Agent 猫」，用一只像素橘猫表现 agent 的整体状态：有卡住的举牌摇晃，刚完成一个就庆祝 8 秒，有在跑的就敲笔记本，否则睡觉。气泡里显示最值得关注的 agent，点一下猫切换；右滑退出。

- 素材由 `pet-app/tools/gen_pet.py` 用代码画（40×40 像素网格放大 4 倍），改形象只改这个脚本再重新生成。
- 手环只支持 png/jpg，不支持 GIF 和视频；动画靠 `image-animator` 帧动画加 CSS `@keyframes`。
- `pet-app/node_modules` 是指向 `band-app/node_modules` 的软链。

```sh
.venv/bin/python pet-app/tools/gen_pet.py [预览图.png]   # 重新生成帧
cd pet-app && npx aiot build && cd ..
.venv/bin/python main.py install pet-app/dist/dev.bysir.herdrpet.debug.1.0.0.rpk
```

`main.py app` 会同时给列表版和宠物版推数据（装了哪个推哪个）。

## 文件

- `miband.py`：协议层（鉴权 HKDF + AES-CCM、对时、发通知）和 BLE 传输
- `spp.py`：经典蓝牙 SPP v2 传输（IOBluetooth RFCOMM，帧带 CRC16，AES-CTR 加密）
- `vela.py`：快应用的安装、启动、收发消息
- `main.py`：命令行入口、herdr 监听（通知模式 `watch` / 快应用模式 `app`）
- `band-app/`：列表版快应用；`pet-app/`：宠物版快应用
- `xiaomi_pb2.py` / `vela_pb2.py`：由 `proto/` 下的 proto 生成
