// 和 Go 侧 AppState（app/app.go）、service.State 对应
export interface Settings {
    address: string
    pet: 'orange' | 'gray' | 'white' // 菜单栏的猫
    claudeUsage: boolean // 接管了 Claude Code 的状态栏来拿额度
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
    versions: Record<string, number> | null // 手环上装了的快应用 → 版本号（versionCode）
    foreground: string
    usage: UsageSource[] | null
    plugins: PluginInfo[] | null
    pluginDir: string
    legacy: string[] | null // 手环上还装着的旧版快应用
    lastPush: number
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
    code: string // 代码的哈希，变了预览重新加载
    hasServer: boolean // 带 server.js
    permissions: string[] | null // server.js 要的权限
    needGrant: boolean // 要了权限还没允许，server.js 没跑
    permKey: string
    settings: PluginSetting[] | null // manifest.json 里声明的配置项
    config: Record<string, unknown> | null // 现在的配置
}

// 和 Go 侧 plugin.Setting 对应
export interface PluginSetting {
    key: string
    title: string
    description?: string
    type: 'select' | 'bool' | 'number' | 'text'
    options?: { value: string, label: string }[]
    default: unknown
}

export interface BundledApp {
    package: string
    name: string
    versionCode: number
    file: string
}
