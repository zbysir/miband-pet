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
}

export interface AppState {
    configured: boolean
    configPath: string
    setupError: string
    installing: string
    installProgress: number
    connected: boolean
    lastError: string
    packages: string[] | null
    foreground: string
    agents: Agent[] | null
    herdrError: string
    lastPush: number
    events: EventRecord[] | null
    settings: Settings | null
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
