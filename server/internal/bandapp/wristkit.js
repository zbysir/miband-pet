// wristkit 在手环上的运行时：编译时打进每个 app 的页面（页面里 import { call } from 'wristkit' 拿到下面导出的东西）。
//
// - 连 Mac：页面显示时发 hello，Mac 回这个 app 的全部数据；之后数据变了 Mac 再推。
//   有的手环（小米手环 9 Pro）刚连上时发的消息会丢，没等到回应就每秒重发，最多 10 次。
// - 数据到了调页面的 onStore(name, value)；配置（名字是 config）还会调 onConfig(config)。
// - 太大的数据 Mac 分几片发，这里拼回去。
// - 数据在手环上存一份，下次打开先用它画，不用等 Mac。
// - call(name, ...args) 交给 Mac 上本 app 的 server.js；console 的输出和页面方法里抛的异常发回 Mac 记日志。
import interconnect from '@system.interconnect'
import storage from '@system.storage'

const CACHE = 'wristkit:stores'
const S = { conn: null, vm: null, stores: {}, heard: false, seq: 0, errs: [], parts: {} }

function send(obj) {
  const fail = (d, code) => {
    if (S.errs.length >= 5) return
    let why = code === undefined ? d : code
    try {
      if (typeof why === 'object') why = JSON.stringify(why)
    } catch (e) {}
    S.errs.push(obj.t + ' ' + why)
  }
  try {
    S.conn.send({ data: obj, success: () => {}, fail })
  } catch (e) {
    fail(e && e.message ? e.message : e)
  }
}

function log(level, msg) {
  send({ t: 'log', level, msg: String(msg).slice(0, 800) })
}

function hello() {
  const seq = ++S.seq
  S.heard = false
  const say = (n) => {
    if (seq !== S.seq || S.heard) return
    send({ t: 'hello' })
    if (n < 10) setTimeout(() => say(n + 1), 1000)
  }
  say(1)
}

function report(where, e) {
  log('error', where + '：' + (e && e.stack ? e.stack : e))
}

function deliver(name, v) {
  S.stores[name] = v
  const vm = S.vm
  if (!vm) return
  try {
    if (name === 'config' && typeof vm.onConfig === 'function') vm.onConfig(v)
    if (typeof vm.onStore === 'function') vm.onStore(name, v)
  } catch (e) {
    report('onStore(' + name + ')', e)
  }
}

function connect() {
  if (S.conn) return
  S.conn = interconnect.instance()
  S.conn.onopen = () => hello()
  S.conn.onmessage = (msg) => {
    let m = msg && msg.data
    try {
      if (typeof m === 'string') m = JSON.parse(m)
    } catch (e) {
      return
    }
    if (!m) return
    if (!S.heard) {
      S.heard = true
      if (S.errs.length > 0) {
        log('warn', '发给 Mac 失败过：' + S.errs.join('，'))
        S.errs = []
      }
    }
    if (m.t === 'store') deliver(m.name, m.v)
    else if (m.t === 'part') {
      // 太大的数据分几片发来，收齐了拼回去
      const p = S.parts[m.id] || (S.parts[m.id] = { list: [], got: 0 })
      if (p.list[m.k] === undefined) p.got++
      p.list[m.k] = m.s
      if (p.got < m.n) return
      delete S.parts[m.id]
      try {
        deliver(m.name, JSON.parse(p.list.join('')))
      } catch (e) {
        report('数据 ' + m.name, e)
      }
    }
  }
}

let consoleHooked = false
function hookConsole() {
  if (consoleHooked || typeof console === 'undefined') return
  consoleHooked = true
  ;['log', 'info', 'warn', 'error'].forEach((level) => {
    const orig = console[level]
    console[level] = function (...args) {
      try {
        if (orig) orig.apply(console, args)
      } catch (e) {}
      log(level === 'info' ? 'log' : level, args.map((a) => (typeof a === 'string' ? a : JSON.stringify(a))).join(' '))
    }
  })
}

// 这个 app 现在的某项数据（server.js 推的，或 config）
export function store(name) {
  return S.stores[name]
}

// 这个 app 现在的配置
export function config() {
  return S.stores.config || {}
}

// 让 Mac 上本 app 的 server.js 做事（交给它 on(name, fn) 注册的函数）。不等结果：server.js 做完会推新数据
export function call(name, ...args) {
  send({ t: 'call', name, args })
}

// 包一层页面：编译时自动加上，页面不用自己调
export function page(def) {
  const out = {}
  Object.keys(def).forEach((k) => {
    const v = def[k]
    out[k] =
      typeof v === 'function'
        ? function (...args) {
            try {
              return v.apply(this, args)
            } catch (e) {
              report(k, e)
            }
          }
        : v
  })
  const own = (name) => out[name]
  const onInit = own('onInit')
  out.onInit = function () {
    hookConsole()
    connect()
    if (onInit) onInit.call(this)
    S.vm = this
    Object.keys(S.stores).forEach((name) => deliver(name, S.stores[name]))
    // 先用上次存的数据画，Mac 的数据到了再换
    storage.get({
      key: CACHE,
      success: (v) => {
        if (S.heard || S.vm !== this) return
        try {
          const c = JSON.parse(v) || {}
          Object.keys(c).forEach((name) => {
            if (S.stores[name] === undefined) deliver(name, c[name])
          })
        } catch (e) {}
      }
    })
  }
  const onShow = own('onShow')
  out.onShow = function () {
    if (onShow) onShow.call(this)
    hello()
  }
  const onHide = own('onHide')
  out.onHide = function () {
    storage.set({ key: CACHE, value: JSON.stringify(S.stores) })
    send({ t: 'bye' })
    if (onHide) onHide.call(this)
  }
  const onDestroy = own('onDestroy')
  out.onDestroy = function () {
    send({ t: 'bye' })
    if (onDestroy) onDestroy.call(this)
    if (S.vm === this) S.vm = null
  }
  return out
}
