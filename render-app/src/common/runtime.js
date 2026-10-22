// 插件运行时：在手环上跑插件。提供 JSX（__h / __Fragment）、hooks、基础组件，
// 把插件渲染出的元素树转成页面模板能画的节点树（格式见 tools/gen_template.py）。
// 每个插件一个运行时实例：createRuntime(host) 。Mac 上的测试也用这个文件（server/internal/plugin）。
//
// host（由页面提供）：
//   render(tree)        交一棵新树（对象）
//   error(msg)          渲染或事件处理出错
//   log(level, msg)
//   asset(path)         插件目录里的图片 → 手环上的路径
//   vibrate(mode)       short / long
//   storageGet(key) / storageSet(key, value)   插件自己的持久化数据（同步）
//   call(name, args)    让 Mac 做事（Mac 上注册的功能，如 feed）；不等结果，Mac 会更新对应的数据
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

  function kebab(k) {
    return k.replace(/[A-Z]/g, (m) => '-' + m.toLowerCase())
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

  const instances = new Map() // 路径 → {type, hooks}
  let current = null
  let hookIndex = 0
  let seen = null
  let effects = []
  let handlers = {}
  let swipeHandlers = []
  const stores = {}

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
      s.set = (next) => {
        const v = typeof next === 'function' ? next(s.v) : next
        if (Object.is(v, s.v)) return
        s.v = v
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

  // Mac 推过来的数据：agents、settings 等，变了会重新渲染
  function useStore(name) {
    return stores[name]
  }

  function useSwipe(fn) {
    swipeHandlers.push(fn)
  }

  function useScreen() {
    return stores.screen || { width: 212, height: 303 }
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

  const TAGS = { view: 'v', text: 'x', image: 'm', sprite: 'a' }
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
      let inst = instances.get(key)
      if (!inst || inst.type !== el.type) {
        if (inst) unmount(inst)
        inst = { type: el.type, hooks: [] }
        instances.set(key, inst)
      }
      seen.add(key)
      const props = Object.assign({}, el.props, { children: el.children.length === 1 ? el.children[0] : el.children })
      const prev = current
      const prevIndex = hookIndex
      current = inst
      hookIndex = 0
      let out
      try {
        out = el.type(props)
      } finally {
        current = prev
        hookIndex = prevIndex
      }
      return expand(out, key)
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
    let e = 0
    if (p.onClick) e |= 1
    if (p.onLongPress) e |= 2
    if (e) {
      node.e = e
      handlers[node.i] = { click: p.onClick, longpress: p.onLongPress }
    }
    if (t === 'v') {
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

  function finish(node) {
    const out = { t: node.t, i: node.i, s: styleString(node.sm) }
    if (node.x !== undefined) out.x = node.x
    if (node.u !== undefined) out.u = node.u
    if (node.f !== undefined) out.f = node.f
    if (node.d !== undefined) out.d = node.d
    if (node.e) out.e = node.e
    if (node.c) out.c = node.c.map(finish)
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
    let nodes
    try {
      nodes = expand(h(root, {}), 'r')
    } catch (e) {
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
    let tree = null
    if (nodes.length === 1 && nodes[0].t === 'v') tree = finish(nodes[0])
    else if (nodes.length > 0) tree = finish({ t: 'v', i: 'root', sm: { 'flex-direction': 'column' }, c: nodes })
    host.render(tree)
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
      scheduleRender()
    },
    // 立刻重新渲染（页面切回这个插件时用）
    render,
    // 点击、滑动处理完马上渲染，不等下一轮计时器（手环上计时器要等几十毫秒）
    dispatch(id, type) {
      const hd = handlers[id]
      const fn = hd && hd[type]
      if (!fn) return
      try {
        fn()
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
