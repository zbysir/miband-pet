import {useCallback, useEffect, useState} from 'react'
import {Install, SaveSetup} from '../wailsjs/go/main/App'
import {LoginPanel} from './Login'
import {Pet} from './Pet'
import type {AppState, BundledApp} from './types'

const STEPS = ['准备', '登录小米账号', '连接手环', '安装手环应用', '完成']

// 连接向导：新手环（或换手环、重新绑定后）从头走一遍。
// 连接本身由后台服务负责（经典蓝牙连不上会自动扫 BLE），向导只做引导和显示进度。
export function Wizard({state, bundled, onFinish}: { state: AppState, bundled: BundledApp[], onFinish: () => void }) {
    const [step, setStep] = useState(0)
    const next = useCallback(() => setStep(s => Math.min(s + 1, STEPS.length - 1)), [])

    return (
        <div className="app wizard">
            <div className="drag"/>
            <ol className="steps">
                {STEPS.map((name, i) => (
                    <li key={name} className={i === step ? 'current' : i < step ? 'done' : ''}>
                        <i>{i < step ? '✓' : i + 1}</i>{name}
                    </li>
                ))}
            </ol>
            <div className="wizard-body">
                {step === 0 && <Prepare onNext={next}/>}
                {step === 1 && <Login state={state} onNext={next}/>}
                {step === 2 && <Connect state={state} onNext={next}/>}
                {step === 3 && <InstallApps state={state} bundled={bundled} onNext={next}/>}
                {step === 4 && <Done state={state} onFinish={onFinish}/>}
            </div>
            {step < 4 && state.configured && (
                <button className="link wizard-close" onClick={onFinish}>退出向导</button>
            )}
        </div>
    )
}

function Prepare({onNext}: { onNext: () => void }) {
    return (
        <>
            <Pet mood="sleep" size={112}/>
            <h1>借用一下手机，完成一次初始化</h1>
            <p className="muted">
                手环的连接密钥是在手机上绑定时生成的，所以需要先用手机绑定一次。
                之后就完全不用手机了，日常由 Mac 直接连手环。
            </p>
            <div className="tips">
                <p><b>已经绑定过？</b>手环在手机的小米运动健康里能看到，直接下一步。</p>
                <p><b>还没绑定？</b>在手机上打开小米运动健康 → 添加设备 → 按提示绑定这只手环，完成后回来点下一步。</p>
            </div>
            <button className="btn btn-primary" onClick={onNext}>已经绑定过了，下一步</button>
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
            <h1>登录小米账号</h1>
            <p className="muted">登录绑定手环用的小米账号，App 会从账号里找到这只手环，自动完成连接设置。</p>
            {!manual && <LoginPanel tryCache onDone={onNext}/>}
            <button className="link" onClick={() => setManual(v => !v)}>{manual ? '改用扫码登录' : '手动填写（高级）'}</button>
            {manual && <>
                <label className="field">
                    <span>手环密钥（auth key）</span>
                    <input value={key} onChange={e => setKey(e.target.value)} placeholder="0x0123…（32 位十六进制）" spellCheck={false}/>
                </label>
                <label className="field">
                    <span>手环蓝牙地址</span>
                    <input value={address} onChange={e => setAddress(e.target.value)} placeholder="AA:BB:CC:DD:EE:FF" spellCheck={false}/>
                </label>
                {err && <p className="error">{err}</p>}
                <button className="btn btn-primary" onClick={saveManual} disabled={!key || !address}>保存</button>
            </>}
            {state.setupError && <p className="error">{state.setupError}</p>}
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
            <h1>让手环连上这台 Mac</h1>
            <ol className="howto">
                <li>在<b>手机的「设置 → 蓝牙」</b>里找到这只手环，点<b>「取消配对 / 忽略此设备」</b>，这样手机就不会再连上来。
                    <span className="muted small block">只在系统蓝牙里取消，不要在小米运动健康里解绑：解绑后要重新绑定。</span></li>
                <li>在手环的设置里找到 <b>「连接新手机」</b>，进入后手环会显示一个二维码，<b>停在这个界面</b>就行，不用扫。</li>
                <li>手环和 Mac 如果弹出<b>配对确认</b>，都点「配对 / 允许」。</li>
            </ol>
            <div className={`connect-status ${state.connected ? 'ok' : ''}`}>
                {state.connected ? (
                    <><i className="dot dot-done"/>已连上手环{state.transport === 'ble' ? '（蓝牙低功耗）' : ''}</>
                ) : (
                    <><span className="spinner"/>正在寻找手环…</>
                )}
            </div>
            {!state.connected && state.lastError && (
                <p className="muted small">还没连上，App 会每隔几秒自动重试。如果一直连不上，试试重启手环后再进入「连接新手机」。</p>
            )}
        </>
    )
}

function InstallApps({state, bundled, onNext}: { state: AppState, bundled: BundledApp[], onNext: () => void }) {
    const installed = new Set(state.packages ?? [])
    const [err, setErr] = useState('')
    const missing = bundled.filter(b => !installed.has(b.package))

    const installAll = async () => {
        setErr('')
        try {
            for (const b of missing) await Install(b.package)
            onNext()
        } catch (e) {
            setErr(String(e))
        }
    }

    return (
        <>
            <Pet mood="work" size={112} skin={state.settings?.pet}/>
            <h1>在手环上装上 Agent 猫</h1>
            <p className="muted">手环上会多出两个应用：「Agent 猫」用一只像素猫显示 agent 的整体状态，「Agents」列出每个 agent。</p>
            {bundled.map(b => (
                <div key={b.package} className="device">
                    <span>{b.name}</span>
                    <span className="muted small">
                        {state.installing === b.package
                            ? `安装中 ${Math.round(state.installProgress * 100)}%`
                            : installed.has(b.package) ? '已安装' : '待安装'}
                    </span>
                </div>
            ))}
            {err && <p className="error">{err}</p>}
            {missing.length > 0 ? (
                <button className="btn btn-primary" onClick={installAll} disabled={!!state.installing || !state.connected}>
                    {state.installing ? '安装中…' : '安装'}
                </button>
            ) : (
                <button className="btn btn-primary" onClick={onNext}>下一步</button>
            )}
            {missing.length > 0 && <button className="link" onClick={onNext}>跳过</button>}
        </>
    )
}

function Done({state, onFinish}: { state: AppState, onFinish: () => void }) {
    return (
        <>
            <Pet mood="celebrate" size={112} skin={state.settings?.pet}/>
            <h1>好了！</h1>
            <p className="muted">
                手环现在由这台 Mac 连着，手机已经取消了配对，不会再连上来，手机蓝牙可以照常使用。
                {state.packages?.includes('dev.bysir.herdrpet') && '在手环上打开「Agent 猫」，就能看到 agent 的状态。'}
                关掉窗口后，miband pet 会留在菜单栏继续工作。
            </p>
            <button className="btn btn-primary" onClick={onFinish}>开始使用</button>
        </>
    )
}
