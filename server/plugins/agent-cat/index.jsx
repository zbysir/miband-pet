// Agent 猫：一只像素猫代表所有 agent 的整体状态。
// 有 agent 卡住了它举牌，卡太久（默认 5 分钟）就着急；刚跑完一个就庆祝 8 秒，有在跑的就敲笔记本，都空闲时睡觉。
// 下面的气泡显示一个需要关注的 agent，点猫或气泡换下一个。
// 每跑完一个任务得一条小鱼干（Mac 记账），点右下角的小鱼干喂猫。标题下面一行小字是最紧的那项额度。
import { View, Text, Image, Sprite, Row, Spacer, useStore, useState, useEffect, useRef, useInterval, useScreen, asset, vibrate, call } from 'miband'

const PRIORITY = { blocked: 0, working: 1, done: 2, idle: 3, unknown: 4 }
const COLOR = { blocked: '#ff9f1c', working: '#2ea0ff', done: '#3ddc84', idle: '#5c6068', unknown: '#5c6068' }
const HEAD_COLOR = { alert: '#ff9f1c', impatient: '#ff5c5c', work: '#2ea0ff', celebrate: '#3ddc84', fed: '#e29640', sleep: '#c9ced8' }
// 每种动作的帧数和一轮的时长（帧由 tools/gen_pet.py 生成）；着急时举牌摇得更快
const FRAMES = { work: [6, 900], alert: [6, 700], celebrate: [6, 800], sleep: [8, 2400] }
const ACTION = { alert: ['alert', 700], impatient: ['alert', 300], work: ['work', 900], celebrate: ['celebrate', 800], fed: ['celebrate', 600], sleep: ['sleep', 2400] }
const CELEBRATE_MS = 8000
const FED_MS = 3000

function frames(skin, action) {
  const list = []
  for (let i = 0; i < FRAMES[action][0]; i++) list.push(asset(`assets/${skin}/${action}_${i}.png`))
  return list
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

function fmtReset(sec) {
  const left = Math.max(0, sec - Date.now() / 1000)
  if (left >= 86400) return Math.floor(left / 86400) + ' 天'
  const h = Math.floor(left / 3600)
  const m = Math.floor((left % 3600) / 60)
  return h + ':' + String(m).padStart(2, '0')
}

// 所有额度里用得最多的那一项
function tightest(usage) {
  let best = null
  for (const src of usage || []) {
    for (const w of src.windows || []) {
      if (!best || w.used > best.used) best = { agent: src.agent === 'claude' ? 'Claude' : src.agent === 'codex' ? 'Codex' : src.agent, ...w }
    }
  }
  return best
}

export default function AgentCat() {
  const agents = useStore('agents') || []
  const settings = useStore('settings') || {}
  const stats = useStore('stats') || { fishLeft: 0 }
  const usage = useStore('usage')
  const skin = settings.pet || 'orange'
  const screen = useScreen()
  const celebrating = useCelebrate(agents)
  const impatient = useImpatient(agents, settings.impatientMin ?? 5)
  const [focusId, setFocusId] = useState('')
  const [fedUntil, setFedUntil] = useState(0)
  const [noFishUntil, setNoFishUntil] = useState(0)
  const feeding = useUntil(fedUntil, setFedUntil)
  const noFish = useUntil(noFishUntil, setNoFishUntil)
  // 点了喂鱼，Mac 记完账推回新的数之前先在手环上减掉
  const [eaten, setEaten] = useState(0)
  useEffect(() => setEaten(0), [stats.fishLeft])
  const fishLeft = Math.max(0, stats.fishLeft - eaten)

  const feed = () => {
    if (fishLeft <= 0) {
      setNoFishUntil(Date.now() + 2000)
      return
    }
    call('feed')
    setEaten(eaten + 1)
    setFedUntil(Date.now() + FED_MS)
  }

  const n = countBy(agents)
  let mood = 'sleep'
  let headline = n.done > 0 ? '都完成啦' : '都在休息 zZ'
  if (feeding) {
    mood = 'fed'
    headline = fishLeft > 0 ? `好吃！还剩 ${fishLeft} 条` : '好吃！吃完啦'
  } else if (noFish) {
    headline = '没有小鱼干了，干完活再来'
  } else if (impatient) {
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

  // 气泡里的 agent：从需要关注的（不是空闲的）里面挑，按优先级排；记住当前是谁，数据更新时不跳回第一个
  const sorted = agents.slice().sort((x, y) => (PRIORITY[x.status] ?? 4) - (PRIORITY[y.status] ?? 4))
  const top = sorted.filter((a) => a.status !== 'idle' && a.status !== 'unknown')
  const pool = top.length > 0 ? top : sorted
  const index = Math.max(0, pool.findIndex((a) => a.id === focusId))
  const focus = pool[index]
  const next = () => {
    if (pool.length > 1) setFocusId(pool[(index + 1) % pool.length].id)
  }

  const quota = tightest(usage)
  const quotaColor = !quota ? '#5c6068' : quota.used >= 80 ? '#ff6b6b' : quota.used >= 50 ? '#ff9f1c' : '#5c6068'

  return (
    <View style={{ width: 212, height: screen.height, flexDirection: 'column', alignItems: 'center', padding: [8, 10, 0, 10], backgroundColor: '#0a0c12' }}>
      <Text style={{ width: 192, height: 26, lines: 1, textOverflow: 'ellipsis', fontSize: 18, fontWeight: 'bold', color: HEAD_COLOR[mood] || HEAD_COLOR.sleep, textAlign: 'center' }}>
        {headline}
      </Text>
      <Text style={{ width: 192, height: 12, lines: 1, fontSize: 10, color: quotaColor, textAlign: 'center' }}>
        {quota ? `${quota.agent} ${quota.name} ${Math.round(quota.used)}%${quota.resetsAt ? ' · ' + fmtReset(quota.resetsAt) + ' 后重置' : ''}` : ''}
      </Text>
      <View style={{ width: 192, height: 150, marginTop: 2, justifyContent: 'center', alignItems: 'center' }} onClick={next}>
        <Sprite key={skin + '-' + mood} frames={frames(skin, action)} duration={duration} style={{ width: 146, height: 146 }} />
      </View>
      {focus ? (
        <View
          onClick={next}
          style={{
            width: 192, height: 62, marginTop: 6, padding: [7, 10], borderRadius: 14, backgroundColor: '#16191f',
            flexDirection: 'column', borderLeftWidth: 4, borderLeftColor: COLOR[focus.status] || COLOR.unknown
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
        <Row align="center" onClick={feed} style={{ height: 22, paddingLeft: 8 }}>
          <Image src={asset('assets/fish.png')} style={{ width: 24, height: 12 }} />
          <Text style={{ marginLeft: 4, fontSize: 12, color: fishLeft > 0 ? '#e29640' : '#5c6068' }}>{fishLeft}</Text>
        </Row>
      </Row>
    </View>
  )
}
