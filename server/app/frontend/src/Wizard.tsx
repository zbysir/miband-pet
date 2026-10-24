import {useCallback, useEffect, useState} from 'react'
import type {ReactNode} from 'react'
import {Check, Loader2} from 'lucide-react'
import {InstallApp, SaveSetup} from '../wailsjs/go/main/App'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {LoginPanel} from './Login'
import {Pet} from './Pet'
import type {AppState} from './types'

const STEPS = ['准备', '登录小米账号', '连接手环', '安装手环应用', '完成']

// 连接向导：新手环（或换手环、重新绑定后）从头走一遍。
// 连接本身由后台服务负责（经典蓝牙连不上会自动扫 BLE），向导只做引导和显示进度。
export function Wizard({state, onFinish}: { state: AppState, onFinish: () => void }) {
    const [step, setStep] = useState(0)
    const next = useCallback(() => setStep(s => Math.min(s + 1, STEPS.length - 1)), [])

    return (
        <div className="flex h-screen flex-col items-center overflow-y-auto px-8 pt-14 pb-8">
            <div className="drag fixed inset-x-0 top-0 z-10 h-11 bg-background"/>
            <ol className="mb-8 flex flex-wrap justify-center gap-x-5 gap-y-2 text-sm">
                {STEPS.map((name, i) => (
                    <li key={name} className={`flex items-center gap-2 ${i === step ? 'font-medium text-foreground' : 'text-muted-foreground'}`}>
                        <span className={`flex size-5 items-center justify-center rounded-full text-xs ${
                            i < step ? 'bg-emerald-500 text-white' : i === step ? 'bg-primary text-primary-foreground' : 'bg-muted'}`}>
                            {i < step ? <Check className="size-3"/> : i + 1}
                        </span>
                        {name}
                    </li>
                ))}
            </ol>
            <div className="flex w-full max-w-md flex-col items-center gap-4 text-center">
                {step === 0 && <Prepare onNext={next}/>}
                {step === 1 && <Login state={state} onNext={next}/>}
                {step === 2 && <Connect state={state} onNext={next}/>}
                {step === 3 && <InstallApps state={state} onNext={next}/>}
                {step === 4 && <Done state={state} onFinish={onFinish}/>}
            </div>
            {step < 4 && state.configured && (
                <Button variant="link" className="mt-6 text-muted-foreground" onClick={onFinish}>退出向导</Button>
            )}
        </div>
    )
}

function Title({children}: { children: ReactNode }) {
    return <h1 className="text-xl font-semibold">{children}</h1>
}

function Muted({children}: { children: ReactNode }) {
    return <p className="text-sm text-muted-foreground">{children}</p>
}

function Error({children}: { children: ReactNode }) {
    return <p className="text-sm text-destructive">{children}</p>
}

function Prepare({onNext}: { onNext: () => void }) {
    return (
        <>
            <Pet mood="sleep" size={112}/>
            <Title>借用一下手机，完成一次初始化</Title>
            <Muted>
                手环的连接密钥是在手机上绑定时生成的，所以需要先用手机绑定一次。
                之后就完全不用手机了，日常由 Mac 直接连手环。
            </Muted>
            <div className="flex w-full flex-col gap-2 rounded-lg border p-4 text-left text-sm">
                <p><b>已经绑定过？</b>手环在手机的小米运动健康里能看到，直接下一步。</p>
                <p><b>还没绑定？</b>在手机上打开小米运动健康 → 添加设备 → 按提示绑定这只手环，完成后回来点下一步。</p>
            </div>
            <Button onClick={onNext}>已经绑定过了，下一步</Button>
        </>
    )
}

function Login({state, onNext}: { state: AppState, onNext: () => void }) {
    const [manual, setManual] = useState(false)
    const [key, setKey] = useState('')
    const [address, setAddress] = useState('')
    const [err, setErr] = useState('')

    const saveManual = () => SaveSetup(key, address).then(onNext).catch(e => setErr(String(e)))

    return (
        <>
            <Title>登录小米账号</Title>
            <Muted>登录绑定手环用的小米账号，App 会从账号里找到这只手环，自动完成连接设置。</Muted>
            {!manual && <LoginPanel tryCache onDone={onNext}/>}
            <Button variant="link" onClick={() => setManual(v => !v)}>{manual ? '改用扫码登录' : '手动填写（高级）'}</Button>
            {manual && (
                <div className="flex w-full flex-col gap-3 text-left">
                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor="key">手环密钥（auth key）</Label>
                        <Input id="key" value={key} onChange={e => setKey(e.target.value)} placeholder="0x0123…（32 位十六进制）" spellCheck={false}/>
                    </div>
                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor="address">手环蓝牙地址</Label>
                        <Input id="address" value={address} onChange={e => setAddress(e.target.value)} placeholder="AA:BB:CC:DD:EE:FF" spellCheck={false}/>
                    </div>
                    {err && <Error>{err}</Error>}
                    <Button onClick={saveManual} disabled={!key || !address}>保存</Button>
                </div>
            )}
            {state.setupError && <Error>{state.setupError}</Error>}
        </>
    )
}

function Connect({state, onNext}: { state: AppState, onNext: () => void }) {
    // 连上后停一下再进下一步，让用户看到“已连上”
    useEffect(() => {
        if (!state.connected) return
        const t = setTimeout(onNext, 1500)
        return () => clearTimeout(t)
    }, [state.connected, onNext])

    return (
        <>
            <Title>让手环连上这台 Mac</Title>
            <ol className="flex list-decimal flex-col gap-3 pl-5 text-left text-sm">
                <li>在<b>手机的「设置 → 蓝牙」</b>里找到这只手环，点<b>「取消配对 / 忽略此设备」</b>，这样手机就不会再连上来。
                    <span className="mt-1 block text-xs text-muted-foreground">只在系统蓝牙里取消，不要在小米运动健康里解绑：解绑后要重新绑定。</span></li>
                <li>在手环的设置里找到 <b>「连接新手机」</b>，进入后手环会显示一个二维码，<b>停在这个界面</b>就行，不用扫。</li>
                <li>手环和 Mac 如果弹出<b>配对确认</b>，都点「配对 / 允许」。</li>
            </ol>
            <div className={`flex items-center gap-2 rounded-lg border px-4 py-3 text-sm ${state.connected ? 'border-emerald-500/50 text-emerald-500' : ''}`}>
                {state.connected ? (
                    <><Check className="size-4"/>已连上手环{state.transport === 'ble' ? '（蓝牙低功耗）' : ''}</>
                ) : (
                    <><Loader2 className="size-4 animate-spin"/>正在寻找手环…</>
                )}
            </div>
            {!state.connected && state.lastError && (
                <Muted>还没连上，App 会每隔几秒自动重试。如果一直连不上，试试重启手环后再进入「连接新手机」。</Muted>
            )}
        </>
    )
}

function InstallApps({state, onNext}: { state: AppState, onNext: () => void }) {
    const installed = new Set(state.packages ?? [])
    const [err, setErr] = useState('')
    const builtin = (state.apps ?? []).filter(a => a.builtin)
    const missing = builtin.filter(a => !installed.has(a.package))

    const installAll = async () => {
        setErr('')
        try {
            for (const a of missing) await InstallApp(a.name)
            onNext()
        } catch (e) {
            setErr(String(e))
        }
    }

    return (
        <>
            <Pet mood="work" size={112} skin={state.settings?.pet}/>
            <Title>把 app 装到手环上</Title>
            <Muted>每个 app 在手环上是一个单独的应用，装一个要十秒左右。以后在 App 里也能装、能更新。</Muted>
            {builtin.map(a => (
                <div key={a.name} className="flex w-full items-center justify-between rounded-lg border px-4 py-3 text-sm">
                    <span>{a.title}</span>
                    <span className="text-xs text-muted-foreground">
                        {state.installing === a.name
                            ? `安装中 ${Math.round(state.installProgress * 100)}%`
                            : installed.has(a.package) ? '已安装' : '待安装'}
                    </span>
                </div>
            ))}
            {err && <Error>{err}</Error>}
            {missing.length > 0 ? (
                <Button onClick={installAll} disabled={!!state.installing || !state.connected}>
                    {state.installing ? '安装中…' : '安装'}
                </Button>
            ) : (
                <Button onClick={onNext}>下一步</Button>
            )}
            {missing.length > 0 && <Button variant="link" className="text-muted-foreground" onClick={onNext}>跳过</Button>}
        </>
    )
}

function Done({state, onFinish}: { state: AppState, onFinish: () => void }) {
    return (
        <>
            <Pet mood="celebrate" size={112} skin={state.settings?.pet}/>
            <Title>好了！</Title>
            <Muted>
                手环现在由这台 Mac 连着，手机已经取消了配对，不会再连上来，手机蓝牙可以照常使用。
                在手环上打开「Agent 猫」，就能看到 agent 的状态。关掉窗口后，wristkit 会留在菜单栏继续工作。
            </Muted>
            <Button onClick={onFinish}>开始使用</Button>
        </>
    )
}
