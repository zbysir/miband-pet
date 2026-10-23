// Agent 猫：一只像素猫代表所有 agent 的整体状态。
// 有 agent 卡住了它举牌，卡太久（默认 5 分钟）就着急；刚跑完一个就庆祝 8 秒，有在跑的就敲笔记本，都空闲时睡觉。
// 下面的气泡显示一个需要关注的 agent，点猫或气泡换下一个。
// 右下角的小鱼干是今天完成了几个任务。agent 状态、小鱼干由本插件的 herdr.js 在 Mac 上读 herdr、记下来。标题下面是额度（哪一项看配置，默认用得最多的那项）。
// 往上翻（跟手的 Pager）或点小鱼干看第二屏「今日」（today.jsx）。额度由本插件的 mac.js 在 Mac 上读。
import { View, Text, Image, Sprite, Row, Spacer, Pager, useStore, useState, useEffect, useRef, useInterval, useScreen, useConfig, asset, vibrate } from 'miband'
import { Today, fmtReset } from './today'

const PRIORITY = { blocked: 0, working: 1, done: 2, idle: 3, unknown: 4 }
const COLOR = { blocked: '#ff9f1c', working: '#2ea0ff', done: '#3ddc84', idle: '#5c6068', unknown: '#5c6068' }
const HEAD_COLOR = { alert: '#ff9f1c', impatient: '#ff5c5c', work: '#2ea0ff', celebrate: '#3ddc84', sleep: '#c9ced8' }
// 每种动作的帧数和一轮的时长（帧由 tools/gen_pet.py 生成）；着急时举牌摇得更快
const FRAMES = { work: [6, 900], alert: [6, 700], celebrate: [6, 800], sleep: [8, 2400] }
const ACTION = { alert: ['alert', 700], impatient: ['alert', 300], work: ['work', 900], celebrate: ['celebrate', 800], sleep: ['sleep', 2400] }
const CELEBRATE_MS = 8000

function frames(skin, action) {
  const list = []
  for (let i = 0; i < FRAMES[action][0]; i++) list.push(asset(`assets/${skin}/${action}_${i}.png`))
  return list
}

function byProjectThenId(x, y) {
  return (x.project || '').localeCompare(y.project || '') || (x.id < y.id ? -1 : x.id > y.id ? 1 : 0)
}

function countBy(agents) {
  const n = { blocked: 0, working: 0, done: 0, idle: 0 }
  for (const a of agents) if (n[a.status] !== undefined) n[a.status]++
  return n
}

// 刚有 agent 从 working 变成 done/idle 时庆祝一会儿
function useCelebrate(agents) {
  const prev = useRef(null)
  const [until, setUntil] = useState(0)
  useEffect(() => {
    let finished = false
    if (prev.current) {
      for (const a of agents) {
        const before = prev.current[a.id]
        if (before === 'working' && (a.status === 'done' || a.status === 'idle')) finished = true
      }
    }
    prev.current = {}
    for (const a of agents) prev.current[a.id] = a.status
    if (finished) setUntil(Date.now() + CELEBRATE_MS)
  }, [agents])
  return useUntil(until, setUntil)
}

// until 之前返回 true，到点后自动重新渲染
function useUntil(until, setUntil) {
  useEffect(() => {
    if (!until) return
    const t = setTimeout(() => setUntil(0), Math.max(0, until - Date.now()))
    return () => clearTimeout(t)
  }, [until])
  return until > 0
}

// 卡得最久的那个 agent 和卡了几分钟（超过设定的分钟数才算）
function useImpatient(agents, limitMin) {
  const [, tick] = useState(0)
  useInterval(() => tick((n) => n + 1), 20000) // 卡着的时候分钟数要往上走
  const warned = useRef({})
  let worst = null
  let mins = 0
  if (limitMin > 0) {
    for (const a of agents) {
      if (a.status !== 'blocked' || !a.since) continue
      const m = Math.floor((Date.now() - a.since) / 60000)
      if (m >= limitMin && m > mins) {
        worst = a
        mins = m
      }
    }
  }
  // 每次卡住只震一下
  useEffect(() => {
    if (!worst) return
    const key = worst.id + ':' + worst.since
    if (!warned.current[key]) {
      warned.current[key] = true
      vibrate('long')
    }
  }, [worst && worst.id, worst && worst.since])
  return worst ? { agent: worst, mins } : null
}

// 首页显示的那项额度：配置里选了具体哪一项（如 claude:5h）就显示它，auto 是所有里面用得最多的
function tightest(usage, which) {
  if (which === 'off') return null
  const [wantAgent, wantWindow] = which && which !== 'auto' ? which.split(':') : []
  let best = null
  for (const src of usage || []) {
    if (wantAgent && src.agent !== wantAgent) continue
    for (const w of src.windows || []) {
      if (wantWindow && w.name !== wantWindow) continue
      if (!best || w.used > best.used) best = { agent: src.agent === 'claude' ? 'Claude' : src.agent === 'codex' ? 'Codex' : src.agent, ...w }
    }
  }
  return best
}

// 标题下面的额度：名字 + 进度条 + 百分比，用得越多颜色越红；没有额度数据时留空占位，布局不跳
function QuotaBar({ quota }) {
  if (!quota) return <View style={{ width: 192, height: 20 }} />
  const pct = Math.min(100, Math.max(0, quota.used))
  const color = quota.used >= 80 ? '#ff6b6b' : quota.used >= 50 ? '#ff9f1c' : '#3ddc84'
  return (
    <Row align="center" style={{ width: 192, height: 20 }}>
      <Text style={{ width: 64, fontSize: 12, color: '#e8ebf1', lines: 1 }}>{`${quota.agent} ${quota.name}`}</Text>
      <View style={{ width: 68, height: 8, borderRadius: 4, backgroundColor: '#2a2e36' }}>
        <View style={{ width: Math.max(4, Math.round((pct / 100) * 68)), height: 8, borderRadius: 4, backgroundColor: color }} />
      </View>
      <Text style={{ width: 60, fontSize: 12, color, textAlign: 'right', lines: 1 }}>
        {`${Math.round(quota.used)}%${quota.resetsAt ? ' ' + fmtReset(quota.resetsAt) : ''}`}
      </Text>
    </Row>
  )
}

// 两屏：猫、今日。往上翻跟着手指走（Pager 在手环上是原生 swiper）
export default function AgentCat() {
  const screen = useScreen()
  const [page, setPage] = useState(0)
  // 今日页等猫显示出来一会儿再画：手环上新建节点慢，两屏一起画打开要多等半秒多
  const [todayReady, setTodayReady] = useState(false)
  useEffect(() => {
    const t = setTimeout(() => setTodayReady(true), 1000)
    return () => clearTimeout(t)
  }, [])
  const size = { width: 212, height: screen.height }
  return (
    <Pager vertical index={page} onChange={setPage} style={size}>
      <CatPage key="cat" openToday={() => setPage(1)} />
      {todayReady || page === 1 ? <Today key="today" /> : <View key="today" style={size} />}
    </Pager>
  )
}

function CatPage({ openToday }) {
  const agents = useStore('agents') || []
  const stats = useStore('stats') || { today: { done: 0 } }
  const usage = useStore('usage') // 本插件 mac.js 读的额度
  const config = useConfig()
  // 猫的样子是插件配置（和 App 里「菜单栏的猫」分开设置）
  const skin = config.skin || 'orange'
  const screen = useScreen()
  const celebrating = useCelebrate(agents)
  const impatient = useImpatient(agents, config.impatientMin ?? 5)
  const [focusId, setFocusId] = useState('')

  const n = countBy(agents)
  let mood = 'sleep'
  let headline = n.done > 0 ? '都完成啦' : '都在休息 zZ'
  if (impatient) {
    mood = 'impatient'
    headline = `${impatient.agent.project || impatient.agent.agent} 等了 ${impatient.mins} 分钟！`
  } else if (n.blocked > 0) {
    mood = 'alert'
    headline = `${n.blocked} 个等你处理！`
  } else if (celebrating) {
    mood = 'celebrate'
    headline = '搞定一个！'
  } else if (n.working > 0) {
    mood = 'work'
    headline = `${n.working} 个在忙`
  }
  const [action, duration] = ACTION[mood]

  // 气泡里的 agent：从需要关注的（不是空闲的）里面挑，按优先级排；记住当前是谁，数据更新时不跳回第一个。
  // 状态相同的再按项目、id 排：Mac 推来的列表顺序不固定，不排的话「1/2」和点了换谁都会乱跳
  const sorted = agents.slice().sort((x, y) => (PRIORITY[x.status] ?? 4) - (PRIORITY[y.status] ?? 4) || byProjectThenId(x, y))
  const top = sorted.filter((a) => a.status !== 'idle' && a.status !== 'unknown')
  const pool = top.length > 0 ? top : sorted
  const index = Math.max(0, pool.findIndex((a) => a.id === focusId))
  const focus = pool[index]
  const next = () => {
    if (pool.length > 1) setFocusId(pool[(index + 1) % pool.length].id)
  }

  const quota = tightest(usage, config.quota || 'auto')

  return (
    <View style={{ width: 212, height: screen.height, flexDirection: 'column', alignItems: 'center', padding: [8, 10, 0, 10], backgroundColor: '#0a0c12' }}>
      <Text style={{ width: 192, height: 26, lines: 1, textOverflow: 'ellipsis', fontSize: 18, fontWeight: 'bold', color: HEAD_COLOR[mood] || HEAD_COLOR.sleep, textAlign: 'center' }}>
        {headline}
      </Text>
      <QuotaBar quota={quota} />
      <View style={{ width: 192, height: 142, marginTop: 2, justifyContent: 'center', alignItems: 'center' }} onClick={next}>
        <Sprite key={skin + '-' + mood} frames={frames(skin, action)} duration={duration} style={{ width: 140, height: 140 }} />
      </View>
      {focus ? (
        // 状态色画成一整圈描边（手环和 App 里的预览画出来一样）
        <View
          onClick={next}
          style={{
            width: 192, height: 62, marginTop: 6, padding: [6, 10], borderRadius: 14, backgroundColor: '#16191f',
            flexDirection: 'column', borderWidth: 2, borderColor: COLOR[focus.status] || COLOR.unknown
          }}
        >
          <Row align="center">
            <Text style={{ flex: 1, fontSize: 14, fontWeight: 'bold', lines: 1, textOverflow: 'ellipsis' }}>{focus.project || focus.agent || '?'}</Text>
            {pool.length > 1 && <Text style={{ marginLeft: 6, fontSize: 11, color: '#8a8f98' }}>{`${index + 1}/${pool.length}`}</Text>}
          </Row>
          <Text style={{ fontSize: 12, color: '#8a8f98', lines: 1, textOverflow: 'ellipsis' }}>{focus.title || ''}</Text>
        </View>
      ) : (
        <View style={{ height: 62, marginTop: 6 }} />
      )}
      <Row align="center" style={{ width: 192, height: 22, marginTop: 8 }}>
        {['blocked', 'working', 'done', 'idle'].map((s) =>
          s === 'blocked' && n.blocked === 0 ? null : (
            <Row key={s} align="center" style={{ marginRight: 10 }}>
              <View style={{ width: 8, height: 8, borderRadius: 4, marginRight: 4, backgroundColor: COLOR[s] }} />
              <Text style={{ fontSize: 12, color: '#8a8f98' }}>{n[s]}</Text>
            </Row>
          )
        )}
        <Spacer />
        <Row align="center" onClick={openToday} style={{ height: 22, paddingLeft: 8 }}>
          <Image src={asset('assets/fish.png')} style={{ width: 24, height: 12 }} />
          <Text style={{ marginLeft: 4, fontSize: 12, color: stats.today.done > 0 ? '#e29640' : '#5c6068' }}>{stats.today.done}</Text>
        </Row>
      </Row>
    </View>
  )
}
