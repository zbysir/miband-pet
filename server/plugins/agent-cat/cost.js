// Agent 猫的 Mac 端（server.js 引入）：记每天 Claude Code 花了多少钱，推给手环（useStore('cost')）。
//
// 钱数是 Claude Code 自己算的：它传给状态栏命令的数据里有 cost.total_cost_usd（这个会话一共花了多少，和 /cost 一样）。
// App 里打开「显示 Claude Code 额度」后，状态栏脚本按会话把这份数据各存一份到 claude-sessions/<会话 id>.json。
// 这里每 2 秒看一眼这个目录，只读修改时间变了的；按会话记下上次的钱数，涨了多少就记到当天。
// 旧版脚本只写 claude-status.json（几个会话同时开着时只留最后刷新的那个），目录里还没有文件时退回读它。
// 所以只记得到打开开关之后的；之前开着的会话只从开始记账时算起。没打开过开关、也没有记录的，不推，界面上不显示。
import { publish, readFile, readDir, stat, storage, home } from 'miband/server'

const DATA = home + '/Library/Application Support/miband-pet'
const DIR = DATA + '/claude-sessions'
const FILE = DATA + '/claude-status.json'
const WEEKDAYS = ['日', '一', '二', '三', '四', '五', '六']
const pad = (n) => (n < 10 ? '0' : '') + n
const dayKey = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`

// { start: 开始记账的时间（毫秒）, days: { '2026-10-23': 美元 }, sessions: { 会话 id: { usd: 上次看到的钱数, seen: 时间 } } }
const cost = storage.get('cost') || { start: Date.now(), days: {}, sessions: {} }
storage.set('cost', cost)

const readMtime = {} // 文件 → 上次读过时的修改时间

function list(dir) {
  try {
    return readDir(dir)
  } catch (e) {
    return []
  }
}

// 记一个会话的最新钱数，返回有没有改动
function record(st, data) {
  const id = data.session_id
  const usd = data.cost && data.cost.total_cost_usd
  if (!id || typeof usd !== 'number') return false
  const prev = cost.sessions[id]
  if (prev && prev.usd === usd) return false
  let delta
  if (!prev) {
    // 第一次见到这个会话：开始记账之后才开的全算；之前就开着的，之前花的不知道是哪天的，从现在起算
    const startedAt = st.mtime - ((data.cost && data.cost.total_duration_ms) || 0)
    delta = startedAt >= cost.start ? usd : 0
  } else {
    // 变少了是会话重新打开（--resume），钱数从 0 重新算
    delta = usd >= prev.usd ? usd - prev.usd : usd
  }
  cost.sessions[id] = { usd, seen: st.mtime }
  if (delta > 0) {
    const k = dayKey(new Date(st.mtime))
    cost.days[k] = (cost.days[k] || 0) + delta
  }
  return true
}

function check() {
  const names = list(DIR).filter((n) => n.endsWith('.json'))
  const files = names.length > 0 ? names.map((n) => DIR + '/' + n) : [FILE]
  let changed = false
  for (const f of files) {
    const st = stat(f)
    if (!st || readMtime[f] === st.mtime) continue
    readMtime[f] = st.mtime
    let data
    try {
      data = JSON.parse(readFile(f))
    } catch (e) {
      continue // 读到一半被删了之类的，下次再看
    }
    if (record(st, data)) changed = true
  }
  if (!changed) return
  // 一周没动静的会话不再记；天数只留最近 400 天
  const now = Date.now()
  for (const sid in cost.sessions) if (now - cost.sessions[sid].seen > 7 * 86400000) delete cost.sessions[sid]
  const keys = Object.keys(cost.days).sort()
  for (const old of keys.slice(0, Math.max(0, keys.length - 400))) delete cost.days[old]
  storage.set('cost', cost)
  publishCost()
}

// 推给手环的样子：今天的钱数 + 最近 7 天（最后一个是今天）
function publishCost() {
  if (Object.keys(cost.sessions).length === 0 && Object.keys(cost.days).length === 0) return publish('cost', null)
  const days = []
  for (let i = 6; i >= 0; i--) {
    const d = new Date()
    d.setDate(d.getDate() - i)
    days.push({ date: `${pad(d.getMonth() + 1)}-${pad(d.getDate())}`, weekday: WEEKDAYS[d.getDay()], usd: cost.days[dayKey(d)] || 0 })
  }
  publish('cost', { today: days[6].usd, days })
}

check()
publishCost()
setInterval(check, 2000)
setInterval(publishCost, 60000) // 过了零点「今天」要换
