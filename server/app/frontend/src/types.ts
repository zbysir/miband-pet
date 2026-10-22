// 和 Go 侧 AppState（app/app.go）、service.State 对应
export type Status = 'blocked' | 'working' | 'done' | 'idle' | 'unknown'

export interface Agent {
    id: string
    agent: string
    project: string
    title: string
    status: Status
}

export interface EventRecord {
    time: number
    title: string
    body: string
    action: 'wake' | 'notify'
}

export type AlertMode = 'auto' | 'wake' | 'notify' | 'both'

export interface Settings {
    notify: boolean
    notifyStart: boolean
    alert: AlertMode
    address: string
    pet: 'orange' | 'gray' | 'white'
    keepAwake: boolean // 有 agent 等你回答时保持亮屏
    keepAwakeSec: number // 每隔几秒亮一次屏
    impatientMin: number // 卡住超过几分钟猫着急、再提醒一次；0 表示关
    claudeUsage: boolean // 接管了 Claude Code 的状态栏来拿额度
}

// 和 Go 侧 service.StatsView 对应
export interface Stats {
    today: { done: number, fed: number }
    fishLeft: number
    fedTotal: number
    days: { date: string, weekday: string, done: number }[]
}

// 和 Go 侧 usage.Source 对应
export interface UsageSource {
    agent: 'claude' | 'codex' | string
    windows: { name: string, used: number, resetsAt: number }[]
    updated: number
}

export interface AppState {
    configured: boolean
    configPath: string
    setupError: string
    installing: string
    installProgress: number
    connected: boolean
    transport: '' | 'classic' | 'ble'
    lastError: string
    packages: string[] | null
    foreground: string
    agents: Agent[] | null
    stats: Stats | null
    usage: UsageSource[] | null
    plugins: PluginInfo[] | null
    pluginDir: string
    legacy: string[] | null // 手环上还装着的旧版快应用
    herdrError: string
    lastPush: number
    events: EventRecord[] | null
    settings: Settings | null
}

// 和 Go 侧 plugin.Info 对应
export interface PluginInfo {
    name: string
    title: string
    builtin: boolean
    dir: string
    error: string
    active: boolean // 手环上当前显示的是这一页
}

export interface BundledApp {
    package: string
    name: string
    versionCode: number
    file: string
}

export const STATUS_TEXT: Record<Status, string> = {
    blocked: '待处理',
    working: '运行中',
    done: '完成',
    idle: '空闲',
    unknown: '未知',
}

export const STATUS_ORDER: Status[] = ['blocked', 'working', 'done', 'idle', 'unknown']
