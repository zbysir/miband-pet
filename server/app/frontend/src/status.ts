import type {AppInfo, AppState} from './types'

// app 在手环上的情况：装了没有、要不要更新（手环上的版本号和 App 里编好的不一样就要更新）
export function bandStatus(state: AppState, app: AppInfo) {
    const v = state.versions?.[app.package]
    const installed = v !== undefined
    const outdated = installed && v !== app.version
    let text = '连接后显示'
    if (state.connected) text = !installed ? '还没装到手环上' : outdated ? '改过了，可以更新到手环' : '手环上是最新的'
    return {installed, outdated, text}
}
