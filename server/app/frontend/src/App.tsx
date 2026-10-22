import {useCallback, useEffect, useMemo, useState} from 'react'
import {BundledApps, CopyBuiltin, GetState, Install, Launch, OpenPluginDir, SetClaudeUsage, SetImpatient, SetKeepAwake, SetPet, ShowPlugin, TestNotify, Uninstall, UpdateSettings} from '../wailsjs/go/main/App'
import {EventsOn} from '../wailsjs/runtime/runtime'
import {Wizard} from './Wizard'
import {Pet, SKINS, useMood} from './Pet'
import type {Mood, Skin} from './Pet'
import type {Agent, AlertMode, AppState, BundledApp, Stats, Status, UsageSource} from './types'
import {STATUS_ORDER, STATUS_TEXT} from './types'

export default function App() {
    const [state, setState] = useState<AppState | null>(null)
    const [bundled, setBundled] = useState<BundledApp[]>([])
    const [toast, setToast] = useState('')
    const [wizard, setWizard] = useState(false)

    const refresh = useCallback(() => {
        GetState().then(raw => {
            const s = raw as unknown as AppState
            setState(prev => {
                if (!prev && !s.configured) setWizard(true) // 第一次打开、还没配置：走连接向导
                return s
            })
        })
    }, [])

    useEffect(() => {
        refresh()
        BundledApps().then(list => setBundled(list ?? []))
        const off = EventsOn('state', refresh)
        const t = setInterval(refresh, 30_000) // 兜底：更新“几分钟前”之类的时间
        return () => {
            off()
            clearInterval(t)
        }
    }, [refresh])

    const run = useCallback((label: string, fn: () => Promise<unknown>) => {
        fn().then(() => setToast(label + '：完成'))
            .catch(e => setToast(label + '失败：' + String(e)))
            .finally(() => setTimeout(() => setToast(''), 3000))
    }, [])

    if (!state) return <div className="app"/>
    if (wizard || !state.configured) {
        return <Wizard state={state} bundled={bundled} onFinish={() => setWizard(false)}/>
    }

    // 横向两栏：左边宠物和设置，右边 agent 列表；两栏各自滚动
    return (
        <div className="app layout">
            <div className="drag"/>
            <aside className="col col-side">
                <Hero state={state} run={run}/>
                <SettingsCard state={state} run={run}/>
                <BandApps state={state} bundled={bundled} run={run}/>
                <Plugins state={state} run={run}/>
                <footer className="footer">
                    <div className="footer-actions">
                        <button className="btn" onClick={() => run('测试通知', TestNotify)} disabled={!state.connected}>发测试通知</button>
                        <button className="btn" onClick={() => setWizard(true)}>换手环 / 重新绑定</button>
                    </div>
                </footer>
            </aside>
            <main className="col col-main">
                <Today stats={state.stats} usage={state.usage ?? []} claudeUsage={!!state.settings?.claudeUsage}/>
                <AgentList agents={state.agents ?? []} herdrError={state.herdrError}/>
                <RecentEvents state={state}/>
            </main>
            {toast && <div className="toast">{toast}</div>}
        </div>
    )
}

// ---------- 顶部：宠物 + 状态 ----------

const HEADLINE: Record<Mood, (n: Record<Status, number>) => string> = {
    alert: n => `${n.blocked} 个等你处理！`,
    celebrate: () => '搞定一个！',
    work: n => `${n.working} 个在忙`,
    sleep: n => (n.done > 0 ? '都完成啦' : '都在休息 zZ'),
}

function countBy(agents: Agent[]): Record<Status, number> {
    const n: Record<Status, number> = {blocked: 0, working: 0, done: 0, idle: 0, unknown: 0}
    for (const a of agents) n[a.status in n ? a.status : 'unknown']++
    return n
}

function Hero({state, run}: { state: AppState, run: (label: string, fn: () => Promise<unknown>) => void }) {
    const agents = state.agents ?? []
    const mood = useMood(agents)
    const counts = useMemo(() => countBy(agents), [agents])

    let pill = {cls: 'pill-ok', text: '手环已连接'}
    if (!state.connected) pill = state.lastError ? {cls: 'pill-bad', text: '连接失败，重试中'} : {cls: 'pill-wait', text: '正在连接手环…'}
    else if (state.foreground) {
        const page = state.plugins?.find(p => p.active)
        pill = {cls: 'pill-ok', text: page ? `手环上正在看${page.title}` : '手环上开着'}
    }

    return (
        <header className={`hero hero-${mood}`}>
            <Pet mood={mood} size={152} skin={state.settings?.pet}/>
            <div className="hero-text">
                <h1 className={`headline headline-${mood}`}>{HEADLINE[mood](counts)}</h1>
                <span className={`pill ${pill.cls}`} title={state.lastError}>{pill.text}</span>
                <div className="counts">
                    {(['blocked', 'working', 'done', 'idle'] as Status[]).map(s => (
                        <span key={s} className="count"><i className={`dot dot-${s}`}/>{counts[s]}</span>
                    ))}
                </div>
            </div>
            <SkinPicker current={state.settings?.pet ?? 'orange'} disabled={!!state.installing}
                        onPick={skin => run('换猫', () => SetPet(skin))}/>
        </header>
    )
}

// 选猫：Mac 界面和手环上马上换（手环上没有的帧会推过去）
function SkinPicker({current, disabled, onPick}: { current: Skin, disabled: boolean, onPick: (skin: Skin) => void }) {
    return (
        <div className="skins" role="radiogroup" aria-label="选一只猫">
            {SKINS.map(s => (
                <button key={s.value} className={`skin ${s.value === current ? 'active' : ''}`}
                        role="radio" aria-checked={s.value === current} title={s.label} disabled={disabled}
                        onClick={() => s.value !== current && onPick(s.value)}>
                    <img src={`/pet/${s.value}/sleep_0.png`} width={40} height={40} alt=""/>
                    <span>{s.label}</span>
                </button>
            ))}
        </div>
    )
}

// ---------- agent 列表 ----------

function AgentList({agents, herdrError}: { agents: Agent[], herdrError: string }) {
    const [showIdle, setShowIdle] = useState(false)
    const groups = useMemo(() => {
        const g: Partial<Record<Status, Agent[]>> = {}
        for (const a of agents) (g[a.status] ??= []).push(a)
        for (const list of Object.values(g)) list!.sort((x, y) => x.project.localeCompare(y.project))
        return g
    }, [agents])

    return (
        <section className="card">
            <h2>Agents <span className="muted">{agents.length}</span></h2>
            {herdrError && <p className="error">读取 herdr 失败：{herdrError}</p>}
            {agents.length === 0 && !herdrError && <p className="muted">没有 agent</p>}
            {STATUS_ORDER.map(status => {
                const list = groups[status]
                if (!list?.length) return null
                const folded = status === 'idle' && !showIdle
                return (
                    <div key={status} className="group">
                        <div className="group-head" onClick={status === 'idle' ? () => setShowIdle(v => !v) : undefined}>
                            <i className={`dot dot-${status}`}/>
                            <span>{STATUS_TEXT[status]}</span>
                            <span className="muted">{list.length}</span>
                            {status === 'idle' && <span className="muted fold">{folded ? '展开' : '收起'}</span>}
                        </div>
                        {!folded && list.map(a => (
                            <div key={a.id} className={`agent agent-${a.status}`}>
                                <div className="agent-main">
                                    <span className="agent-project">{a.project || '?'}</span>
                                    <span className="agent-kind">{a.agent}</span>
                                </div>
                                <div className="agent-title">{a.title}</div>
                            </div>
                        ))}
                    </div>
                )
            })}
        </section>
    )
}

// ---------- 今日：小鱼干和额度 ----------

const AGENT_NAME: Record<string, string> = {claude: 'Claude Code', codex: 'Codex'}

function resetIn(sec: number): string {
    const left = Math.max(0, sec - Date.now() / 1000)
    if (left >= 86400) return `${Math.floor(left / 86400)} 天后重置`
    const h = Math.floor(left / 3600), m = Math.floor((left % 3600) / 60)
    return h > 0 ? `${h} 小时 ${m} 分后重置` : `${m} 分钟后重置`
}

function Today({stats, usage, claudeUsage}: { stats: Stats | null, usage: UsageSource[], claudeUsage: boolean }) {
    if (!stats) return null
    const top = Math.max(1, ...stats.days.map(d => d.done))
    return (
        <section className="card">
            <h2>今日</h2>
            <div className="today">
                <img src="/fish.png" width={48} height={24} alt="" className="pixel"/>
                <div>
                    <div className="today-big">{stats.today.done} 条小鱼干</div>
                    <div className="muted small">完成 {stats.today.done} 个任务 · 喂了 {stats.today.fed} 条 · 还剩 {stats.fishLeft} 条</div>
                </div>
            </div>
            <div className="week">
                {stats.days.map((d, i) => (
                    <div key={d.date} className={`week-day ${i === stats.days.length - 1 ? 'today-col' : ''}`} title={`${d.date} 完成 ${d.done} 个`}>
                        <span className="week-num">{d.done || ''}</span>
                        <i style={{height: `${Math.max(2, Math.round(d.done / top * 48))}px`}}/>
                        <span className="week-label">{d.weekday}</span>
                    </div>
                ))}
            </div>
            {usage.flatMap(src => src.windows.map(w => (
                <div key={src.agent + w.name} className="quota">
                    <span className="quota-label">{AGENT_NAME[src.agent] ?? src.agent} {w.name}</span>
                    <span className="quota-bar"><i className={w.used >= 80 ? 'bad' : w.used >= 50 ? 'warn' : ''} style={{width: `${Math.min(100, w.used)}%`}}/></span>
                    <span className="quota-num">{Math.round(w.used)}%</span>
                    <span className="muted small">{w.resetsAt ? resetIn(w.resetsAt) : ''}</span>
                </div>
            )))}
            {!claudeUsage && <p className="muted small">要看 Claude Code 的额度，在左边「提醒」里打开「显示 Claude Code 额度」。</p>}
        </section>
    )
}

// ---------- 最近提醒 ----------

function RecentEvents({state}: { state: AppState }) {
    const events = (state.events ?? []).slice(0, 5)
    if (events.length === 0) return null
    return (
        <section className="card">
            <h2>最近提醒</h2>
            {events.map(e => (
                <div key={e.time + e.title} className="event">
                    <span className={`tag tag-${e.action}`}>{e.action === 'wake' ? '亮屏' : '通知'}</span>
                    <div className="event-text">
                        <div>{e.title}</div>
                        <div className="muted small">{e.body}</div>
                    </div>
                    <span className="muted small">{ago(e.time)}</span>
                </div>
            ))}
        </section>
    )
}

// ---------- 设置 ----------

const ALERT_MODES: { value: AlertMode, label: string, desc: string }[] = [
    {value: 'auto', label: '自动', desc: '手环上正开着猫或列表时只亮屏，否则发通知震动'},
    {value: 'wake', label: '只亮屏', desc: '把猫打开到前台，手环亮屏但不震动'},
    {value: 'notify', label: '只通知', desc: '发通知，手环震动'},
    {value: 'both', label: '都要', desc: '打开猫并发通知'},
]

// 保持亮屏的间隔：手环亮屏最长 60 秒，要比手环上设的亮屏时间短
const KEEP_AWAKE_SECS = [15, 30, 50]

// 卡住多久算太久（0 表示关）
const IMPATIENT_MINS = [0, 3, 5, 10]

function SettingsCard({state, run}: { state: AppState, run: (label: string, fn: () => Promise<unknown>) => void }) {
    const s = state.settings
    if (!s) return null
    const save = (notify: boolean, notifyStart: boolean, alert: AlertMode) =>
        run('保存设置', () => UpdateSettings(notify, notifyStart, alert))
    const mode = ALERT_MODES.find(m => m.value === s.alert) ?? ALERT_MODES[0]
    const keepAwake = (on: boolean, sec: number) => run('保存设置', () => SetKeepAwake(on, sec))

    return (
        <section className="card">
            <h2>提醒</h2>
            <Toggle label="agent 有变化时提醒" checked={s.notify} onChange={v => save(v, s.notifyStart, s.alert)}/>
            <Toggle label="开始跑也提醒" hint="关掉后只在跑完、卡住时提醒" checked={s.notifyStart} disabled={!s.notify}
                    onChange={v => save(s.notify, v, s.alert)}/>
            <div className={`segmented ${s.notify ? '' : 'disabled'}`}>
                {ALERT_MODES.map(m => (
                    <button key={m.value} className={m.value === s.alert ? 'active' : ''}
                            onClick={() => save(s.notify, s.notifyStart, m.value)} disabled={!s.notify}>
                        {m.label}
                    </button>
                ))}
            </div>
            <p className="muted small">{mode.desc}</p>
            <Toggle label="有 agent 等你回答时保持亮屏"
                    hint={`每隔 ${s.keepAwakeSec} 秒把猫打开一次，手环不熄屏。手环上的亮屏时间要比这个长（最长 60 秒）`}
                    checked={s.keepAwake} onChange={v => keepAwake(v, s.keepAwakeSec)}/>
            <div className={`segmented ${s.keepAwake ? '' : 'disabled'}`}>
                {KEEP_AWAKE_SECS.map(sec => (
                    <button key={sec} className={sec === s.keepAwakeSec ? 'active' : ''}
                            onClick={() => keepAwake(true, sec)} disabled={!s.keepAwake}>
                        每 {sec} 秒
                    </button>
                ))}
            </div>
            <div className="setting-label">agent 卡住多久算太久</div>
            <div className="segmented">
                {IMPATIENT_MINS.map(min => (
                    <button key={min} className={min === s.impatientMin ? 'active' : ''}
                            onClick={() => run('保存设置', () => SetImpatient(min))}>
                        {min === 0 ? '不管' : `${min} 分钟`}
                    </button>
                ))}
            </div>
            <p className="muted small">{s.impatientMin > 0 ? `卡住超过 ${s.impatientMin} 分钟，手环上的猫会着急，并再提醒你一次` : '卡多久都不再提醒'}</p>
            <Toggle label="显示 Claude Code 额度"
                    hint="Claude Code 只把额度告诉状态栏命令，所以要把它的状态栏换成一个小脚本：先把数据存一份，再交给你原来的状态栏命令，显示不变。关掉会原样还原（改之前会备份 ~/.claude/settings.json）。只有 Pro/Max 订阅才有额度。Codex 的额度不用设置。"
                    checked={s.claudeUsage} onChange={v => run(v ? '打开额度' : '关掉额度', () => SetClaudeUsage(v))}/>
        </section>
    )
}

function Toggle({label, hint, checked, disabled, onChange}: {
    label: string, hint?: string, checked: boolean, disabled?: boolean, onChange: (v: boolean) => void
}) {
    return (
        <label className={`toggle ${disabled ? 'disabled' : ''}`}>
            <span>
                {label}
                {hint && <span className="muted small block">{hint}</span>}
            </span>
            <input type="checkbox" checked={checked} disabled={disabled} onChange={e => onChange(e.target.checked)}/>
            <i className="switch"/>
        </label>
    )
}

// ---------- 手环快应用 ----------

function BandApps({state, bundled, run}: {
    state: AppState, bundled: BundledApp[], run: (label: string, fn: () => Promise<unknown>) => void
}) {
    const installed = new Set(state.packages ?? [])
    return (
        <section className="card">
            <h2>手环上的应用</h2>
            {bundled.length === 0 && <p className="muted small">这个 App 没有打包手环应用（用 make app 构建）</p>}
            {(state.legacy ?? []).map(pkg => (
                <div key={pkg} className="band-app">
                    <div>
                        <div>{LEGACY_NAMES[pkg] ?? pkg}<span className="muted small">（旧版）</span></div>
                        <div className="muted small">已经改成插件，不再推送，可以卸载</div>
                    </div>
                    <div className="band-app-actions">
                        <button className="btn" disabled={!state.connected || !!state.installing}
                                onClick={() => run('打开', () => Launch(pkg))}>在手环上打开</button>
                        <button className="btn" disabled={!state.connected || !!state.installing}
                                onClick={() => run('卸载', () => Uninstall(pkg))}>卸载</button>
                    </div>
                </div>
            ))}
            {bundled.map(b => {
                const has = installed.has(b.package)
                const busy = state.installing === b.package
                return (
                    <div key={b.package} className="band-app">
                        <div>
                            <div>{b.name}</div>
                            <div className="muted small">
                                {!state.connected ? '连接后显示' : has ? '已安装' : '未安装'} · v{b.versionCode}
                            </div>
                        </div>
                        <div className="band-app-actions">
                            {has && <button className="btn" disabled={!state.connected || busy}
                                            onClick={() => run('打开', () => Launch(b.package))}>在手环上打开</button>}
                            <button className="btn" disabled={!state.connected || !!state.installing}
                                    onClick={() => run(has ? '更新' : '安装', () => Install(b.package))}>
                                {busy ? `${Math.round(state.installProgress * 100)}%` : has ? '更新' : '安装'}
                            </button>
                        </div>
                        {busy && <div className="progress"><i style={{width: `${state.installProgress * 100}%`}}/></div>}
                    </div>
                )
            })}
        </section>
    )
}

const LEGACY_NAMES: Record<string, string> = {
    'dev.bysir.herdrpet': 'Agent 猫',
    'dev.bysir.herdrband': 'Agents',
}

// ---------- 插件 ----------

// 手环上一次显示一个插件，在这里切换；用户插件放在插件目录里，改了自动重新加载
function Plugins({state, run}: { state: AppState, run: (label: string, fn: () => Promise<unknown>) => void }) {
    const list = state.plugins ?? []
    return (
        <section className="card">
            <h2>插件 <span className="muted">{list.length}</span></h2>
            {list.map(p => (
                <div key={p.name} className="band-app">
                    <div>
                        <div>
                            {p.title}
                            {p.active && <span className="tag tag-wake plugin-tag">手环上</span>}
                        </div>
                        <div className="muted small">{p.builtin ? '内置' : shortPath(p.dir)}</div>
                        {p.error && <div className="error small plugin-error">{p.error}</div>}
                    </div>
                    <div className="band-app-actions">
                        {p.builtin && <button className="btn" title="把源码复制到插件目录，改了就替换内置的这个"
                                              onClick={() => run('复制', () => CopyBuiltin(p.name))}>复制出来改</button>}
                        {!(p.active && state.foreground) &&
                            <button className="btn" disabled={!state.connected || !state.packages?.length}
                                    onClick={() => run('切换', () => ShowPlugin(p.name))}>切换</button>}
                    </div>
                </div>
            ))}
            <p className="muted small">点「切换」在手环上显示这个插件（手环上没开着会先打开）。自己写插件：把插件文件夹放进插件目录，保存后自动生效。</p>
            <div className="footer-actions">
                <button className="btn" onClick={() => run('打开插件目录', OpenPluginDir)}>打开插件目录</button>
            </div>
        </section>
    )
}

// ---------- 工具 ----------

function ago(ms: number): string {
    const s = Math.max(0, Math.round((Date.now() - ms) / 1000))
    if (s < 60) return '刚刚'
    if (s < 3600) return `${Math.floor(s / 60)} 分钟前`
    return `${Math.floor(s / 3600)} 小时前`
}

function shortPath(p: string): string {
    return p.replace(/^\/Users\/[^/]+/, '~')
}
