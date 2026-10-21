"""经典蓝牙 SPP 传输：手环 8 Pro / 9 / 9 Pro / 10 Pro 等。

移植自 Gadgetbridge 的 XiaomiSppSupport + XiaomiSppProtocolV2：
- 先发一个 v1 格式的版本探测包，手环回复版本 >= 2 就走 v2
- v2 帧：a5 a5 | 类型 | 序号 | 长度(u16) | CRC16/ARC(u16) | 负载
- 数据帧负载：通道 | opcode(1 明文 / 2 加密) | 内容；加密是 AES-CTR
- macOS 上用 IOBluetooth 开 RFCOMM，回调要靠 NSRunLoop 驱动，所以在 asyncio 里定时泵一下 runloop
"""

import asyncio
import logging
import struct

import IOBluetooth
import objc
from Foundation import NSDate, NSDefaultRunLoopMode, NSObject, NSRunLoop

import xiaomi_pb2 as pb
from miband import AUTH_TYPE, XiaomiDevice

log = logging.getLogger("miband.spp")

SPP_UUID16 = 0x1101

V1_PREAMBLE = b"\xba\xdc\xfe"
V1_EPILOGUE = b"\xef"
V2_PREAMBLE = b"\xa5\xa5"

V2_NAK = 0
V2_ACK = 1
V2_SESSION_CONFIG = 2
V2_DATA = 3

CH_PROTOBUF = 1
OP_PLAIN = 1
OP_ENCRYPTED = 2

# 抓包自官方 App：start session，版本 1.0.0，最大帧 0xfc00，窗口 32，超时 10000ms
SESSION_START = bytes([
    1,
    1, 0x03, 0x00, 0x01, 0x00, 0x00,
    2, 0x02, 0x00, 0x00, 0xfc,
    3, 0x02, 0x00, 0x20, 0x00,
    4, 0x02, 0x00, 0x10, 0x27,
])


def crc16_arc(data: bytes) -> int:
    crc = 0
    for b in data:
        crc ^= b
        for _ in range(8):
            crc = (crc >> 1) ^ 0xA001 if crc & 1 else crc >> 1
    return crc


def v2_frame(kind: int, seq: int, payload: bytes = b"") -> bytes:
    return V2_PREAMBLE + struct.pack("<BBHH", kind & 0xF, seq & 0xFF, len(payload), crc16_arc(payload)) + payload


def v1_version_request() -> bytes:
    # 通道 0(version)，flags 0x40(needs response)，长度 3，opcode 0(read)，serial 0，data type 0(plain)
    return V1_PREAMBLE + bytes([0x00, 0x40]) + struct.pack("<H", 3) + bytes([0, 0, 0]) + V1_EPILOGUE


class _RfcommDelegate(NSObject):
    def initWithOwner_(self, owner):
        self = objc.super(_RfcommDelegate, self).init()
        if self is not None:
            self.owner = owner
        return self

    def rfcommChannelData_data_length_(self, channel, data, length):
        if hasattr(data, "as_buffer"):
            raw = bytes(data.as_buffer(length))
        else:
            raw = bytes(data[:length])
        self.owner._on_bytes(raw)

    def rfcommChannelClosed_(self, channel):
        self.owner._on_closed()


def find_paired(name_hint: str = "Band") -> list:
    devs = IOBluetooth.IOBluetoothDevice.pairedDevices() or []
    return [d for d in devs if name_hint.lower() in (d.name() or "").lower()]


class MiBandSpp(XiaomiDevice):
    def __init__(self, auth_key: str, address: str | None = None):
        super().__init__(auth_key, address)
        self.device = None
        self.channel = None
        self._delegate = None
        self._buf = bytearray()
        self._seq = 0
        self._version: asyncio.Future | None = None
        self._session: asyncio.Future | None = None
        self._pump_task: asyncio.Task | None = None
        self._open = False
        self._acks: dict[int, asyncio.Future] = {}
        # 手环 10 系列不回 v1 版本探测，默认直接开 v2 session（同 Gadgetbridge XiaomiBleProtocolV2）
        self.probe_v1 = False

    @property
    def connected(self) -> bool:
        return self._open and self.authed

    # ---------- runloop ----------

    async def _pump(self):
        loop = NSRunLoop.currentRunLoop()
        while True:
            loop.runMode_beforeDate_(NSDefaultRunLoopMode, NSDate.dateWithTimeIntervalSinceNow_(0.005))
            await asyncio.sleep(0.01)

    # ---------- 连接 ----------

    async def connect(self):
        if self._pump_task is None:
            self._pump_task = asyncio.ensure_future(self._pump())
        self.crypto = None
        self.authed = False
        self._buf.clear()
        self._seq = 0

        if self.address:
            self.device = IOBluetooth.IOBluetoothDevice.deviceWithAddressString_(self.address.replace(":", "-"))
        else:
            cands = find_paired()
            if not cands:
                raise RuntimeError("Mac 的蓝牙里没有已配对的手环：先在 系统设置 → 蓝牙 里配对手环（配对前手机要关蓝牙）")
            self.device = cands[0]
        log.info("连接 %s (%s)", self.device.name(), self.device.addressString())

        ret = self.device.openConnection()
        if ret != 0:
            raise RuntimeError(f"建立蓝牙连接失败 (IOReturn 0x{ret & 0xffffffff:x})，手环是不是还连着手机？")

        ret = self.device.performSDPQuery_(None)
        rec = None
        for _ in range(50):  # SDP 查询是异步的，等结果回来
            rec = self.device.getServiceRecordForUUID_(IOBluetooth.IOBluetoothSDPUUID.uuid16_(SPP_UUID16))
            if rec is not None:
                break
            await asyncio.sleep(0.1)
        if rec is None:
            self._dump_services()
            raise RuntimeError("手环上没找到 SPP 服务")
        ret, channel_id = rec.getRFCOMMChannelID_(None)
        if ret != 0:
            raise RuntimeError(f"读取 RFCOMM 通道号失败 (0x{ret & 0xffffffff:x})")
        log.info("SPP 通道 %d", channel_id)

        self._delegate = _RfcommDelegate.alloc().initWithOwner_(self)
        ret, self.channel = self.device.openRFCOMMChannelSync_withChannelID_delegate_(None, channel_id, self._delegate)
        if ret != 0 or self.channel is None:
            raise RuntimeError(f"打开 RFCOMM 失败 (0x{ret & 0xffffffff:x})")
        self._open = True
        log.info("RFCOMM 已打开，MTU %d", self.channel.getMTU())

        loop = asyncio.get_running_loop()
        if self.probe_v1:
            self._version = loop.create_future()
            self._write(v1_version_request())
            try:
                version = await asyncio.wait_for(self._version, 3)
                log.info("SPP 协议版本 %s", version.hex())
                if not version or version[0] < 2:
                    raise RuntimeError(f"SPP 版本 {version.hex()} < 2，这里只实现了 v2")
            except asyncio.TimeoutError:
                log.info("没收到版本号，直接走 v2")

        self._session = loop.create_future()
        self._write(v2_frame(V2_SESSION_CONFIG, 0, SESSION_START))
        await asyncio.wait_for(self._session, 5)
        log.info("session 已建立")

        await self._authenticate()
        await self.set_time()

    def _dump_services(self):
        for rec in self.device.services() or []:
            log.info("  服务: %s", rec.getServiceName())

    async def disconnect(self):
        if self.channel is not None:
            self.channel.closeChannel()
            self.channel = None
        if self.device is not None:
            self.device.closeConnection()
        self._open = False
        self.authed = False

    def _on_closed(self):
        self._open = False
        self._on_disconnected()

    # ---------- 收发 ----------

    def _write(self, data: bytes):
        mtu = self.channel.getMTU() or 127
        for i in range(0, len(data), mtu):
            part = data[i:i + mtu]
            ret = self.channel.writeSync_length_(part, len(part))
            if ret != 0:
                raise RuntimeError(f"RFCOMM 写入失败 (0x{ret & 0xffffffff:x})")

    def _next_seq(self) -> int:
        seq = self._seq
        self._seq = (self._seq + 1) & 0xFF
        return seq

    async def send_raw(self, cmd_type: int, payload: bytes):
        # 鉴权命令走明文；其它命令鉴权后加密
        if cmd_type != AUTH_TYPE and self.authed:
            body = bytes([CH_PROTOBUF, OP_ENCRYPTED]) + self.crypto.encrypt_v2(payload)
        else:
            body = bytes([CH_PROTOBUF, OP_PLAIN]) + payload
        # 鉴权过程中由回调驱动，不等 ack
        await self.send_data(body, wait_ack=cmd_type != AUTH_TYPE)

    async def send_data(self, body: bytes, wait_ack: bool = True, timeout: float = 5, retries: int = 3):
        """发一个 v2 数据帧并等手环 ACK（停等）。NAK 或超时就原样重发。

        不等 ack 紧接着断开的话，手环来不及处理（实测通知会丢）。
        """
        seq = self._next_seq()
        frame = v2_frame(V2_DATA, seq, body)
        if not wait_ack:
            self._write(frame)
            return
        for attempt in range(retries + 1):
            ack = self._acks[seq] = asyncio.get_running_loop().create_future()
            self._write(frame)
            try:
                if await asyncio.wait_for(ack, timeout):
                    return
                log.debug("seq=%d 收到 NAK，重发", seq)
            except asyncio.TimeoutError:
                log.debug("seq=%d 等 ack 超时，重发", seq)
            finally:
                self._acks.pop(seq, None)
        raise RuntimeError(f"数据帧 seq={seq} 重发 {retries} 次仍没被确认")

    def _on_bytes(self, data: bytes):
        log.debug("收到 %s", data.hex())
        self._buf += data
        while self._buf:
            n = self._parse_one()
            if n == 0:
                return
            del self._buf[:n]

    def _parse_one(self) -> int:
        """返回消耗的字节数；0 表示数据不完整，等后续字节。"""
        buf = bytes(self._buf)
        if buf.startswith(V1_PREAMBLE):
            if len(buf) < 11:
                return 0
            length = struct.unpack_from("<H", buf, 5)[0] - 3
            total = 11 + length
            if len(buf) < total:
                return 0
            channel = buf[3] & 0xF
            payload = buf[10:10 + length]
            if channel == 0 and self._version and not self._version.done():
                self._version.set_result(payload)
            else:
                log.debug("忽略 v1 包 channel=%d", channel)
            return total

        if buf.startswith(V2_PREAMBLE):
            if len(buf) < 8:
                return 0
            kind = buf[2] & 0xF
            seq = buf[3]
            length, crc = struct.unpack_from("<HH", buf, 4)
            total = 8 + length
            if len(buf) < total:
                return 0
            payload = buf[8:total]
            if crc16_arc(payload) != crc:
                log.warning("v2 包 CRC 不对，丢弃")
                return total
            self._handle_v2(kind, seq, payload)
            return total

        # 不认识的字节：跳到下一个可能的包头
        for i in range(1, len(buf)):
            if buf[i] in (V1_PREAMBLE[0], V2_PREAMBLE[0]):
                return i
        return len(buf)

    def _handle_v2(self, kind: int, seq: int, payload: bytes):
        if kind == V2_SESSION_CONFIG:
            if self._session and not self._session.done():
                self._session.set_result(payload)
        elif kind == V2_DATA:
            self._write(v2_frame(V2_ACK, seq))
            if len(payload) < 2:
                return
            channel, op, body = payload[0] & 0xF, payload[1], payload[2:]
            if op == OP_ENCRYPTED:
                body = self.crypto.decrypt_v2(body)
            if channel == CH_PROTOBUF:
                self.on_command_bytes(body)
            else:
                log.debug("忽略通道 %d 的数据", channel)
        elif kind in (V2_ACK, V2_NAK):
            # ACK(S)：S 及之前的帧都确认了（累计确认，序号 8 位回绕）
            # NAK(S)：S 之前的确认了，S 及之后的要重发
            for pending, fut in list(self._acks.items()):
                if fut.done():
                    continue
                if kind == V2_ACK and (seq - pending) & 0xFF < 128:
                    fut.set_result(True)
                elif kind == V2_NAK:
                    fut.set_result((seq - pending) & 0xFF < 128 and pending != seq)
        else:
            log.debug("未知 v2 包类型 %d", kind)


class MiBandBle2(MiBandSpp):
    """手环 10 / 10 Pro 等：SPP v2 帧跑在 BLE 上（fe95 服务，0x5f 写、0x5e 通知），见 Gadgetbridge XiaomiBleProtocolV2。

    跟经典蓝牙 SPP 的区别：没有 v1 版本探测，直接发 session config；字节流按 BLE 单包上限切块写。
    """

    SERVICE = "0000fe95-0000-1000-8000-00805f9b34fb"
    RX = "0000005e-0000-1000-8000-00805f9b34fb"
    TX = "0000005f-0000-1000-8000-00805f9b34fb"

    def __init__(self, auth_key: str, address: str | None = None, mac: str | None = None):
        super().__init__(auth_key, address)
        self.mac = bytes.fromhex(mac.replace(":", "")) if mac else None
        self.client = None
        self._wq: asyncio.Queue[bytes] | None = None
        self._writer: asyncio.Task | None = None
        self._chunk = 20

    async def find(self, timeout: float = 30):
        from bleak import BleakScanner

        def match(d, adv):
            # 手环不带名字广播，靠 MiBeacon（fe95 服务数据）里倒序的 MAC 认
            data = adv.service_data.get(self.SERVICE, b"")
            return bool(self.mac) and self.mac[::-1] in data

        dev = await BleakScanner.find_device_by_filter(match, timeout=timeout)
        return dev

    async def connect(self):
        from bleak import BleakClient

        self.crypto = None
        self.authed = False
        self._buf.clear()
        self._seq = 0

        target = self.address or await self.find()
        if not target:
            raise RuntimeError("BLE 没扫到手环：手机蓝牙要关着，手环在旁边")
        log.info("BLE 连接 %s", getattr(target, "address", target))
        self.client = BleakClient(target, disconnected_callback=lambda _c: self._on_closed())
        await self.client.connect()
        char = self.client.services.get_characteristic(self.TX)
        if char is None or self.client.services.get_characteristic(self.RX) is None:
            raise RuntimeError("手环上没有 fe95/5e/5f 特征")
        self._chunk = max(20, char.max_write_without_response_size)
        log.info("已连接，MTU %s，单包 %d 字节", self.client.mtu_size, self._chunk)
        self._open = True

        self._wq = asyncio.Queue()
        self._writer = asyncio.ensure_future(self._write_loop())
        await self.client.start_notify(self.RX, lambda _c, d: self._on_bytes(bytes(d)))

        self._session = asyncio.get_running_loop().create_future()
        self._write(v2_frame(V2_SESSION_CONFIG, 0, SESSION_START))
        await asyncio.wait_for(self._session, 10)
        log.info("session 已建立")

        await self._authenticate()
        await self.set_time()

    def _write(self, data: bytes):
        self._wq.put_nowait(data)

    async def _write_loop(self):
        while True:
            data = await self._wq.get()
            try:
                for i in range(0, len(data), self._chunk):
                    await self.client.write_gatt_char(self.TX, data[i:i + self._chunk], response=False)
            except Exception as e:
                log.warning("BLE 写入失败: %s", e)
            finally:
                self._wq.task_done()

    async def send_raw(self, cmd_type, payload):
        await super().send_raw(cmd_type, payload)
        await self._wq.join()

    async def disconnect(self):
        if self._writer:
            self._writer.cancel()
        if self.client:
            await self.client.disconnect()
        self._open = False
        self.authed = False
