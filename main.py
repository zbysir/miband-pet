"""命令行入口。

  python main.py scan                     扫描附近的小米手环
  python main.py notify 标题 正文          连上手环发一条通知
  python main.py watch                    常驻：herdr agent 状态变化时发通知到手环
  python main.py install [rpk]            把快应用装到手环（默认 band-app/dist 里的）
  python main.py app                      常驻：把 herdr agent 列表推给手环上的快应用

配置读 config.json：{"auth_key": "...", "transport": "spp|ble", "address": "可选，scan 得到的地址"}
  transport：手环 8 用 ble；10 / 10 Pro 用 ble2（SPP v2 帧走 BLE）；8 Pro / 9 / 9 Pro 用 spp（默认）
"""

import argparse
import asyncio
import json
import logging
import os
import sys
import time

from bleak import BleakScanner

from miband import MiBand8, XiaomiDevice

HERE = os.path.dirname(os.path.abspath(__file__))
CONFIG_PATH = os.path.join(HERE, "config.json")

POLL_SECONDS = 3
RECONNECT_SECONDS = 15

log = logging.getLogger("main")


def load_config() -> dict:
    try:
        with open(CONFIG_PATH) as f:
            cfg = json.load(f)
    except FileNotFoundError:
        sys.exit(f"缺少 {CONFIG_PATH}，参考 config.example.json 填 auth_key")
    if not cfg.get("auth_key"):
        sys.exit("config.json 里 auth_key 为空")
    return cfg


def make_band(cfg: dict) -> XiaomiDevice:
    if cfg.get("transport", "spp") == "ble":
        return MiBand8(cfg["auth_key"], cfg.get("address"))
    if cfg.get("transport") == "ble2":
        from spp import MiBandBle2
        return MiBandBle2(cfg["auth_key"], mac=cfg.get("address"))
    from spp import MiBandSpp
    return MiBandSpp(cfg["auth_key"], cfg.get("address"))


async def cmd_scan(_args):
    from spp import find_paired
    paired = find_paired("")
    print("Mac 已配对的经典蓝牙设备（手环 9 Pro / 10 Pro 走这个）:")
    for d in paired:
        print(f"  {d.name():<32} {d.addressString()}  {'已连接' if d.isConnected() else ''}")
    print()
    print("BLE 扫描 10 秒（手环 8 走这个）...")
    found = await BleakScanner.discover(timeout=10, return_adv=True)
    rows = []
    for dev, adv in found.values():
        name = dev.name or adv.local_name or ""
        if "xiaomi" in name.lower() or "band" in name.lower() or "0000fe95-0000-1000-8000-00805f9b34fb" in adv.service_uuids:
            rows.append((adv.rssi, name, dev.address))
    if not rows:
        print("没找到。确认手机已关蓝牙（手环被手机连着时不广播），手环在旁边且亮屏。")
        return
    for rssi, name, addr in sorted(rows, reverse=True):
        print(f"{rssi:>4} dBm  {name:<32} {addr}")


async def cmd_notify(args):
    cfg = load_config()
    band = make_band(cfg)
    await band.connect()
    try:
        await band.notify(args.title, args.body)
        await asyncio.sleep(1)  # 给手环一点时间展示，再断开
        print("已发送")
    finally:
        await band.disconnect()


# ---------------- herdr ----------------

STATUS_TEXT = {"done": "完成", "blocked": "等你处理", "idle": "完成"}


async def herdr_agents() -> dict[str, dict]:
    proc = await asyncio.create_subprocess_exec(
        "herdr", "agent", "list",
        stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE,
    )
    out, err = await proc.communicate()
    if proc.returncode != 0:
        raise RuntimeError(f"herdr agent list 失败: {err.decode().strip()}")
    agents = json.loads(out)["result"]["agents"]
    return {a["pane_id"]: a for a in agents}


def agent_label(a: dict) -> str:
    title = a.get("name") or a.get("terminal_title_stripped") or a["pane_id"]
    project = os.path.basename(a.get("cwd", "").rstrip("/"))
    return f"{project} · {title}" if project else title


def events(prev: dict[str, dict], cur: dict[str, dict]) -> list[tuple[str, str]]:
    """只在“从 working 变走”或“变成 blocked”时提醒，避免刷屏。"""
    out = []
    for pane, a in cur.items():
        old = prev.get(pane)
        if not old:
            continue
        before, after = old.get("agent_status"), a.get("agent_status")
        if before == after:
            continue
        if after == "blocked" or (before == "working" and after in ("done", "idle")):
            out.append((f"{a.get('agent', 'agent')} {STATUS_TEXT.get(after, after)}", agent_label(a)))
    return out


class Watcher:
    def __init__(self, cfg: dict):
        self.band = make_band(cfg)
        self.queue: asyncio.Queue[tuple[str, str]] = asyncio.Queue()

    async def poll_herdr(self):
        prev: dict[str, dict] = {}
        while True:
            try:
                cur = await herdr_agents()
                for ev in events(prev, cur):
                    log.info("事件: %s / %s", *ev)
                    self.queue.put_nowait(ev)
                prev = cur
            except Exception as e:
                log.warning("读取 herdr 失败: %s", e)
            await asyncio.sleep(POLL_SECONDS)

    async def ensure_connected(self):
        while not self.band.connected:
            try:
                await self.band.connect()
                working = [a for a in (await herdr_agents()).values() if a.get("agent_status") == "working"]
                await self.band.notify("herdr 已连接", f"{len(working)} 个 agent 正在运行")
            except Exception as e:
                log.warning("连接手环失败: %s，%ds 后重试", e, RECONNECT_SECONDS)
                try:
                    await self.band.disconnect()
                except Exception:
                    pass
                await asyncio.sleep(RECONNECT_SECONDS)

    async def deliver(self):
        while True:
            await self.ensure_connected()
            title, body = await self.queue.get()
            try:
                await self.band.notify(title, body)
            except Exception as e:
                log.warning("发送失败，重连后补发: %s", e)
                self.queue.put_nowait((title, body))
                try:
                    await self.band.disconnect()
                except Exception:
                    pass

    async def run(self):
        await asyncio.gather(self.poll_herdr(), self.deliver())


async def cmd_watch(_args):
    await Watcher(load_config()).run()


# ---------------- 手环快应用 ----------------

# 手环上接收 agent 列表的快应用：列表版和宠物版，装了哪个就推给哪个
APP_PACKAGES = ["dev.bysir.herdrband", "dev.bysir.herdrpet"]
APP_RPK = os.path.join(HERE, "pet-app", "dist", "dev.bysir.herdrpet.debug.1.0.0.rpk")


def agents_snapshot(agents: dict[str, dict]) -> dict:
    """推给快应用的数据；字段要精简，整条消息要能装进一个数据帧。"""
    out = []
    for a in agents.values():
        out.append({
            "id": a["pane_id"],
            "agent": a.get("agent"),
            "project": os.path.basename(a.get("cwd", "").rstrip("/")),
            "title": a.get("name") or a.get("terminal_title_stripped") or "",
            "status": a.get("agent_status"),
        })
    return {"t": "agents", "ts": int(time.time() * 1000), "agents": out}


class AppBridge:
    """常驻：连着手环，herdr 状态一变就把全量列表推给快应用；快应用打开时发 hello 也推一次。

    有 agent 跑完或卡住时，再额外发一条通知让手环震动（亮屏与否看手环上「通知亮屏」的设置）；
    config.json 里 "notify": false 可关。
    """

    def __init__(self, cfg: dict):
        from vela import VelaApps

        self.band = make_band(cfg)
        self.apps = VelaApps(self.band)
        self.apps.on_message = self._on_app_message
        self.latest: dict | None = None
        self.dirty = asyncio.Event()
        self.packages: list[str] = []
        self.notify = cfg.get("notify", True)
        self.notes: list[tuple[str, str]] = []

    def _on_app_message(self, pkg: str, msg):
        if isinstance(msg, dict) and msg.get("t") == "hello":
            self.dirty.set()

    async def poll_herdr(self):
        last_key = None
        prev: dict[str, dict] = {}
        while True:
            try:
                raw = await herdr_agents()
                if self.notify:
                    for ev in events(prev, raw):
                        log.info("事件: %s / %s", *ev)
                        self.notes.append(ev)
                prev = raw
                snap = agents_snapshot(raw)
                key = json.dumps([(a["id"], a["status"], a["title"]) for a in snap["agents"]])
                self.latest = snap
                if key != last_key:
                    last_key = key
                    self.dirty.set()
            except Exception as e:
                log.warning("读取 herdr 失败: %s", e)
            await asyncio.sleep(POLL_SECONDS)

    async def ensure_connected(self):
        while not self.band.connected:
            try:
                await self.band.connect()
                installed = {a.package_name for a in await self.apps.list_apps()}
                self.packages = [p for p in APP_PACKAGES if p in installed]
                log.info("手环上的快应用：%s", self.packages or "一个都没装")
                for pkg in self.packages:
                    await self.apps.set_status(pkg, True)
                self.dirty.set()
            except Exception as e:
                log.warning("连接手环失败: %s，%ds 后重试", e, RECONNECT_SECONDS)
                try:
                    await self.band.disconnect()
                except Exception:
                    pass
                await asyncio.sleep(RECONNECT_SECONDS)

    async def deliver(self):
        while True:
            await self.ensure_connected()
            try:
                await asyncio.wait_for(self.dirty.wait(), 60)
            except asyncio.TimeoutError:
                continue  # 定期回到 ensure_connected 检查连接
            self.dirty.clear()
            if not self.latest:
                continue
            try:
                self.latest["ts"] = int(time.time() * 1000)
                for pkg in self.packages:
                    await self.apps.send(pkg, self.latest)
                log.info("推送 %d 个 agent", len(self.latest["agents"]))
                await self._send_notes()
            except Exception as e:
                log.warning("推送失败，重连后重推: %s", e)
                self.dirty.set()
                try:
                    await self.band.disconnect()
                except Exception:
                    pass

    async def _send_notes(self):
        """攒下的事件合成一条通知发出去（同一轮里多个 agent 一起变化时不连震好几下）。"""
        if not self.notes:
            return
        notes, self.notes = self.notes, []
        if len(notes) == 1:
            title, body = notes[0]
        else:
            title = f"{len(notes)} 个 agent 有变化"
            body = "\n".join(f"{t}：{b}" for t, b in notes)
        await self.band.notify(title, body)

    async def run(self):
        await asyncio.gather(self.poll_herdr(), self.deliver())


async def cmd_app(_args):
    await AppBridge(load_config()).run()


async def cmd_install(args):
    from vela import VelaApps, rpk_manifest

    band = make_band(load_config())
    apps = VelaApps(band)
    await band.connect()
    try:
        await apps.install(args.rpk, progress=lambda i, n: print(f"\r传输 {i}/{n}", end="", flush=True))
        print()
        await apps.launch(rpk_manifest(args.rpk)["package"])
        print("已安装并打开")
    finally:
        await band.disconnect()


def main():
    p = argparse.ArgumentParser(description="Mac 直连小米手环 8")
    p.add_argument("-v", "--verbose", action="store_true")
    sub = p.add_subparsers(dest="cmd", required=True)
    sub.add_parser("scan").set_defaults(func=cmd_scan)
    n = sub.add_parser("notify")
    n.add_argument("title")
    n.add_argument("body", nargs="?", default="")
    n.set_defaults(func=cmd_notify)
    sub.add_parser("watch").set_defaults(func=cmd_watch)
    sub.add_parser("app").set_defaults(func=cmd_app)
    ins = sub.add_parser("install")
    ins.add_argument("rpk", nargs="?", default=APP_RPK)
    ins.set_defaults(func=cmd_install)
    args = p.parse_args()

    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(asctime)s %(name)s %(levelname)s %(message)s",
    )
    logging.getLogger("bleak").setLevel(logging.INFO)
    try:
        asyncio.run(args.func(args))
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
