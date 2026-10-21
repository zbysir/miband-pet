import {useCallback, useEffect, useState} from 'react'
import {CachedDevices, CancelLogin, SaveSetup, StartLogin} from '../wailsjs/go/main/App'
import {EventsOn} from '../wailsjs/runtime/runtime'

interface Device {
    name: string
    mac: string
    authKey: string
    model: string
}

type Phase =
    | { kind: 'checking' }                 // 先用缓存的登录态试试
    | { kind: 'idle' }
    | { kind: 'qr', image: string }        // 等扫码
    | { kind: 'devices', devices: Device[] }
    | { kind: 'error', message: string }

// 扫码登录小米账号，从小米健康云拉手环的 auth key 和地址；选中后保存配置并连接。
export function LoginPanel({tryCache, onDone}: { tryCache: boolean, onDone: () => void }) {
    const [phase, setPhase] = useState<Phase>(tryCache ? {kind: 'checking'} : {kind: 'idle'})
    const [saving, setSaving] = useState(false)

    const startQR = useCallback(() => {
        setPhase({kind: 'checking'})
        StartLogin()
            .then(image => setPhase({kind: 'qr', image}))
            .catch(e => setPhase({kind: 'error', message: String(e)}))
    }, [])

    useEffect(() => {
        const off = EventsOn('login', (res: { devices: Device[] | null, error: string }) => {
            if (res.error) setPhase({kind: 'error', message: res.error})
            else setPhase({kind: 'devices', devices: res.devices ?? []})
        })
        if (tryCache) {
            CachedDevices()
                .then(devices => setPhase({kind: 'devices', devices: (devices ?? []) as Device[]}))
                .catch(() => startQR()) // 没登录过或登录态过期 → 扫码
        }
        return () => {
            off()
            CancelLogin()
        }
    }, [tryCache, startQR])

    const use = (d: Device) => {
        setSaving(true)
        SaveSetup(d.authKey, d.mac)
            .then(onDone)
            .catch(e => setPhase({kind: 'error', message: String(e)}))
            .finally(() => setSaving(false))
    }

    switch (phase.kind) {
        case 'checking':
            return <p className="muted">正在连接小米账号…</p>
        case 'idle':
            return <button className="btn btn-primary" onClick={startQR}>扫码登录小米账号</button>
        case 'qr':
            return (
                <div className="login-qr">
                    <img src={phase.image} width={180} height={180} alt="登录二维码"/>
                    <p className="muted small">用手机上登录了同一小米账号的「小米运动健康」或系统「扫一扫」扫码，在手机上确认登录。二维码 5 分钟内有效。</p>
                </div>
            )
        case 'error':
            return (
                <div>
                    <p className="error">{phase.message}</p>
                    <button className="btn" onClick={startQR}>重新扫码</button>
                </div>
            )
        case 'devices': {
            const bands = phase.devices.filter(d => d.authKey)
            if (bands.length === 0) {
                return <p className="error">账号下没找到带 key 的设备。先在手机上用小米运动健康绑定一次手环。</p>
            }
            return (
                <div className="device-list">
                    {bands.map(d => (
                        <div key={d.mac} className="device">
                            <div>
                                <div>{d.name}</div>
                                <div className="muted small">{d.mac}</div>
                            </div>
                            <button className="btn btn-primary" disabled={saving} onClick={() => use(d)}>
                                {saving ? '连接中…' : '使用这个手环'}
                            </button>
                        </div>
                    ))}
                </div>
            )
        }
    }
}
