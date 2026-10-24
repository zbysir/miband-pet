// 猫的第二屏「今日」：今天完成了几个任务（一个任务一条小鱼干）、Claude Code 花了多少钱，最近 7 天的柱状图，以及所有额度。
// 点小鱼干或钱数，柱状图换成对应的那项。
// 小鱼干是本插件 herdr.js 记的（useStore('stats')），钱数是 cost.js 记的（useStore('cost')），额度是 server.js 读的（useStore('usage')）。
import { View, Text, Image, Row, Col, Spacer, useStore, useState, useScreen, asset } from 'miband'

const WEEKDAYS = ['日', '一', '二', '三', '四', '五', '六']
const CHART_H = 40
const AGENT_NAME = { claude: 'Claude', codex: 'Codex' }
// 柱状图：小鱼干是橙色，钱是蓝色；today 是今天那根，other 是其它天
const METRIC = {
  done: { today: '#e29640', other: '#5c4a3a', label: (v) => (v > 0 ? String(v) : '') },
  usd: { today: '#6fb6ff', other: '#2b4060', label: (v) => (v > 0 ? fmtUsdShort(v) : '') }
}

// 钱数：大字用，如 $7.25、$79.3、$412
export function fmtUsd(v) {
  if (v < 100) return '$' + v.toFixed(2)
  if (v < 1000) return '$' + v.toFixed(1)
  return '$' + Math.round(v)
}

// 钱数：小地方用，如 $7.3、$79、$1.2k
export function fmtUsdShort(v) {
  if (v < 10) return '$' + v.toFixed(1)
  if (v < 1000) return '$' + Math.round(v)
  return '$' + (v / 1000).toFixed(1) + 'k'
}

export function fmtReset(sec) {
  const left = Math.max(0, sec - Date.now() / 1000)
  if (left >= 86400) return Math.floor(left / 86400) + ' 天'
  const h = Math.floor(left / 3600)
  const m = Math.floor((left % 3600) / 60)
  return h + ':' + String(m).padStart(2, '0')
}

// days: [{ date, weekday, value }]
function Chart({ days, metric }) {
  const m = METRIC[metric]
  const top = Math.max(metric === 'usd' ? 0.01 : 1, ...days.map((d) => d.value))
  return (
    <Row justify="between" align="end" style={{ width: 192, marginTop: 8 }}>
      {days.map((d, i) => {
        const today = i === days.length - 1
        return (
          <Col key={d.date} align="center" style={{ width: 26 }}>
            <Text style={{ fontSize: 9, color: today ? m.today : '#8a8f98', lines: 1 }}>{m.label(d.value)}</Text>
            <View style={{ width: 14, height: Math.max(2, Math.round((d.value / top) * CHART_H)), marginTop: 2, borderRadius: 3, backgroundColor: today ? m.today : m.other }} />
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

// 额度列表。单独一个组件：切小鱼干 / 钱数时不用重算这里
function Quotas() {
  const usage = useStore('usage') || []
  const quotas = []
  for (const src of usage) for (const w of src.windows || []) quotas.push({ label: `${AGENT_NAME[src.agent] || src.agent} ${w.name}`, w })
  return (
    <Col>
      <Text style={{ fontSize: 12, color: '#8a8f98', marginTop: 10 }}>额度</Text>
      {quotas.length > 0 ? (
        quotas.slice(0, 4).map((q) => <Quota key={q.label} label={q.label} w={q.w} />)
      ) : (
        <Text style={{ fontSize: 10, color: '#5c6068', marginTop: 4 }}>还没有额度数据。Claude Code 的要在 App 里打开「显示 Claude Code 额度」</Text>
      )}
    </Col>
  )
}

// ready 为 false 时只画一块同样大的空白（打开时先不画今日页，见 index.jsx）。空白和今日页是同一个组件交出的同一个根节点，
// 换过来时 Pager 的几页结构不变，猫那页不会跟着重建
export function Today({ ready }) {
  // 整页一样大，放在 Pager 里当第二屏
  const screen = useScreen()
  if (!ready) return <Col style={{ width: 212, height: screen.height, backgroundColor: '#0a0c12' }} />
  const now = new Date()

  return (
    <Col style={{ width: 212, height: screen.height, padding: [10, 10], backgroundColor: '#0a0c12' }}>
      <Row align="center">
        <Text style={{ fontSize: 18, fontWeight: 'bold' }}>今日</Text>
        <Spacer />
        <Text style={{ fontSize: 11, color: '#8a8f98' }}>{`${now.getMonth() + 1}月${now.getDate()}日 周${WEEKDAYS[now.getDay()]}`}</Text>
      </Row>
      <Metrics />
      <Quotas />
      <Spacer />
      <Text style={{ fontSize: 9, color: '#3a3d44', textAlign: 'center' }}>往下翻回到猫</Text>
    </Col>
  )
}

// 小鱼干、钱数两块和柱状图。点哪块柱状图就换成哪项；选中的状态在这里，切换时只重算这一部分
function Metrics() {
  const stats = useStore('stats') || { today: { done: 0 }, days: [] }
  const cost = useStore('cost')
  const [picked, setMetric] = useState('done')
  const metric = cost ? picked : 'done'
  const days = metric === 'usd' ? cost.days.map((d) => ({ ...d, value: d.usd })) : stats.days.map((d) => ({ ...d, value: d.done }))
  // 选中的那项下面一条亮色短线。不换整块的底色：小米手环 9 Pro 上元素换了样式要重建（连同里面的图片、文字），
  // 换一条线只重建这一个元素，切换快很多
  const mark = (m) => <View style={{ width: 20, height: 2, marginTop: 3, borderRadius: 1, backgroundColor: metric === m ? METRIC[m].today : '#22252c' }} />
  return (
    <>
      {cost ? (
        <Row align="center" style={{ marginTop: 6 }}>
          <Col style={{ width: 92, padding: [4, 6] }} onClick={() => setMetric('done')}>
            <Row align="center">
              <Image src={asset('assets/fish.png')} style={{ width: 28, height: 14 }} />
              <Text style={{ marginLeft: 6, fontSize: 20, fontWeight: 'bold', color: '#e29640' }}>{String(stats.today.done)}</Text>
            </Row>
            <Text style={{ fontSize: 11, color: '#8a8f98' }}>完成的任务</Text>
            {mark('done')}
          </Col>
          <Spacer />
          <Col style={{ width: 92, padding: [4, 6] }} onClick={() => setMetric('usd')}>
            <Text style={{ fontSize: 20, fontWeight: 'bold', color: METRIC.usd.today, lines: 1 }}>{fmtUsd(cost.today)}</Text>
            <Text style={{ fontSize: 11, color: '#8a8f98' }}>Claude 花的钱</Text>
            {mark('usd')}
          </Col>
        </Row>
      ) : (
        <Row align="center" style={{ marginTop: 8 }}>
          <Image src={asset('assets/fish.png')} style={{ width: 40, height: 20 }} />
          <Col style={{ marginLeft: 10 }}>
            <Text style={{ fontSize: 20, fontWeight: 'bold', color: '#e29640' }}>{`${stats.today.done} 条小鱼干`}</Text>
            <Text style={{ fontSize: 11, color: '#8a8f98' }}>{`今天完成了 ${stats.today.done} 个任务`}</Text>
          </Col>
        </Row>
      )}
      {days.length > 0 && <Chart days={days} metric={metric} />}
    </>
  )
}
