"""手环上的 Vela 快应用：列出/安装/启动，以及跟快应用互发消息（快应用侧是 @system.interconnect）。

协议参考 OronBox（lib/src/device/xiaomi/components/thirdparty_app_system.dart）：
- 命令 type 20（THIRDPARTY_APP），id 见 vela.proto 的 ThirdpartyAppID
- 手机→快应用：SEND_PHONE_MESSAGE；快应用→手机：SEND_WEAR_MESSAGE
- 消息里要带快应用的包名 + 签名指纹（指纹从 GET_INSTALLED_LIST 拿）
- 快应用的 interconnect 要收到 SYNC_PHONE_APP_STATUS=CONNECTED 才会触发 onopen
"""

import asyncio
import hashlib
import json
import logging
import struct
import zipfile
import zlib

import vela_pb2 as vp
from miband import XiaomiDevice

log = logging.getLogger("miband.vela")

TYPE_THIRDPARTY_APP = 20
TYPE_MASS = 22
MASS_DATA_TYPE_APP = 64
T = vp.ThirdpartyApp


def rpk_manifest(path: str) -> dict:
    """rpk 是 zip，根目录有 manifest.json。"""
    with zipfile.ZipFile(path) as z:
        return json.loads(z.read("manifest.json"))


def _packet(app_id: int) -> vp.VelaPacket:
    return vp.VelaPacket(type=TYPE_THIRDPARTY_APP, id=app_id)


class VelaApps:
    def __init__(self, band: XiaomiDevice):
        self.band = band
        self.on_message = None  # (package, dict|bytes) -> None
        self._apps: dict[str, vp.AppItem] = {}
        band.packet_handlers[TYPE_THIRDPARTY_APP] = self._on_packet

    async def list_apps(self) -> list[vp.AppItem]:
        raw = await self.band.request(TYPE_THIRDPARTY_APP, T.GET_INSTALLED_LIST, _packet(T.GET_INSTALLED_LIST).SerializeToString())
        pkt = vp.VelaPacket()
        pkt.ParseFromString(raw)
        apps = list(pkt.thirdparty_app.app_item_list.list)
        self._apps = {a.package_name: a for a in apps}
        return apps

    async def _basic(self, package: str) -> vp.BasicInfo:
        if package not in self._apps:
            await self.list_apps()
        app = self._apps.get(package)
        if app is None:
            raise RuntimeError(f"手环上没装 {package}")
        return vp.BasicInfo(package_name=package, fingerprint=app.fingerprint)

    async def set_status(self, package: str, connected: bool = True):
        pkt = _packet(T.SYNC_PHONE_APP_STATUS)
        st = pkt.thirdparty_app.app_status
        st.basic_info.CopyFrom(await self._basic(package))
        st.status = vp.PhoneAppStatus.CONNECTED if connected else vp.PhoneAppStatus.DISCONNECTED
        await self.band.send_raw(TYPE_THIRDPARTY_APP, pkt.SerializeToString())

    async def send(self, package: str, data):
        """发给快应用；dict/list 会转成 JSON 字符串，快应用里 onmessage 的 data 就是这个字符串。"""
        if not isinstance(data, (bytes, str)):
            data = json.dumps(data, ensure_ascii=False, separators=(",", ":"))
        if isinstance(data, str):
            data = data.encode()
        pkt = _packet(T.SEND_PHONE_MESSAGE)
        mc = pkt.thirdparty_app.message_content
        mc.basic_info.CopyFrom(await self._basic(package))
        mc.content = data
        await self.band.send_raw(TYPE_THIRDPARTY_APP, pkt.SerializeToString())

    async def launch(self, package: str, uri: str = "pages/index"):
        pkt = _packet(T.LAUNCH_APP)
        pkt.thirdparty_app.launch_info.basic_info.CopyFrom(await self._basic(package))
        pkt.thirdparty_app.launch_info.uri = uri
        await self.band.send_raw(TYPE_THIRDPARTY_APP, pkt.SerializeToString())

    async def install(self, path: str, progress=None):
        """装 rpk：PREPARE_INSTALL_APP → MASS PREPARE → 分片传文件 → 等 REPORT_INSTALL_RESULT。"""
        data = open(path, "rb").read()
        manifest = rpk_manifest(path)
        package = manifest["package"]
        md5 = hashlib.md5(data).digest()
        log.info("安装 %s（%d 字节）", package, len(data))

        pkt = _packet(T.PREPARE_INSTALL_APP)
        req = pkt.thirdparty_app.install_request
        req.package_name, req.version_code, req.package_size = package, int(manifest.get("versionCode", 1)), len(data)
        resp = vp.VelaPacket()
        resp.ParseFromString(await self.band.request(TYPE_THIRDPARTY_APP, T.PREPARE_INSTALL_APP, pkt.SerializeToString()))
        status = resp.thirdparty_app.install_response.prepare_status
        if status != vp.READY:
            raise RuntimeError(f"手环拒绝安装：{vp.PrepareStatus.Name(status)}")

        pkt = vp.VelaPacket(type=TYPE_MASS, id=vp.Mass.PREPARE)
        pr = pkt.mass.prepare_request
        pr.data_type, pr.data_id, pr.data_length = MASS_DATA_TYPE_APP, md5, len(data)
        resp = vp.VelaPacket()
        resp.ParseFromString(await self.band.request(TYPE_MASS, vp.Mass.PREPARE, pkt.SerializeToString()))
        mp = resp.mass.prepare_response
        if mp.prepare_status != vp.READY:
            raise RuntimeError(f"手环拒绝接收文件：{vp.PrepareStatus.Name(mp.prepare_status)}")
        if mp.remained_data_length:
            log.warning("手环说已有 %d 字节（断点续传），这里不支持，仍发整个文件", mp.remained_data_length)
        slice_len = mp.expected_slice_length or 244

        # 内层：00 | 类型 | md5 | 长度 | 文件 | crc32(前面全部)
        inner = bytes([0, MASS_DATA_TYPE_APP]) + md5 + struct.pack("<I", len(data)) + data
        inner += struct.pack("<I", zlib.crc32(inner) & 0xFFFFFFFF)
        frag = slice_len - 6
        total = -(-len(inner) // frag)

        result = self.band.expect(TYPE_THIRDPARTY_APP, T.REPORT_INSTALL_RESULT)  # 先挂监听，结果可能来得很快
        for i in range(total):
            # 分片走 mass 通道（2）明文：02 01 | 总片数 | 序号(从 1 起) | 数据
            body = bytes([2, 1]) + struct.pack("<HH", total, i + 1) + inner[i * frag:(i + 1) * frag]
            await self.band.send_data(body, timeout=30)
            if progress:
                progress(i + 1, total)

        resp = vp.VelaPacket()
        resp.ParseFromString(await asyncio.wait_for(result, 60))
        code = resp.thirdparty_app.install_result.code
        if code != vp.AppInstallResult.INSTALL_SUCCESS:
            raise RuntimeError(f"安装失败：{vp.AppInstallResult.Code.Name(code)}")
        self._apps.pop(package, None)
        log.info("安装成功：%s", package)

    async def remove(self, package: str):
        pkt = _packet(T.REMOVE_APP)
        pkt.thirdparty_app.basic_info.CopyFrom(await self._basic(package))
        await self.band.send_raw(TYPE_THIRDPARTY_APP, pkt.SerializeToString())
        self._apps.pop(package, None)

    def _on_packet(self, raw: bytes):
        pkt = vp.VelaPacket()
        try:
            pkt.ParseFromString(raw)
        except Exception:
            log.warning("解析快应用消息失败: %s", raw.hex())
            return
        app = pkt.thirdparty_app
        if pkt.id == T.SEND_WEAR_MESSAGE:
            pkg = app.message_content.basic_info.package_name
            content = app.message_content.content
            try:
                payload = json.loads(content)
            except ValueError:
                payload = content
            log.info("快应用 %s 发来: %s", pkg, payload)
            if self.on_message:
                self.on_message(pkg, payload)
        elif app.HasField("basic_info"):
            # 快应用打开 interconnect 时会来问手机端在不在；两个参考实现都只看 payload 类型不看 id
            pkg = app.basic_info.package_name
            log.info("快应用 %s 询问手机端状态，回复在线", pkg)
            self.band.spawn(self.set_status(pkg, True))
        else:
            log.debug("快应用消息 id=%d: %s", pkt.id, app)
