// 猫的第二屏「今日」：今天完成了几个任务（一个任务一条小鱼干），最近 7 天的柱状图，以及所有额度。
// 小鱼干是本插件 herdr.js 记的（useStore('stats')），额度是本插件 mac.js 读的（useStore('usage')）。
import { View, Text, Image, Row, Col, Spacer, useStore, useScreen, asset } from 'miband'

const WEEKDAYS = ['日', '一', '二', '三', '四', '五', '六']
const CHART_H = 40
const AGENT_NAME = { claude: 'Claude', codex: 'Codex' }

export function fmtReset(sec) {
  const left = Math.max(0, sec - Date.now() / 1000)
  if (left >= 86400) return Math.floor(left / 86400) + ' 天'
  const h = Math.floor(left / 3600)
  const m = Math.floor((left % 3600) / 60)
  return h + ':' + String(m).padStart(2, '0')
}

function Chart({ days }) {
  const top = Math.max(1, ...days.map((d) => d.done))
  return (
    <Row justify="between" align="end" style={{ width: 192, marginTop: 8 }}>
      {days.map((d, i) => {
        const today = i === days.length - 1
        return (
          <Col key={d.date} align="center" style={{ width: 24 }}>
            <Text style={{ fontSize: 9, color: today ? '#e29640' : '#8a8f98' }}>{d.done > 0 ? d.done : ''}</Text>
            <View style={{ width: 14, height: Math.max(2, Math.round((d.done / top) * CHART_H)), marginTop: 2, borderRadius: 3, backgroundColor: today ? '#e29640' : '#5c4a3a' }} />
            <Text style={{ fontSize: 10, marginTop: 3, color: today ? '#ffffff' : '#5c6068' }}>{d.weekday}</Text>
          </Col>
        )
      })}
    </Row>
  )
}

function Quota({ label, w }) {
  const pct = Math.min(100, Math.max(0, w.used))
  const color = w.used >= 80 ? '#ff6b6b' : w.used >= 50 ? '#ff9f1c' : '#3ddc84'
  return (
    <Row align="center" style={{ width: 192, height: 16, marginTop: 4 }}>
      <Text style={{ width: 66, fontSize: 11, color: '#c9ced8', lines: 1 }}>{label}</Text>
      <View style={{ width: 62, height: 6, borderRadius: 3, backgroundColor: '#22252c' }}>
        <View style={{ width: Math.max(2, Math.round((pct / 100) * 62)), height: 6, borderRadius: 3, backgroundColor: color }} />
      </View>
      <Spacer />
      <Text style={{ fontSize: 10, color: '#8a8f98' }}>{`${Math.round(w.used)}%${w.resetsAt ? ' ' + fmtReset(w.resetsAt) : ''}`}</Text>
    </Row>
  )
}

export function Today() {
  // 整页一样大，放在 Pager 里当第二屏
  const stats = useStore('stats') || { today: { done: 0 }, days: [] }
  const usage = useStore('usage') || []
  const screen = useScreen()
  const now = new Date()
  const quotas = []
  for (const src of usage) for (const w of src.windows || []) quotas.push({ label: `${AGENT_NAME[src.agent] || src.agent} ${w.name}`, w })

  return (
    <Col style={{ width: 212, height: screen.height, padding: [10, 10], backgroundColor: '#0a0c12' }}>
      <Row align="center">
        <Text style={{ fontSize: 18, fontWeight: 'bold' }}>今日</Text>
        <Spacer />
        <Text style={{ fontSize: 11, color: '#8a8f98' }}>{`${now.getMonth() + 1}月${now.getDate()}日 周${WEEKDAYS[now.getDay()]}`}</Text>
      </Row>
      <Row align="center" style={{ marginTop: 8 }}>
        <Image src={asset('assets/fish.png')} style={{ width: 40, height: 20 }} />
        <Col style={{ marginLeft: 10 }}>
          <Text style={{ fontSize: 20, fontWeight: 'bold', color: '#e29640' }}>{`${stats.today.done} 条小鱼干`}</Text>
          <Text style={{ fontSize: 11, color: '#8a8f98' }}>{`今天完成了 ${stats.today.done} 个任务`}</Text>
        </Col>
      </Row>
      {stats.days.length > 0 && <Chart days={stats.days} />}
      <Text style={{ fontSize: 12, color: '#8a8f98', marginTop: 10 }}>额度</Text>
      {quotas.length > 0 ? (
        quotas.slice(0, 4).map((q) => <Quota key={q.label} label={q.label} w={q.w} />)
      ) : (
        <Text style={{ fontSize: 10, color: '#5c6068', marginTop: 4 }}>还没有额度数据。Claude Code 的要在 App 里打开「显示 Claude Code 额度」</Text>
      )}
      <Spacer />
      <Text style={{ fontSize: 9, color: '#3a3d44', textAlign: 'center' }}>往下翻回到猫</Text>
    </Col>
  )
}
