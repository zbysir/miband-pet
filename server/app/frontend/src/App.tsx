import {useCallback, useEffect, useState} from 'react'
import type {ReactNode} from 'react'
import {Bell, FolderOpen, PawPrint, Puzzle, RefreshCw, Watch} from 'lucide-react'
import {
    AllowPlugin, BundledApps, CopyBuiltin, GetState, Install, Launch, OpenPluginDir, RevokePlugin, SetClaudeUsage,
    SetImpatient, SetKeepAwake, SetPet, SetPluginConfig, TestNotify, Uninstall, UpdateSettings,
} from '../wailsjs/go/main/App'
import {EventsOn} from '../wailsjs/runtime/runtime'
import {Badge} from '@/components/ui/badge'
import {Button} from '@/components/ui/button'
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from '@/components/ui/card'
import {Input} from '@/components/ui/input'
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from '@/components/ui/select'
import {Separator} from '@/components/ui/separator'
import {Switch} from '@/components/ui/switch'
import {ToggleGroup, ToggleGroupItem} from '@/components/ui/toggle-group'
import {Wizard} from './Wizard'
import {Preview} from './Preview'
import {SKINS} from './Pet'
import type {Skin} from './Pet'
import type {AlertMode, AppState, BundledApp, PluginInfo, PluginSetting} from './types'

type Run = (label: string, fn: () => Promise<unknown>) => void

export default function App() {
    const [state, setState] = useState<AppState | null>(null)
    const [bundled, setBundled] = useState<BundledApp[]>([])
    const [toast, setToast] = useState('')
    const [wizard, setWizard] = useState(false)
    const [selected, setSelected] = useState('')

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
        const t = setInterval(refresh, 30_000)
        return () => {
            off()
            clearInterval(t)
        }
    }, [refresh])

    const run: Run = useCallback((label, fn) => {
        fn().then(() => setToast(label + '：完成'))
            .catch(e => setToast(label + '失败：' + String(e)))
            .finally(() => setTimeout(() => setToast(''), 3000))
    }, [])

    if (!state) return null
    if (wizard || !state.configured) {
        return <Wizard state={state} bundled={bundled} onFinish={() => setWizard(false)}/>
    }

    // 左边预览的是哪个插件：手动选过的 → 手环上正在显示的 → 第一个
    const plugins = state.plugins ?? []
    const plugin = plugins.find(p => p.name === selected) ?? plugins.find(p => p.active) ?? plugins[0]

    return (
        <div className="grid h-screen grid-cols-[minmax(320px,380px)_1fr]">
            <div className="drag fixed inset-x-0 top-0 z-10 h-11 bg-background"/>
            {/* 两栏各自滚动；内容包一层，不然高度固定的 flex 列会把卡片压扁 */}
            <aside className="overflow-y-auto border-r px-6 pt-14 pb-6">
                <div className="flex flex-col gap-4">
                    <div className="flex items-center justify-between">
                        <h1 className="text-base font-semibold">插件预览</h1>
                        <StatusBadge state={state}/>
                    </div>
                    <Preview state={state} run={run} plugin={plugin} onSelect={setSelected}/>
                </div>
            </aside>
            <main className="overflow-y-auto px-6 pt-14 pb-6">
                <div className="flex flex-col gap-4">
                {plugin && (plugin.settings?.length ?? 0) > 0 && <PluginSettingsCard plugin={plugin} run={run}/>}
                <MenubarCard state={state} run={run}/>
                <AlertCard state={state} run={run}/>
                <BandAppsCard state={state} bundled={bundled} run={run}/>
                <PluginsCard state={state} run={run} onSelect={setSelected}/>
                <div className="flex gap-2">
                    <Button variant="outline" onClick={() => run('测试通知', TestNotify)} disabled={!state.connected}><Bell/>发测试通知</Button>
                    <Button variant="outline" onClick={() => setWizard(true)}><RefreshCw/>换手环 / 重新绑定</Button>
                </div>
                </div>
            </main>
            {toast && (
                <div className="fixed bottom-5 left-1/2 z-20 -translate-x-1/2 rounded-lg border bg-popover px-4 py-2 text-sm text-popover-foreground shadow-lg">
                    {toast}
                </div>
            )}
        </div>
    )
}

// ---------- 通用的设置行 ----------

// 一行设置：左边标题和说明，右边控件（开关之类）
function Row({title, hint, children}: { title: ReactNode, hint?: ReactNode, children?: ReactNode }) {
    return (
        <div className="flex items-center justify-between gap-6">
            <div className="min-w-0">
                <div className="text-sm font-medium">{title}</div>
                {hint && <div className="mt-0.5 text-xs text-muted-foreground">{hint}</div>}
            </div>
            {children && <div className="shrink-0">{children}</div>}
        </div>
    )
}

// 几个选项里选一个：选项少平铺成一排，多了用下拉框
function Choice({value, options, onChange, disabled}: {
    value: string, options: { value: string, label: string }[], onChange: (v: string) => void, disabled?: boolean,
}) {
    if (options.length > 4) {
        return (
            <Select value={value} onValueChange={onChange} disabled={disabled}>
                <SelectTrigger className="w-48"><SelectValue/></SelectTrigger>
                <SelectContent>
                    {options.map(o => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}
                </SelectContent>
            </Select>
        )
    }
    return (
        <ToggleGroup type="single" variant="outline" size="sm" value={value} disabled={disabled}
                     onValueChange={v => v && v !== value && onChange(v)}>
            {options.map(o => (
                <ToggleGroupItem key={o.value} value={o.value}
                                 className="px-3 data-[state=on]:border-primary data-[state=on]:bg-primary data-[state=on]:text-primary-foreground">
                    {o.label}
                </ToggleGroupItem>
            ))}
        </ToggleGroup>
    )
}

// ---------- 状态 ----------

function StatusBadge({state}: { state: AppState }) {
    let text = '手环已连接'
    let dot = 'bg-emerald-500'
    if (!state.connected) {
        text = state.lastError ? '连接失败，重试中' : '正在连接手环…'
        dot = state.lastError ? 'bg-red-500' : 'bg-amber-500'
    }
    return (
        <Badge variant="outline" title={state.lastError} className="gap-1.5">
            <span className={`size-1.5 rounded-full ${dot}`}/>{text}
        </Badge>
    )
}

// ---------- 选中插件的配置 ----------

function PluginSettingsCard({plugin, run}: { plugin: PluginInfo, run: Run }) {
    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2"><Puzzle className="size-4"/>{plugin.title}</CardTitle>
                <CardDescription>左边预览的这个插件的设置，改了马上推到手环</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-5">
                {(plugin.settings ?? []).map(s => <SettingField key={s.key} plugin={plugin} setting={s} run={run}/>)}
            </CardContent>
        </Card>
    )
}

// 插件的一个配置项：照着 manifest.json 里 settings 的声明画
function SettingField({plugin, setting, run}: { plugin: PluginInfo, setting: PluginSetting, run: Run }) {
    const value = plugin.config?.[setting.key] ?? setting.default
    const save = (v: unknown) => run('保存配置', () => SetPluginConfig(plugin.name, setting.key, JSON.stringify(v)))
    switch (setting.type) {
        case 'bool':
            return <Row title={setting.title} hint={setting.description}><Switch checked={!!value} onCheckedChange={save}/></Row>
        case 'select':
            return (
                <div className="flex flex-col gap-2">
                    <Row title={setting.title} hint={setting.description}/>
                    <Choice value={String(value)} options={setting.options ?? []} onChange={save}/>
                </div>
            )
        default:
            return (
                <div className="flex flex-col gap-2">
                    <Row title={setting.title} hint={setting.description}/>
                    <Input type={setting.type === 'number' ? 'number' : 'text'} defaultValue={String(value ?? '')} className="max-w-64"
                           onBlur={e => {
                               if (e.target.value === String(value)) return
                               save(setting.type === 'number' ? Number(e.target.value) : e.target.value)
                           }}/>
                </div>
            )
    }
}

// ---------- 菜单栏的猫：App 自己的设置，和手环上 Agent 猫插件的「猫」分开选 ----------

function MenubarCard({state, run}: { state: AppState, run: Run }) {
    const current = state.settings?.pet ?? 'orange'
    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2"><PawPrint className="size-4"/>菜单栏的猫</CardTitle>
                <CardDescription>手环上的猫在插件设置里单独选</CardDescription>
            </CardHeader>
            <CardContent className="flex gap-2">
                {SKINS.map(s => (
                    <Button key={s.value} variant={s.value === current ? 'secondary' : 'ghost'}
                            className={`h-auto flex-col gap-1 px-4 py-2 ${s.value === current ? 'ring-2 ring-ring' : ''}`}
                            onClick={() => s.value !== current && run('换猫', () => SetPet(s.value as Skin))}>
                        <img src={`/pet/${s.value}/sleep_0.png`} width={40} height={40} alt="" className="pixel"/>
                        <span className="text-xs">{s.label}</span>
                    </Button>
                ))}
            </CardContent>
        </Card>
    )
}

// ---------- 提醒 ----------

const ALERT_MODES: { value: AlertMode, label: string, desc: string }[] = [
    {value: 'auto', label: '自动', desc: '手环上正开着 wristkit 时只亮屏，否则发通知震动'},
    {value: 'wake', label: '只亮屏', desc: '把手环上的 wristkit 打开到前台，亮屏但不震动'},
    {value: 'notify', label: '只通知', desc: '发通知，手环震动'},
    {value: 'both', label: '都要', desc: '打开 wristkit 并发通知'},
]

// 保持亮屏的间隔：手环亮屏最长 60 秒，要比手环上设的亮屏时间短
const KEEP_AWAKE_SECS = [15, 30, 50]

// 卡住多久算太久（0 表示不管）
const IMPATIENT_MINS = [0, 3, 5, 10]

function AlertCard({state, run}: { state: AppState, run: Run }) {
    const s = state.settings
    if (!s) return null
    const save = (notify: boolean, notifyStart: boolean, alert: AlertMode) => run('保存设置', () => UpdateSettings(notify, notifyStart, alert))
    const keepAwake = (on: boolean, sec: number) => run('保存设置', () => SetKeepAwake(on, sec))
    const mode = ALERT_MODES.find(m => m.value === s.alert) ?? ALERT_MODES[0]
    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2"><Bell className="size-4"/>提醒</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-5">
                <Row title="agent 有变化时提醒"><Switch checked={s.notify} onCheckedChange={v => save(v, s.notifyStart, s.alert)}/></Row>
                <Row title="开始跑也提醒" hint="关掉后只在跑完、卡住时提醒">
                    <Switch checked={s.notifyStart} disabled={!s.notify} onCheckedChange={v => save(s.notify, v, s.alert)}/>
                </Row>
                <div className="flex flex-col gap-2">
                    <Row title="怎么提醒" hint={mode.desc}/>
                    <Choice value={s.alert} disabled={!s.notify} onChange={v => save(s.notify, s.notifyStart, v as AlertMode)}
                            options={ALERT_MODES.map(m => ({value: m.value, label: m.label}))}/>
                </div>
                <Separator/>
                <Row title="agent 卡住多久算太久"
                     hint={s.impatientMin > 0 ? `卡住超过 ${s.impatientMin} 分钟，手环上的猫会着急，并再提醒你一次` : '卡多久都不再提醒'}/>
                <Choice value={String(s.impatientMin)} onChange={v => run('保存设置', () => SetImpatient(Number(v)))}
                        options={IMPATIENT_MINS.map(m => ({value: String(m), label: m === 0 ? '不管' : `${m} 分钟`}))}/>
                <Row title="有 agent 等你回答时保持亮屏"
                     hint={`每隔 ${s.keepAwakeSec} 秒把手环上的 wristkit 打开一次，手环不熄屏。手环上的亮屏时间要比这个长（最长 60 秒）`}>
                    <Switch checked={s.keepAwake} onCheckedChange={v => keepAwake(v, s.keepAwakeSec)}/>
                </Row>
                <Choice value={String(s.keepAwakeSec)} disabled={!s.keepAwake} onChange={v => keepAwake(true, Number(v))}
                        options={KEEP_AWAKE_SECS.map(sec => ({value: String(sec), label: `每 ${sec} 秒`}))}/>
                <Separator/>
                <Row title="显示 Claude Code 额度"
                     hint="Claude Code 只把额度告诉状态栏命令，所以要把它的状态栏换成一个小脚本：先存一份数据，再交给你原来的状态栏命令，显示不变。关掉会原样还原（改之前会备份 ~/.claude/settings.json）。只有 Pro/Max 订阅才有额度；Codex 的额度不用设置。">
                    <Switch checked={s.claudeUsage} onCheckedChange={v => run(v ? '打开额度' : '关掉额度', () => SetClaudeUsage(v))}/>
                </Row>
            </CardContent>
        </Card>
    )
}

// ---------- 手环上的应用 ----------

const LEGACY_NAMES: Record<string, string> = {
    'dev.bysir.herdrpet': 'Agent 猫',
    'dev.bysir.herdrband': 'Agents',
}

function BandAppsCard({state, bundled, run}: { state: AppState, bundled: BundledApp[], run: Run }) {
    const installed = new Set(state.packages ?? [])
    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2"><Watch className="size-4"/>手环上的应用</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
                {bundled.length === 0 && <p className="text-xs text-muted-foreground">这个 App 没有打包手环应用（用 make app 构建）</p>}
                {bundled.map(b => {
                    const has = installed.has(b.package)
                    const busy = state.installing === b.package
                    return (
                        <div key={b.package} className="flex flex-col gap-2">
                            <Row title={b.name} hint={`${!state.connected ? '连接后显示' : has ? '已安装' : '未安装'} · v${b.versionCode}`}>
                                <div className="flex gap-2">
                                    {has && <Button variant="outline" size="sm" disabled={!state.connected || busy}
                                                    onClick={() => run('打开', () => Launch(b.package))}>在手环上打开</Button>}
                                    <Button size="sm" disabled={!state.connected || !!state.installing}
                                            onClick={() => run(has ? '更新' : '安装', () => Install(b.package))}>
                                        {busy ? `${Math.round(state.installProgress * 100)}%` : has ? '更新' : '安装'}
                                    </Button>
                                </div>
                            </Row>
                            {busy && (
                                <div className="h-1 overflow-hidden rounded-full bg-muted">
                                    <div className="h-full bg-primary transition-all" style={{width: `${state.installProgress * 100}%`}}/>
                                </div>
                            )}
                        </div>
                    )
                })}
                {(state.legacy ?? []).map(pkg => (
                    <Row key={pkg} title={<>{LEGACY_NAMES[pkg] ?? pkg} <span className="text-muted-foreground">（旧版）</span></>}
                         hint="已经改成插件，不再推送，可以卸载">
                        <div className="flex gap-2">
                            <Button variant="outline" size="sm" disabled={!state.connected || !!state.installing}
                                    onClick={() => run('打开', () => Launch(pkg))}>在手环上打开</Button>
                            <Button variant="destructive" size="sm" disabled={!state.connected || !!state.installing}
                                    onClick={() => run('卸载', () => Uninstall(pkg))}>卸载</Button>
                        </div>
                    </Row>
                ))}
            </CardContent>
        </Card>
    )
}

// ---------- 插件 ----------

function PluginsCard({state, run, onSelect}: { state: AppState, run: Run, onSelect: (name: string) => void }) {
    const list = state.plugins ?? []
    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2"><Puzzle className="size-4"/>插件 <span className="text-muted-foreground">{list.length}</span></CardTitle>
                <CardDescription>自己写插件：把插件文件夹放进插件目录，保存后自动生效</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
                {list.map(p => (
                    <div key={p.name} className="flex flex-col gap-1">
                        <Row title={<span className="flex items-center gap-2">{p.title}{p.active && <Badge variant="secondary">手环上</Badge>}</span>}
                             hint={p.builtin ? '内置' : shortPath(p.dir)}>
                            <div className="flex gap-2">
                                {p.needGrant && <Button size="sm" onClick={() => run('允许', () => AllowPlugin(p.name, p.permKey))}>允许</Button>}
                                {!p.builtin && !p.needGrant && (p.permissions?.length ?? 0) > 0 &&
                                    <Button variant="outline" size="sm" onClick={() => run('收回权限', () => RevokePlugin(p.name))}>收回权限</Button>}
                                <Button variant="ghost" size="sm" onClick={() => onSelect(p.name)}>预览</Button>
                                {p.builtin && <Button variant="outline" size="sm" title="把源码复制到插件目录，改了就替换内置的这个"
                                                      onClick={() => run('复制', () => CopyBuiltin(p.name))}>复制出来改</Button>}
                            </div>
                        </Row>
                        {!p.builtin && (p.permissions?.length ?? 0) > 0 && (
                            <p className="text-xs text-muted-foreground">
                                需要权限：{p.permissions!.join('、')}{p.needGrant ? '（还没允许，这部分功能用不了）' : ''}
                            </p>
                        )}
                        {p.error && <p className="max-h-32 overflow-auto whitespace-pre-wrap break-all text-xs text-destructive">{p.error}</p>}
                    </div>
                ))}
                <div>
                    <Button variant="outline" size="sm" onClick={() => run('打开插件目录', OpenPluginDir)}><FolderOpen/>打开插件目录</Button>
                </div>
            </CardContent>
        </Card>
    )
}

function shortPath(p: string): string {
    return p.replace(/^\/Users\/[^/]+/, '~')
}
