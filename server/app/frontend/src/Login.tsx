import {useCallback, useEffect, useState} from 'react'
import {CachedDevices, CancelLogin, SaveSetup, StartLogin} from '../wailsjs/go/main/App'
import {EventsOn} from '../wailsjs/runtime/runtime'
import {Button} from '@/components/ui/button'

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
            return <p className="text-sm text-muted-foreground">正在连接小米账号…</p>
        case 'idle':
            return <Button onClick={startQR}>扫码登录小米账号</Button>
        case 'qr':
            return (
                <div className="flex flex-col items-center gap-3">
                    <img src={phase.image} width={180} height={180} alt="登录二维码" className="rounded-lg bg-white p-2"/>
                    <p className="text-xs text-muted-foreground">用手机<b>相机</b>或系统<b>「扫一扫」</b>扫码（小米手机直接用相机），然后在手机上确认登录小米账号。不要用小米运动健康扫，它不认这个码。二维码 5 分钟内有效。</p>
                </div>
            )
        case 'error':
            return (
                <div className="flex flex-col items-center gap-3">
                    <p className="text-sm text-destructive">{phase.message}</p>
                    <Button variant="outline" onClick={startQR}>重新扫码</Button>
                </div>
            )
        case 'devices': {
            const bands = phase.devices.filter(d => d.authKey)
            if (bands.length === 0) {
                return <p className="text-sm text-destructive">账号下没找到手环。先在手机上用小米运动健康绑定一次手环。</p>
            }
            return (
                <div className="flex w-full flex-col gap-2">
                    {bands.map(d => (
                        <div key={d.mac} className="flex items-center justify-between rounded-lg border px-4 py-3 text-left">
                            <div>
                                <div className="text-sm font-medium">{d.name}</div>
                                <div className="text-xs text-muted-foreground">{d.mac}</div>
                            </div>
                            <Button disabled={saving} onClick={() => use(d)}>
                                {saving ? '连接中…' : '使用这个手环'}
                            </Button>
                        </div>
                    ))}
                </div>
            )
        }
    }
}
