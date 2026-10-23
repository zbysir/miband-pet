// Agent 猫的 Mac 端。agent 状态、每日统计、提醒在 herdr.js；这里读 Claude Code 和 Codex 的额度，
// 推给手环（useStore('usage')）。读不到的不推，界面上也就不显示。
//
//   - Claude Code：只把额度传给状态栏命令。App 里打开「显示 Claude Code 额度」后，状态栏脚本会把数据存到
//     ~/Library/Application Support/miband-pet/claude-status.json（server/internal/usage/statusline.go），这里读它。
//     只有 Pro/Max 订阅才有。
//   - Codex：会话日志 ~/.codex/sessions/年/月/日/rollout-*.jsonl 里的 token_count 记录自带额度，读最近的一条。
import { publish, on, readFile, readDir, stat, home } from 'miband/mac'
import './herdr'

const CLAUDE = home + '/Library/Application Support/miband-pet/claude-status.json'
const CODEX = home + '/.codex/sessions'

function list(dir) {
  try {
    return readDir(dir).sort()
  } catch (e) {
    return []
  }
}

function claude(now) {
  const st = stat(CLAUDE)
  if (!st) return null
  let data
  try {
    data = JSON.parse(readFile(CLAUDE))
  } catch (e) {
    return null
  }
  const rl = data.rate_limits || {}
  const windows = []
  for (const [key, name] of [['five_hour', '5h'], ['seven_day', '7d'], ['spend_limit', '额度']]) {
    const w = rl[key]
    if (!w || w.used_percentage == null) continue
    // 窗口重置之后旧数据就不对了（下次刷新状态栏前不知道新窗口用了多少）
    if (w.resets_at && now >= w.resets_at) continue
    windows.push({ name, used: w.used_percentage, resetsAt: w.resets_at || 0 })
  }
  return { agent: 'claude', windows, updated: Math.floor(st.mtime / 1000) }
}

function windowName(minutes) {
  if (minutes === 300) return '5h'
  if (minutes === 10080) return '7d'
  if (minutes > 0 && minutes % 1440 === 0) return minutes / 1440 + 'd'
  if (minutes > 0 && minutes % 60 === 0) return minutes / 60 + 'h'
  return minutes > 0 ? minutes + 'm' : '?'
}

// 最近写过的会话日志：只看最近 3 天的目录
function latestCodexFile() {
  const days = []
  for (const y of list(CODEX)) for (const m of list(`${CODEX}/${y}`)) for (const d of list(`${CODEX}/${y}/${m}`)) days.push(`${CODEX}/${y}/${m}/${d}`)
  days.sort()
  let best = null
  let bestTime = 0
  for (const d of days.slice(-3)) {
    for (const f of list(d)) {
      if (!f.endsWith('.jsonl')) continue
      const st = stat(`${d}/${f}`)
      if (st && st.mtime > bestTime) {
        best = `${d}/${f}`
        bestTime = st.mtime
      }
    }
  }
  return best
}

function codex(now) {
  const file = latestCodexFile()
  if (!file) return null
  // 日志可能几十 MB，只读最后 2MB
  const lines = readFile(file, { tail: 2 << 20 }).split('\n')
  for (let i = lines.length - 1; i >= 0; i--) {
    if (lines[i].indexOf('"rate_limits"') < 0) continue
    let line
    try {
      line = JSON.parse(lines[i]) // 第一行可能被截断，解析不了就跳过
    } catch (e) {
      continue
    }
    const rl = line.payload && line.payload.rate_limits
    if (!rl) continue
    const windows = []
    for (const w of [rl.primary, rl.secondary]) {
      if (!w || w.used_percent == null || (w.resets_at && now >= w.resets_at)) continue
      windows.push({ name: windowName(w.window_minutes), used: w.used_percent, resetsAt: w.resets_at || 0 })
    }
    const t = Date.parse(line.timestamp)
    return { agent: 'codex', windows, updated: Math.floor((isNaN(t) ? Date.now() : t) / 1000) }
  }
  return null
}

// Codex 的日志大，每分钟才读一次，中间用上次读到的
let codexLast = null

function publishAll() {
  const now = Date.now() / 1000
  publish('usage', [claude(now), codexLast].filter((s) => s && s.windows.length > 0))
}

function refresh() {
  codexLast = codex(Date.now() / 1000)
  publishAll()
}

// App 里开关「显示 Claude Code 额度」后马上刷新一次
on('refresh', refresh)
refresh()
setInterval(refresh, 60000)
// Claude 的状态栏数据很小，每 5 秒看一眼有没有更新，更新了马上推
let claudeTime = 0
setInterval(() => {
  const st = stat(CLAUDE)
  const t = st ? st.mtime : 0
  if (t !== claudeTime) {
    claudeTime = t
    publishAll()
  }
}, 5000)
