"""Mac 直连小米手环 8 的最小 BLE 客户端。

协议移植自 Gadgetbridge 的 Xiaomi 实现（service/devices/xiaomi/）：
- 服务 0000fe95，命令从 0051 收、往 0052 写，帧头 + protobuf(xiaomi.proto)
- 鉴权：交换 nonce → HKDF(“miwear-auth”) 派生密钥 → 之后 AES-CCM(4 字节 tag) 加密
"""

import asyncio
import hashlib
import hmac
import logging
import os
import struct
import time

from bleak import BleakClient, BleakScanner
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
from cryptography.hazmat.primitives.ciphers.aead import AESCCM

import xiaomi_pb2 as pb

log = logging.getLogger("miband")

UUID_SERVICE = "0000fe95-0000-1000-8000-00805f9b34fb"
UUID_CMD_READ = "00000051-0000-1000-8000-00805f9b34fb"
UUID_CMD_WRITE = "00000052-0000-1000-8000-00805f9b34fb"
UUID_ACTIVITY = "00000053-0000-1000-8000-00805f9b34fb"
UUID_UPLOAD = "00000055-0000-1000-8000-00805f9b34fb"

NAME_PREFIX = "Xiaomi Smart Band 8"

AUTH_TYPE = 1
AUTH_CMD_NONCE = 26
AUTH_CMD_AUTH = 27
NOTIFICATION_TYPE = 7
NOTIFICATION_CMD_SEND = 0
SYSTEM_TYPE = 2
SYSTEM_CMD_SET_TIME = 3

ACK_TIMEOUT = 10


def _hmac(key: bytes, data: bytes) -> bytes:
    return hmac.new(key, data, hashlib.sha256).digest()


def derive_keys(secret: bytes, phone_nonce: bytes, watch_nonce: bytes) -> bytes:
    """XiaomiAuthService.computeAuthStep3Hmac：本质是 HKDF-SHA256，info="miwear-auth"，输出 64 字节。"""
    prk = _hmac(phone_nonce + watch_nonce, secret)
    out, t, i = b"", b"", 1
    while len(out) < 64:
        t = _hmac(prk, t + b"miwear-auth" + bytes([i]))
        out += t
        i += 1
    return out[:64]


class Crypto:
    def __init__(self, okm: bytes):
        self.dec_key = okm[0:16]
        self.enc_key = okm[16:32]
        self.dec_nonce = okm[32:36]
        self.enc_nonce = okm[36:40]

    def encrypt(self, data: bytes, index: int) -> bytes:
        nonce = self.enc_nonce + struct.pack("<II", 0, index)
        return AESCCM(self.enc_key, tag_length=4).encrypt(nonce, data, None)

    def decrypt(self, data: bytes) -> bytes:
        nonce = self.dec_nonce + struct.pack("<II", 0, 0)
        return AESCCM(self.dec_key, tag_length=4).decrypt(nonce, data, None)

    # SPP v2 用的是 AES-CTR，IV 直接用 key（Gadgetbridge 原注释：“I wish I was kidding”）
    def encrypt_v2(self, data: bytes) -> bytes:
        return Cipher(algorithms.AES(self.enc_key), modes.CTR(self.enc_key)).encryptor().update(data)

    def decrypt_v2(self, data: bytes) -> bytes:
        return Cipher(algorithms.AES(self.dec_key), modes.CTR(self.dec_key)).decryptor().update(data)


class Channel:
    """对应 Gadgetbridge 的 XiaomiCharacteristic：负责一个特征上的分包、ack、加解密。"""

    def __init__(self, band: "MiBand8", uuid: str):
        self.band = band
        self.uuid = uuid
        self.num_chunks = 0
        self.chunks: dict[int, bytes] = {}
        self.encrypted_index = 0
        self._ack: asyncio.Future | None = None
        self._send_lock = asyncio.Lock()

    async def _raw_write(self, data: bytes):
        await self.band.client.write_gatt_char(self.uuid, data, response=False)

    # ---------- 接收 ----------

    def on_notify(self, data: bytes):
        chunk = struct.unpack_from("<H", data, 0)[0]
        if chunk != 0:
            if chunk > self.num_chunks:
                log.warning("%s: 忽略越界分包 %d/%d", self.uuid[4:8], chunk, self.num_chunks)
                return
            self.chunks[chunk] = data[2:]
            if len(self.chunks) == self.num_chunks:
                payload = b"".join(self.chunks[i + 1] for i in range(self.num_chunks))
                self.num_chunks = 0
                self.chunks.clear()
                self.band.spawn(self._raw_write(b"\x00\x00\x01\x00"))
                if self.band.crypto:
                    payload = self.band.crypto.decrypt(payload)
                self.band.on_command_bytes(payload)
            return

        kind = data[2]
        if kind == 0:  # 对端开始分包发送
            self.num_chunks = struct.unpack_from("<H", data, 4)[0]
            self.chunks.clear()
            self.band.spawn(self._raw_write(b"\x00\x00\x01\x01"))
        elif kind == 1:  # 对端对我们分包发送的 ack
            self._resolve(("chunk", data[3]))
        elif kind == 2:  # 单包命令
            self.band.spawn(self._raw_write(b"\x00\x00\x03\x00"))
            payload = data[4:]
            if data[3] == 1:
                payload = self.band.crypto.decrypt(payload)
            self.band.on_command_bytes(payload)
        elif kind == 3:  # 对我们单包命令的 ack
            self._resolve(("single", data[3]))
        else:
            log.warning("%s: 未知帧类型 %d: %s", self.uuid[4:8], kind, data.hex())

    def _resolve(self, value):
        if self._ack and not self._ack.done():
            self._ack.set_result(value)

    async def _wait_ack(self):
        try:
            return await asyncio.wait_for(self._ack, ACK_TIMEOUT)
        finally:
            self._ack = None

    # ---------- 发送 ----------

    async def write(self, payload: bytes):
        async with self._send_lock:
            crypto = self.band.crypto if self.band.authed else None
            max_write = self.band.max_write
            if crypto:
                payload = crypto.encrypt(payload, self.encrypted_index)

            loop = asyncio.get_running_loop()
            if len(payload) + 6 <= max_write:
                if crypto:
                    header = struct.pack("<HBBH", 0, 2, 1, self.encrypted_index)
                    self.encrypted_index += 1
                else:
                    header = struct.pack("<HBB", 0, 2, 2)
                self._ack = loop.create_future()
                await self._raw_write(header + payload)
                kind, result = await self._wait_ack()
                if result != 0:
                    raise RuntimeError(f"手环拒收命令 (nack {result})")
                return

            if crypto:
                payload = struct.pack("<H", self.encrypted_index) + payload
                self.encrypted_index += 1
            size = max_write - 2
            n = -(-len(payload) // size)
            self._ack = loop.create_future()
            await self._raw_write(struct.pack("<HBBH", 0, 0, 1 if crypto else 0, n))
            if await self._wait_ack() != ("chunk", 1):
                raise RuntimeError("手环没接受分包传输")
            self._ack = loop.create_future()
            for i in range(n):
                await self._raw_write(struct.pack("<H", i + 1) + payload[i * size:(i + 1) * size])
            kind, sub = await self._wait_ack()
            if sub != 0:
                raise RuntimeError(f"分包发送失败 (subtype {sub})")


def _peek_type_id(data: bytes) -> tuple[int, int]:
    """只解析字段 1(type) 和 2(id)；不同 type 的负载用不同的 proto，先看类型再分发。"""
    i, cmd_type, cmd_id = 0, -1, 0
    try:
        while i < len(data):
            key, i = _varint(data, i)
            field, wire = key >> 3, key & 7
            if wire == 0:
                val, i = _varint(data, i)
                if field == 1:
                    cmd_type = val
                elif field == 2:
                    cmd_id = val
            elif wire == 2:
                n, i = _varint(data, i)
                i += n
            elif wire == 5:
                i += 4
            elif wire == 1:
                i += 8
            else:
                break
    except IndexError:
        pass
    return cmd_type, cmd_id


def _varint(data: bytes, i: int) -> tuple[int, int]:
    val = shift = 0
    while True:
        b = data[i]
        i += 1
        val |= (b & 0x7F) << shift
        if not b & 0x80:
            return val, i
        shift += 7


class XiaomiDevice:
    """协议层：鉴权、对时、发通知。子类负责传输（BLE / 经典蓝牙 SPP），实现 connect / disconnect / send_command。"""

    def __init__(self, auth_key: str, address: str | None = None):
        key = auth_key.strip().removeprefix("0x")
        self.secret = bytes.fromhex(key)[:16].ljust(16, b"\x00")
        self.address = address
        self.crypto: Crypto | None = None  # 收到手环 nonce 后就有，用于解密
        self.authed = False  # 鉴权成功后，往外发的命令才加密
        self._phone_nonce = b""
        self._auth_done: asyncio.Future | None = None
        self._tasks: set[asyncio.Task] = set()
        self.on_disconnect = None
        # 按命令 type 注册的处理函数（收到原始 protobuf 字节）；用于 vela.py 这类扩展
        self.packet_handlers: dict[int, callable] = {}
        self._waiters: list[tuple[int, int, asyncio.Future]] = []

    def spawn(self, coro):
        task = asyncio.ensure_future(coro)
        self._tasks.add(task)
        task.add_done_callback(self._tasks.discard)

    @property
    def connected(self) -> bool:
        raise NotImplementedError

    async def connect(self):
        raise NotImplementedError

    async def disconnect(self):
        raise NotImplementedError

    async def send_raw(self, cmd_type: int, data: bytes):
        """发一个已经序列化好的 protobuf 包；cmd_type 决定走鉴权通道还是加密通道。"""
        raise NotImplementedError

    async def send_command(self, cmd: pb.Command):
        await self.send_raw(cmd.type, cmd.SerializeToString())

    def expect(self, cmd_type: int, cmd_id: int) -> asyncio.Future:
        """先登记要等的回包，再发请求，避免回包比 await 先到。结果是回包的原始字节。"""
        fut = asyncio.get_running_loop().create_future()
        self._waiters.append((cmd_type, cmd_id, fut))
        return fut

    async def request(self, cmd_type: int, cmd_id: int, data: bytes, reply_id: int | None = None, timeout: float = 10) -> bytes:
        fut = self.expect(cmd_type, cmd_id if reply_id is None else reply_id)
        await self.send_raw(cmd_type, data)
        try:
            return await asyncio.wait_for(fut, timeout)
        finally:
            self._waiters = [w for w in self._waiters if w[2] is not fut]

    def _on_disconnected(self, *_):
        log.warning("手环断开")
        self.authed = False
        if self._auth_done and not self._auth_done.done():
            self._auth_done.set_exception(RuntimeError("鉴权过程中手环断开"))
        if self.on_disconnect:
            self.on_disconnect()

    # ---------- 鉴权 ----------

    async def _authenticate(self):
        self._phone_nonce = os.urandom(16)
        self._auth_done = asyncio.get_running_loop().create_future()
        cmd = pb.Command(type=AUTH_TYPE, subtype=AUTH_CMD_NONCE)
        cmd.auth.phoneNonce.nonce = self._phone_nonce
        await self.send_command(cmd)
        await asyncio.wait_for(self._auth_done, 15)
        self._on_authed()
        log.info("鉴权成功")

    def _on_authed(self):
        pass

    def _handle_auth(self, cmd: pb.Command):
        if cmd.subtype == AUTH_CMD_NONCE:
            watch = cmd.auth.watchNonce
            crypto = Crypto(derive_keys(self.secret, self._phone_nonce, watch.nonce))
            if _hmac(crypto.dec_key, watch.nonce + self._phone_nonce) != watch.hmac:
                self._auth_done.set_exception(RuntimeError("auth key 不对（手环返回的 hmac 校验失败）"))
                return
            info = pb.AuthDeviceInfo(unknown1=0, phoneApiLevel=34, phoneName="Mac", unknown3=224, region="CN")
            reply = pb.Command(type=AUTH_TYPE, subtype=AUTH_CMD_AUTH)
            reply.auth.authStep3.encryptedNonces = _hmac(crypto.enc_key, self._phone_nonce + watch.nonce)
            reply.auth.authStep3.encryptedDeviceInfo = crypto.encrypt(info.SerializeToString(), 0)

            async def step3():
                await self.send_command(reply)

            self.crypto = crypto
            self.spawn(step3())
        elif cmd.subtype == AUTH_CMD_AUTH:
            self.authed = True
            if not self._auth_done.done():
                self._auth_done.set_result(True)
        else:
            log.warning("未知鉴权响应: %s", cmd)

    # ---------- 命令 ----------

    def on_command_bytes(self, data: bytes):
        cmd_type, cmd_id = _peek_type_id(data)
        for w in list(self._waiters):
            if w[0] == cmd_type and w[1] == cmd_id and not w[2].done():
                w[2].set_result(data)
                self._waiters.remove(w)
                return
        handler = self.packet_handlers.get(cmd_type)
        if handler:
            handler(data)
            return
        cmd = pb.Command()
        try:
            cmd.ParseFromString(data)
        except Exception:
            log.warning("解析命令失败: %s", data.hex())
            return
        if cmd.type == AUTH_TYPE:
            self._handle_auth(cmd)
        else:
            log.debug("手环命令 type=%d subtype=%d", cmd.type, cmd.subtype)

    async def set_time(self):
        now = time.localtime()
        cmd = pb.Command(type=SYSTEM_TYPE, subtype=SYSTEM_CMD_SET_TIME)
        clock = cmd.system.clock
        clock.date.year, clock.date.month, clock.date.day = now.tm_year, now.tm_mon, now.tm_mday
        clock.time.hour, clock.time.minute, clock.time.second = now.tm_hour, now.tm_min, now.tm_sec
        clock.time.millisecond = 0
        offset = now.tm_gmtoff // 60 // 15
        clock.timezone.zoneOffset = offset
        clock.timezone.dstOffset = 0
        clock.timezone.name = os.path.realpath("/etc/localtime").split("zoneinfo/")[-1]
        clock.isNot24hour = False
        await self.send_command(cmd)

    async def notify(self, title: str, body: str, app_name: str = "herdr", nid: int | None = None):
        cmd = pb.Command(type=NOTIFICATION_TYPE, subtype=NOTIFICATION_CMD_SEND)
        n = cmd.notification.notification2.notification3
        n.package = "dev.herdr.agent"
        n.appName = app_name
        n.title = title
        n.body = body
        n.unknown4 = ""
        n.timestamp = time.strftime("%Y%m%dT%H%M%S")
        n.id = nid if nid is not None else int(time.time() * 1000) & 0x7FFFFFFF
        await self.send_command(cmd)


class MiBand8(XiaomiDevice):
    """手环 8：BLE，服务 fe95。"""

    def __init__(self, auth_key: str, address: str | None = None):
        super().__init__(auth_key, address)
        self.client: BleakClient | None = None
        self.max_write = 244
        self.channels: dict[str, Channel] = {}

    @staticmethod
    async def find(timeout: float = 15) -> str | None:
        dev = await BleakScanner.find_device_by_filter(
            lambda d, adv: (d.name or adv.local_name or "").startswith(NAME_PREFIX),
            timeout=timeout,
        )
        return dev.address if dev else None

    @property
    def connected(self) -> bool:
        return bool(self.client and self.client.is_connected and self.authed)

    async def connect(self):
        if not self.address:
            log.info("扫描 %s ...", NAME_PREFIX)
            self.address = await self.find()
            if not self.address:
                raise RuntimeError("没扫到手环：确认手环没被手机连着（关掉手机蓝牙），并且就在旁边")
        log.info("连接 %s", self.address)
        self.crypto = None
        self.authed = False
        self.client = BleakClient(self.address, disconnected_callback=self._on_disconnected)
        await self.client.connect()
        self.max_write = max(23, (self.client.mtu_size or 247)) - 3
        log.info("已连接，单包上限 %d 字节", self.max_write)

        self.channels = {}
        for uuid in (UUID_CMD_WRITE, UUID_CMD_READ, UUID_ACTIVITY, UUID_UPLOAD):
            if self.client.services.get_characteristic(uuid) is None:
                if uuid in (UUID_CMD_WRITE, UUID_CMD_READ):
                    raise RuntimeError(f"手环上没有特征 {uuid}，不是小米手环 8 协议？")
                continue
            ch = Channel(self, uuid)
            self.channels[uuid] = ch
            await self.client.start_notify(uuid, lambda _c, d, ch=ch: ch.on_notify(bytes(d)))

        await self._authenticate()
        await self.set_time()

    async def disconnect(self):
        if self.client:
            await self.client.disconnect()

    def _on_authed(self):
        for ch in self.channels.values():
            ch.encrypted_index = 1  # 0 已被鉴权用掉

    async def send_raw(self, cmd_type: int, data: bytes):
        await self.channels[UUID_CMD_WRITE].write(data)
