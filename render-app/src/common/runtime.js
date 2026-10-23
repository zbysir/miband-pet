// 插件运行时：在手环上跑插件。提供 JSX（__h / __Fragment）、hooks、基础组件，
// 把插件渲染出的元素树转成页面模板能画的节点树（格式见 tools/gen_template.py）。
// 每个插件一个运行时实例：createRuntime(host) 。Mac 上的测试也用这个文件（server/internal/plugin）。
//
// host（由页面提供）：
//   render(tree, timing) 交一棵新树（对象）；timing 是这次渲染各步的耗时（毫秒）：
//                       { at: 开始渲染的时间, comp: 插件组件函数, build: 转成节点, style: 拼样式字符串 }
//   error(msg)          渲染或事件处理出错
//   log(level, msg)
//   asset(path)         插件目录里的图片 → 手环上的路径
//   vibrate(mode)       short / long
//   storageGet(key) / storageSet(key, value)   插件自己的持久化数据（同步）
//   call(name, args)    让 Mac 做事（Mac 上注册的功能，如 feed）；不等结果，Mac 会更新对应的数据
//   scope               插件名 + ':'；useStore(name) 读本插件 server.js publish 的（scope + name）
//
// 返回：{ api, mount(component), setStore(name, value), dispatch(id, type), swipe(dir) }
// api 就是插件里 import ... from 'miband' 拿到的东西。
module.exports = function createRuntime(host) {
  'use strict'

  // ---------- 元素 ----------

  const Fragment = { fragment: true }

  function h(type, props) {
    const children = []
    for (let i = 2; i < arguments.length; i++) children.push(arguments[i])
    props = props || {}
    return { type, props, key: props.key, children }
  }

  // ---------- 样式：对象 / 字符串 / 数组 → {属性: 值} ----------

  const UNITLESS = { flex: 1, flexGrow: 1, flexShrink: 1, opacity: 1, lines: 1, fontWeight: 1, zIndex: 1 }

  // 属性名就那么几十个，转过的记下来（手环上正则替换很慢，每次渲染每个节点都要转）
  const kebabCache = {}
  function kebab(k) {
    let v = kebabCache[k]
    if (v === undefined) {
      v = k.replace(/[A-Z]/g, (m) => '-' + m.toLowerCase())
      kebabCache[k] = v
    }
    return v
  }

  function cssValue(k, v) {
    if (typeof v === 'number') return UNITLESS[k] ? String(v) : v + 'px'
    if (Array.isArray(v)) return v.map((x) => cssValue(k, x)).join(' ') // padding: [4, 8]
    return String(v)
  }

  // 写简写（padding）时去掉之前写的单个方向，和 CSS 的覆盖顺序一致
  function put(out, k, v) {
    if (k === 'padding' || k === 'margin') {
      ;['top', 'right', 'bottom', 'left'].forEach((side) => delete out[k + '-' + side])
    }
    out[k] = v
  }

  function styleMap(s, out) {
    if (!s) return out
    if (Array.isArray(s)) {
      s.forEach((x) => styleMap(x, out))
      return out
    }
    if (typeof s === 'string') {
      s.split(';').forEach((part) => {
        const i = part.indexOf(':')
        if (i > 0) put(out, part.slice(0, i).trim(), part.slice(i + 1).trim())
      })
      return out
    }
    for (const k in s) {
      const v = s[k]
      if (v === undefined || v === null || v === false) continue
      put(out, kebab(k), cssValue(k, v))
    }
    return out
  }

  // 手环的内联样式不认多值简写（padding:7px 10px 不生效），展开成四个方向
  function expandBox(m) {
    ;['padding', 'margin'].forEach((k) => {
      const v = m[k]
      if (v === undefined) return
      const p = String(v).trim().split(/\s+/)
      if (p.length < 2) return
      const [top, right = top, bottom = top, left = right] = p
      delete m[k]
      // 写在简写后面的单个方向（如 paddingLeft）优先
      if (m[k + '-top'] === undefined) m[k + '-top'] = top
      if (m[k + '-right'] === undefined) m[k + '-right'] = right
      if (m[k + '-bottom'] === undefined) m[k + '-bottom'] = bottom
      if (m[k + '-left'] === undefined) m[k + '-left'] = left
    })
    return m
  }

  function styleString(m) {
    m = expandBox(m)
    return Object.keys(m).map((k) => k + ':' + m[k]).join(';')
  }

  // ---------- 组件实例和 hooks ----------
  //
  // 只重算变了的组件（和 React 一样）：手环上的 JS 很慢，整个插件从根重算一遍要两三百毫秒。
  // 每个组件实例记着上次的 props、上次返回的元素（out）、展开成的节点（nodes），以及展开时登记的东西（rec：
  // 节点 id、点击处理、滑动处理、用到的子组件）。
  //   - setState 只标记自己 dirty，祖先标记 below（下面有变了的）；Mac 推来的数据变了，标记读过这项数据的组件
  //   - 重算时：自己没变、下面也没变、props 一样 → 直接用上次的节点，把 rec 重新登记一遍
  //            自己没变、下面有变的 → 不调组件函数，把上次返回的元素重新展开（没变的子组件 props 还是同一个对象，接着复用）
  //            其它 → 调组件函数

  const instances = new Map() // 路径 → 组件实例
  let current = null // 正在跑函数的组件（hooks 用）
  let owner = null // 正在展开的组件（新建的子组件记它当 parent）
  let hookIndex = 0
  let seen = null
  let effects = []
  let compMs = 0 // 这次渲染里插件组件函数一共跑了多久
  let handlers = {}
  let swipeHandlers = []
  let rec = null // 正在展开的组件的登记
  const stores = {}

  function newRec() {
    return { ids: [], handlers: [], swipes: [], seen: [], kids: [] }
  }

  // 复用上次的节点时，把它展开时登记的东西再登记一遍
  function replay(r) {
    for (let i = 0; i < r.ids.length; i += 2) usedIds.set(r.ids[i], r.ids[i + 1])
    for (let i = 0; i < r.handlers.length; i += 2) handlers[r.handlers[i]] = r.handlers[i + 1]
    r.swipes.forEach((fn) => swipeHandlers.push(fn))
    r.seen.forEach((k) => seen.add(k))
    r.kids.forEach(replay)
  }

  function markDirty(inst) {
    inst.dirty = true
    for (let p = inst.parent; p && !p.below; p = p.parent) p.below = true
  }

  // props 一样：每一项是同一个值；children 都是空的也算一样（每次渲染都是新的空数组）
  function sameProps(a, b) {
    const ka = Object.keys(a)
    if (ka.length !== Object.keys(b).length) return false
    for (const k of ka) {
      if (Object.is(a[k], b[k])) continue
      if (k === 'children' && Array.isArray(a[k]) && Array.isArray(b[k]) && a[k].length === 0 && b[k].length === 0) continue
      return false
    }
    return true
  }

  function hook(init) {
    if (!current) throw new Error('hooks 只能在组件函数里调用')
    const list = current.hooks
    if (hookIndex >= list.length) list.push(init())
    return list[hookIndex++]
  }

  function depsChanged(a, b) {
    if (!a || !b || a.length !== b.length) return true
    for (let i = 0; i < a.length; i++) if (!Object.is(a[i], b[i])) return true
    return false
  }

  function useState(initial) {
    const s = hook(() => ({ v: typeof initial === 'function' ? initial() : initial }))
    if (!s.set) {
      const inst = current
      s.set = (next) => {
        const v = typeof next === 'function' ? next(s.v) : next
        if (Object.is(v, s.v)) return
        s.v = v
        markDirty(inst)
        scheduleRender()
      }
    }
    return [s.v, s.set]
  }

  function useRef(initial) {
    return hook(() => ({ current: initial }))
  }

  function useMemo(fn, deps) {
    const m = hook(() => ({ deps: undefined, v: undefined }))
    if (depsChanged(m.deps, deps)) {
      m.v = fn()
      m.deps = deps
    }
    return m.v
  }

  function useCallback(fn, deps) {
    return useMemo(() => fn, deps)
  }

  function useEffect(fn, deps) {
    const e = hook(() => ({ deps: undefined, cleanup: null }))
    if (deps !== undefined && !depsChanged(e.deps, deps)) return
    e.deps = deps
    effects.push(() => {
      if (typeof e.cleanup === 'function') e.cleanup()
      const c = fn()
      e.cleanup = typeof c === 'function' ? c : null
    })
  }

  function useInterval(fn, ms) {
    const ref = useRef(fn)
    ref.current = fn
    useEffect(() => {
      if (ms == null) return
      const t = setInterval(() => ref.current(), ms)
      return () => clearInterval(t)
    }, [ms])
  }

  // Mac 推过来的数据，变了会重新渲染。只能读本插件 server.js publish 的和基座给的（BASE_STORES），
  // 插件之间不共用数据
  const BASE_STORES = ['screen', 'band']
  function useStore(name) {
    const key = BASE_STORES.indexOf(name) >= 0 || !host.scope ? name : host.scope + name
    if (current) current.reads.add(key) // 这项数据变了要重算这个组件
    return stores[key]
  }

  // 滑动处理记在组件上：组件没重跑、复用上次的节点时，上次登记的照样生效
  function useSwipe(fn) {
    if (current) current.swipes.push(fn)
  }

  function useScreen() {
    return useStore('screen') || { width: 212, height: 303 }
  }

  // 插件的配置：manifest.json 里 settings 声明的，用户在 App 里改，Mac 推过来（没改过的是默认值）
  function useConfig() {
    return useStore('config') || {}
  }

  function unmount(inst) {
    inst.hooks.forEach((x) => {
      if (x && typeof x.cleanup === 'function') x.cleanup()
    })
  }

  // ---------- 基础组件 ----------

  const View = 'view'
  const Text = 'text'
  const Image = 'image'
  const Sprite = 'sprite'

  // 翻页容器：每个子元素是一页，手环上是原生的 swiper，翻页跟着手指走。只能是插件界面的最外层。
  // 不传 index 时自己记住翻到了第几页（重新渲染不跳回去）；传了 index 就由插件控制（配合 onChange 用）。
  // onChange(index) 在翻页后调用
  function Pager(props) {
    const [inner, setInner] = useState(0)
    const index = props.index !== undefined ? props.index : inner
    const onChange = props.onChange
    return h('pager', Object.assign({}, props, {
      index,
      onChange: (i) => {
        setInner(i)
        if (onChange) onChange(i)
      }
    }), props.children)
  }

  const JUSTIFY = { start: 'flex-start', end: 'flex-end', center: 'center', between: 'space-between', around: 'space-around' }
  const ALIGN = { start: 'flex-start', end: 'flex-end', center: 'center', stretch: 'stretch', baseline: 'baseline' }

  function box(dir) {
    return function (props) {
      const base = { flexDirection: dir }
      if (props.justify) base.justifyContent = JUSTIFY[props.justify] || props.justify
      if (props.align) base.alignItems = ALIGN[props.align] || props.align
      if (props.padding !== undefined) base.padding = props.padding
      if (props.flex !== undefined) base.flex = props.flex
      const p = Object.assign({}, props, { style: [base, props.style], __gap: props.gap, __dir: dir })
      delete p.justify
      delete p.align
      delete p.padding
      delete p.gap
      return h('view', p, props.children)
    }
  }
  const Row = box('row')
  const Col = box('column')

  function Spacer() {
    return h('view', { style: { flexGrow: 1 } })
  }

  // ---------- 渲染：元素树 → 节点树 ----------

  const TAGS = { view: 'v', text: 'x', image: 'm', sprite: 'a', pager: 'w' }
  let ids = new Map() // 路径 → 短 id，跨渲染保持稳定（页面按 id 复用元素）
  let idSeq = 0
  let usedIds = null

  function idFor(path) {
    let id = ids.get(path)
    if (!id) {
      id = 'n' + (++idSeq).toString(36)
      ids.set(path, id)
    }
    usedIds.set(path, id)
    rec.ids.push(path, id)
    return id
  }

  function textOf(children) {
    let s = ''
    const walk = (c) => {
      if (c === null || c === undefined || c === false || c === true) return
      if (Array.isArray(c)) return c.forEach(walk)
      if (typeof c === 'object') return walk(c.children)
      s += String(c)
    }
    walk(children)
    return s
  }

  function textNode(text, path) {
    return { t: 'x', i: idFor(path), sm: { color: '#ffffff', 'font-size': '14px' }, x: text }
  }

  // 展开一个元素（或数组、文字），返回节点数组
  function expand(el, path) {
    if (el === null || el === undefined || el === false || el === true) return []
    if (Array.isArray(el)) {
      const out = []
      el.forEach((c, i) => {
        const k = c && typeof c === 'object' && c.key != null ? 'k' + c.key : String(i)
        out.push.apply(out, expand(c, path + '.' + k))
      })
      return out
    }
    if (typeof el !== 'object') return [textNode(String(el), path)]
    if (el.type === Fragment) return expand(el.children, path)
    if (typeof el.type === 'function') {
      const key = path + ':' + (el.type.name || 'C')
      const props = Object.assign({}, el.props, { children: el.children.length === 1 ? el.children[0] : el.children })
      let inst = instances.get(key)
      if (inst && inst.type === el.type && inst.nodes && !inst.dirty && sameProps(inst.props, props)) {
        if (!inst.below) {
          replay(inst.rec)
          rec.kids.push(inst.rec)
          return inst.nodes
        }
        return expandInst(inst, key, false)
      }
      if (!inst || inst.type !== el.type) {
        if (inst) unmount(inst)
        inst = { type: el.type, hooks: [], parent: owner, reads: new Set(), swipes: [], dirty: false, below: false }
        instances.set(key, inst)
      }
      inst.props = props
      return expandInst(inst, key, true)
    }
    const t = TAGS[el.type]
    if (!t) throw new Error('不认识的组件：' + el.type)
    const p = el.props
    const node = { t, i: idFor(path), sm: {} }
    if (t === 'x') {
      node.sm = { color: '#ffffff', 'font-size': '14px' }
      node.x = textOf(el.children)
    }
    styleMap(p.style, node.sm)
    if (t === 'm') node.u = p.src || ''
    if (t === 'a') {
      node.f = (p.frames || []).map((src) => ({ src }))
      node.d = typeof p.duration === 'number' ? p.duration + 'ms' : p.duration || '1000ms'
    }
    if (t === 'w') {
      node.vt = p.vertical !== false
      node.ix = p.index || 0
    }
    let e = 0
    if (p.onClick) e |= 1
    if (p.onLongPress) e |= 2
    if (p.onChange && t === 'w') e |= 4
    if (e) {
      node.e = e
      const hd = { click: p.onClick, longpress: p.onLongPress, change: p.onChange }
      handlers[node.i] = hd
      rec.handlers.push(node.i, hd)
    }
    if (t === 'v' || t === 'w') {
      node.c = expand(el.children, path)
      if (p.__gap) {
        const prop = p.__dir === 'row' ? 'margin-left' : 'margin-top'
        node.c.forEach((c, i) => {
          if (i > 0 && c.sm[prop] === undefined) c.sm[prop] = cssValue('margin', p.__gap)
        })
      }
    }
    return [node]
  }

  // 展开一个组件实例：run 时调组件函数，否则用它上次返回的元素
  function expandInst(inst, key, run) {
    const parentRec = rec
    const prevOwner = owner
    rec = newRec()
    rec.seen.push(key)
    seen.add(key)
    if (run) {
      const prev = current
      const prevIndex = hookIndex
      current = inst
      hookIndex = 0
      inst.reads = new Set()
      inst.swipes = []
      const t0 = Date.now()
      try {
        inst.out = inst.type(inst.props)
      } finally {
        compMs += Date.now() - t0
        current = prev
        hookIndex = prevIndex
      }
    }
    inst.swipes.forEach((fn) => swipeHandlers.push(fn))
    rec.swipes = inst.swipes
    owner = inst
    let nodes
    try {
      nodes = expand(inst.out, key)
    } finally {
      owner = prevOwner
    }
    inst.rec = rec
    inst.nodes = nodes
    inst.dirty = false
    inst.below = false
    parentRec.kids.push(rec)
    rec = parentRec
    return nodes
  }

  // 节点 → 交给页面的样子。复用的节点上次算过，直接用（页面也靠同一个对象认出没变的部分）
  function finish(node) {
    if (node.fin) return node.fin
    const out = { t: node.t, i: node.i, s: styleString(node.sm) }
    if (node.x !== undefined) out.x = node.x
    if (node.u !== undefined) out.u = node.u
    if (node.f !== undefined) out.f = node.f
    if (node.d !== undefined) out.d = node.d
    if (node.vt !== undefined) out.vt = node.vt
    if (node.ix !== undefined) out.ix = node.ix
    if (node.e) out.e = node.e
    if (node.c) out.c = node.c.map(finish)
    node.fin = out
    return out
  }

  function errText(e) {
    return e && e.stack ? String(e.stack) : String(e && e.message ? e.message : e)
  }

  let root = null
  let scheduled = false

  function render() {
    scheduled = false
    if (!root) return
    seen = new Set()
    effects = []
    handlers = {}
    swipeHandlers = []
    usedIds = new Map()
    compMs = 0
    rec = newRec()
    owner = null
    const t0 = Date.now()
    let nodes
    try {
      nodes = expand(h(root, {}), 'r')
    } catch (e) {
      // 出错的那次展开到一半，组件记的节点不完整：下次都重跑
      instances.forEach((inst) => (inst.nodes = null))
      host.error('渲染出错：' + errText(e))
      return
    }
    ids = usedIds
    instances.forEach((inst, k) => {
      if (!seen.has(k)) {
        unmount(inst)
        instances.delete(k)
      }
    })
    const t1 = Date.now()
    let tree = null
    if (nodes.length === 1 && (nodes[0].t === 'v' || nodes[0].t === 'w')) tree = finish(nodes[0])
    else if (nodes.length > 0) tree = finish({ t: 'v', i: 'root', sm: { 'flex-direction': 'column' }, c: nodes })
    host.render(tree, { at: t0, comp: compMs, build: t1 - t0 - compMs, style: Date.now() - t1 })
    const run = effects
    effects = []
    run.forEach((f) => {
      try {
        f()
      } catch (e) {
        host.error('effect 出错：' + errText(e))
      }
    })
  }

  function scheduleRender() {
    if (scheduled) return
    scheduled = true
    // 点击后已经同步渲染过的话，这里就不用再渲染一遍
    setTimeout(() => {
      if (scheduled) render()
    }, 0)
  }

  // ---------- 给插件用的 API（import ... from 'miband'） ----------

  const api = {
    h,
    Fragment,
    View,
    Text,
    Image,
    Sprite,
    Pager,
    Row,
    Col,
    Spacer,
    useState,
    useRef,
    useMemo,
    useCallback,
    useEffect,
    useInterval,
    useStore,
    useSwipe,
    useScreen,
    useConfig,
    asset: (path) => host.asset(path),
    vibrate: (mode) => host.vibrate(mode || 'short'),
    call: (name, ...args) => host.call(name, args),
    storage: {
      get: (key) => host.storageGet(key),
      set: (key, value) => host.storageSet(key, value)
    },
    console: {
      log: (...a) => host.log('info', a.map(String).join(' ')),
      warn: (...a) => host.log('warn', a.map(String).join(' ')),
      error: (...a) => host.log('error', a.map(String).join(' '))
    }
  }

  return {
    api,
    mount(component) {
      root = component
      scheduleRender()
    },
    setStore(name, value) {
      stores[name] = value
      instances.forEach((inst) => {
        if (inst.reads.has(name)) markDirty(inst)
      })
      scheduleRender()
    },
    // 立刻重新渲染（页面切回这个插件时用）
    render,
    // 点击、滑动处理完马上渲染，不等下一轮计时器（手环上计时器要等几十毫秒）
    dispatch(id, type, arg) {
      const hd = handlers[id]
      const fn = hd && hd[type]
      if (!fn) return
      try {
        fn(arg)
      } catch (e) {
        host.error('事件处理出错：' + errText(e))
      }
      if (scheduled) render()
    },
    swipe(dir) {
      swipeHandlers.forEach((fn) => {
        try {
          fn(dir)
        } catch (e) {
          host.error('滑动处理出错：' + errText(e))
        }
      })
      if (scheduled) render()
    }
  }
}
