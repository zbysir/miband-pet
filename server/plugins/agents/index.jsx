// Agents：按状态排序列出每个 agent 的项目和任务。
// 手环上新建节点很慢（约 10ms 一个，见 docs/render-spike.md），所以不做长列表滚动，改成分页：上下滑翻页。
import { View, Text, Row, Col, Spacer, useStore, useState, useSwipe, useScreen } from 'miband'

const ORDER = { blocked: 0, working: 1, done: 2, idle: 3, unknown: 4 }
const STATUS_TEXT = { blocked: '待处理', working: '运行中', done: '完成', idle: '空闲', unknown: '未知' }
const COLOR = { blocked: '#ff9f1c', working: '#2ea0ff', done: '#3ddc84', idle: '#5c6068', unknown: '#5c6068' }
const CARD_H = 52
const CARD_GAP = 6
const HEADER_H = 58 // 标题 + 统计 + 间距

function Card({ agent }) {
  const color = COLOR[agent.status] || COLOR.unknown
  return (
    <View style={{ width: 196, height: CARD_H, backgroundColor: '#16191f', borderRadius: 12, flexDirection: 'row' }}>
      <View style={{ width: 4, height: CARD_H, borderTopLeftRadius: 12, borderBottomLeftRadius: 12, backgroundColor: color }} />
      <Col justify="between" style={{ width: 192, padding: [6, 9] }}>
        <Row justify="between" align="center">
          <Text style={{ width: 120, fontSize: 14, fontWeight: 'bold', lines: 1, textOverflow: 'ellipsis' }}>{agent.project || agent.agent || '?'}</Text>
          <Text style={{ fontSize: 11, color: agent.status === 'idle' ? '#8a8f98' : color }}>{STATUS_TEXT[agent.status] || STATUS_TEXT.unknown}</Text>
        </Row>
        <Text style={{ width: 174, fontSize: 12, color: '#8a8f98', lines: 1, textOverflow: 'ellipsis' }}>{agent.title || ''}</Text>
      </Col>
    </View>
  )
}

export default function Agents() {
  const agents = useStore('agents') || []
  const screen = useScreen()
  const [page, setPage] = useState(0)

  // 状态 → 项目 → id：Mac 推来的列表顺序不固定，同一个项目的几个 agent 要靠 id 排稳，不然会在页与页之间跳
  const sorted = agents.slice().sort((x, y) =>
    (ORDER[x.status] ?? 4) - (ORDER[y.status] ?? 4) || (x.project || '').localeCompare(y.project || '') || (x.id < y.id ? -1 : x.id > y.id ? 1 : 0)
  )
  const perPage = Math.max(1, Math.floor((screen.height - HEADER_H + CARD_GAP) / (CARD_H + CARD_GAP)))
  const pages = Math.max(1, Math.ceil(sorted.length / perPage))
  const cur = Math.min(page, pages - 1)

  // 上滑看下一页，下滑回上一页
  useSwipe((dir) => {
    if (dir === 'up') setPage(Math.min(cur + 1, pages - 1))
    if (dir === 'down') setPage(Math.max(cur - 1, 0))
  })

  const n = { blocked: 0, working: 0, done: 0 }
  for (const a of agents) if (n[a.status] !== undefined) n[a.status]++

  return (
    <Col style={{ width: 212, height: screen.height, padding: 8, backgroundColor: '#000000' }}>
      <Row justify="between" align="center" style={{ height: 26 }}>
        <Text style={{ fontSize: 18, fontWeight: 'bold' }}>Agents</Text>
        {pages > 1 && <Text style={{ fontSize: 11, color: '#8a8f98' }}>{`${cur + 1}/${pages}`}</Text>}
      </Row>
      <Row gap={8} style={{ marginTop: 2 }}>
        {n.blocked > 0 && <Text style={{ fontSize: 12, color: COLOR.blocked }}>{`${n.blocked} 待处理`}</Text>}
        <Text style={{ fontSize: 12, color: COLOR.working }}>{`${n.working} 运行`}</Text>
        {n.done > 0 && <Text style={{ fontSize: 12, color: COLOR.done }}>{`${n.done} 完成`}</Text>}
        <Spacer />
        <Text style={{ fontSize: 12, color: '#5c6068' }}>{`共 ${agents.length}`}</Text>
      </Row>
      <Col gap={CARD_GAP} style={{ marginTop: 8 }}>
        {sorted.slice(cur * perPage, (cur + 1) * perPage).map((a) => (
          <Card key={a.id} agent={a} />
        ))}
      </Col>
      {agents.length === 0 && <Text style={{ marginTop: 30, fontSize: 13, color: '#5c6068', textAlign: 'center' }}>没有 agent</Text>}
    </Col>
  )
}
