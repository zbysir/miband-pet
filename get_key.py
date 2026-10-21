"""扫码登录小米账号，从小米健康云拉手环的 auth key，写进 config.json。

huami-token 的账号密码登录会被小米的二次验证拦下（"Missing ssecurity or location"），
这里改用扫码登录（account.xiaomi.com/longPolling），登录后复用 huami-token 的加密请求去拉设备列表。
"""

import json
import os
import subprocess
import sys
import time

import requests
from huami_token.xiaomi import XiaomiClient, XiaomiSession, _parse_xiaomi_json

HERE = os.path.dirname(os.path.abspath(__file__))
CONFIG_PATH = os.path.join(HERE, "config.json")
QR_PATH = os.path.join(HERE, "login_qr.png")
SESSION_PATH = os.path.join(HERE, ".xiaomi_session.json")  # 缓存登录态，避免反复扫码

UA = "Mozilla/5.0 (Linux; Android 12; Pixel 4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0 Mobile Safari/537.36"


def qr_login() -> XiaomiSession:
    s = XiaomiSession(username="qr", password="")
    http = requests.Session()
    http.headers["User-Agent"] = UA

    r = http.get(
        "https://account.xiaomi.com/pass/serviceLogin",
        params={"_json": "true", "sid": "miothealth", "_locale": "zh_CN"},
        cookies={"deviceId": s.device_id},
    )
    page = _parse_xiaomi_json(r.text)

    r = http.get(
        "https://account.xiaomi.com/longPolling/loginUrl",
        params={
            "_qrsize": "480",
            "qs": page["qs"],
            "bizDeviceType": "",
            "callback": page["callback"],
            "_json": "true",
            "theme": "",
            "sid": "miothealth",
            "needTheme": "false",
            "showActiveX": "false",
            "serviceParam": page.get("serviceParam", ""),
            "_locale": "zh_CN",
            "_sign": page["_sign"],
            "_dc": str(int(time.time() * 1000)),
        },
    )
    qr = _parse_xiaomi_json(r.text)
    if "qr" not in qr or "lp" not in qr:
        sys.exit(f"拿二维码失败: {qr}")

    with open(QR_PATH, "wb") as f:
        f.write(http.get(qr["qr"]).content)
    subprocess.run(["open", QR_PATH])
    print(f"二维码已打开（{QR_PATH}），用手机上登录了同一个小米账号的「小米运动健康」或系统扫一扫扫码确认。", flush=True)

    deadline = time.time() + 300
    data = None
    while time.time() < deadline:
        try:
            r = http.get(qr["lp"], timeout=60)
        except requests.exceptions.Timeout:
            continue
        data = _parse_xiaomi_json(r.text)
        if data.get("ssecurity"):
            break
        print("等待扫码...", data.get("desc") or data.get("code"), flush=True)
        time.sleep(1)
    else:
        sys.exit("5 分钟内没扫码，退出")
    os.remove(QR_PATH)

    s._ssecurity = data["ssecurity"]
    s._user_id = str(data.get("userId", ""))
    s._c_user_id = data.get("cUserId")
    s._nonce = data.get("nonce")
    s._location = data["location"]
    if s._nonce is not None:
        s._get_service_token()
    else:
        r = http.get(s._location)
        s._service_token = http.cookies.get("serviceToken") or r.cookies.get("serviceToken")
        if not s._service_token:
            sys.exit("登录成功但没拿到 serviceToken")
    print(f"登录成功，userId={s._user_id}", flush=True)
    return s


def load_session() -> XiaomiSession | None:
    try:
        with open(SESSION_PATH) as f:
            d = json.load(f)
    except FileNotFoundError:
        return None
    s = XiaomiSession(username="qr", password="")
    s._ssecurity, s._service_token, s._user_id, s._c_user_id = d["ssecurity"], d["service_token"], d["user_id"], d["c_user_id"]
    return s


def save_session(s: XiaomiSession):
    with open(SESSION_PATH, "w") as f:
        json.dump({"ssecurity": s._ssecurity, "service_token": s._service_token,
                   "user_id": s._user_id, "c_user_id": s._c_user_id}, f)
    os.chmod(SESSION_PATH, 0o600)


def mask(v) -> str:
    v = str(v)
    return v if len(v) <= 8 else f"{v[:4]}…{v[-4:]} (len {len(v)})"


def dump(obj, prefix=""):
    if isinstance(obj, str) and obj[:1] in "{[":
        try:
            obj = json.loads(obj)
        except ValueError:
            pass
    if isinstance(obj, dict):
        for k, v in obj.items():
            dump(v, f"{prefix}{k}.")
    elif isinstance(obj, list):
        for i, v in enumerate(obj):
            dump(v, f"{prefix}{i}.")
    else:
        print(f"    {prefix.rstrip('.')} = {mask(obj)}")


def main():
    session = load_session()
    if session is None:
        session = qr_login()
        save_session(session)
    sources = []
    for region in ("cn", "ru", "de", "sg", "us", "i2"):
        result = XiaomiClient(session, region=region).get_source_list()
        sources = (result.get("result") or {}).get("list") or []
        if sources:
            break
    if not sources:
        sys.exit(f"账号下没找到绑定的设备: {result}")

    bands = []
    for src in sources:
        detail = src.get("detail") or {}
        if isinstance(detail, str):
            detail = json.loads(detail)
        name = (src.get("name") or "").strip()
        key = detail.get("auth_key") or detail.get("encrypt_key") or detail.get("token") or ""
        mac = detail.get("mac") or src.get("mac") or ""
        print(f"- {name}  MAC={mac}  key={'有' if key else '无'}")
        if not key:
            dump(src)
        if key:
            bands.append((name, mac, key))
    if not bands:
        sys.exit("设备列表里没有 auth_key")

    name, mac, key = next((b for b in bands if "band" in b[0].lower() or "手环" in b[0]), bands[0])
    cfg = {"auth_key": "", "transport": "spp", "address": ""}
    if os.path.exists(CONFIG_PATH):
        with open(CONFIG_PATH) as f:
            cfg.update(json.load(f))
    cfg["auth_key"] = "0x" + key
    cfg["address"] = mac
    with open(CONFIG_PATH, "w") as f:
        json.dump(cfg, f, indent=2, ensure_ascii=False)
    print(f"已把 {name} 的 key 和 MAC 写进 {CONFIG_PATH}")


if __name__ == "__main__":
    main()
