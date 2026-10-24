# 致谢与第三方来源

本项目整体以 **GNU AGPL-3.0** 发布（见 `LICENSE`），因为协议实现移植自下面两个 AGPL-3.0 项目：

- **[Gadgetbridge](https://codeberg.org/Freeyourgadget/Gadgetbridge)**（AGPL-3.0）
  小米穿戴设备的 BLE/SPP 协议、鉴权流程、protobuf 定义。`proto/xiaomi.proto` 直接取自该项目。
- **[OronBox](https://github.com/zxor-org/OronBox)**（AGPL-3.0）
  Vela 第三方快应用的安装（MASS 大文件传输）和消息收发协议。`proto/vela.proto` 摘自该项目的 `protos/xiaomi/`。

另外参考或使用了：

- **[AIStatusForMiBand10pro](https://github.com/YuzH1/AIStatusForMiBand10pro)**（MIT，Copyright (c) 2026 YangZifan (YuzH1)）
  快应用的写法参考了它；`server/internal/quickapp/sign/`、`spike/render-app/sign/debug/` 下的调试签名证书取自该项目。MIT 许可全文见下。
- **[huami-token](https://github.com/argrento/huami-token)**（MIT）
  `get_key.py` 依赖它的小米健康云接口封装（作为依赖安装，未包含其代码）。
- **[AstroBox](https://github.com/AstralSightStudios/AstroBox-Public)**（AGPL-3.0）
  核对快应用协议时参考。

本项目与小米公司无关，不是官方产品。使用非官方协议连接手环，风险自负。

---

MIT License（适用于 AIStatusForMiBand10pro 的相关文件）

Copyright (c) 2026 YangZifan (YuzH1)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
