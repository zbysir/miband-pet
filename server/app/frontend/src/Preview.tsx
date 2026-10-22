import {useEffect, useRef, useState} from 'react'
import type {CSSProperties, MouseEvent, PointerEvent} from 'react'
import {Sun, Watch} from 'lucide-react'
import {Button} from '@/components/ui/button'
import {Tabs, TabsList, TabsTrigger} from '@/components/ui/tabs'
import {Launch, PluginBundle, PluginCall, PluginStores, ShowPlugin} from '../wailsjs/go/main/App'
import {EventsOn} from '../wailsjs/runtime/runtime'
import type {AppState, PluginInfo} from './types'

// 在 Mac 窗口里跑插件：和手环上同一份运行时（render-app/src/common/runtime.js，make app 时复制到 public/runtime.js）、
// 同一份编译好的插件代码、同一份数据，节点树画成 HTML。点击、长按、上下滑都能在这里调试，call() 也会交给 Mac
// 上真正的处理函数（mac.js 或 App 的功能）。

// 节点树：格式见 render-app/tools/gen_template.py
interface Node {
    t: 'v' | 'x' | 'm' | 'a' | 'w'
    i: string
    s: string
    c?: Node[]
    x?: string
    u?: string
    f?: { src: string }[]
    d?: string
    e?: number
    vt?: boolean // pager 是不是纵向
    ix?: number // pager 当前第几页
}

interface Runtime {
    setStore(name: string, value: unknown): void
    mount(component: unknown): void
    dispatch(id: string, type: string, arg?: unknown): void
    swipe(dir: string): void
    render(): void
    api: Record<string, any>
}

const SCREEN = {width: 212, height: 303}
const ZOOM = 1.2

let runtimeSource: Promise<string> | null = null

function loadRuntime(): Promise<(host: object) => Runtime> {
    runtimeSource ??= fetch('/runtime.js').then(r => {
        if (!r.ok) throw new Error('找不到 runtime.js（make app 时从 render-app 复制过来）')
        return r.text()
    })
    return runtimeSource.then(src => {
        const mod: { exports: any } = {exports: {}}
        new Function('module', src)(mod)
        return mod.exports
    })
}

// "a:b;c-d:e" → React 的 style
function cssOf(s: string): Record<string, string> {
    const out: Record<string, string> = {}
    for (const part of s.split(';')) {
        const i = part.indexOf(':')
        if (i <= 0) continue
        const k = part.slice(0, i).trim().replace(/-([a-z])/g, (_, c) => c.toUpperCase())
        out[k] = part.slice(i + 1).trim()
    }
    return out
}

// 手环快应用的布局默认值：只有 flex，div 默认横排，边框写了宽度和颜色就是实线
const VIEW_BASE: CSSProperties = {
    display: 'flex', flexDirection: 'row', boxSizing: 'border-box', position: 'relative',
    minWidth: 0, minHeight: 0, borderStyle: 'solid', borderWidth: 0, flexShrink: 1,
}

function textStyle(css: Record<string, string>): CSSProperties {
    const st: CSSProperties = {display: 'block', boxSizing: 'border-box', borderStyle: 'solid', borderWidth: 0, flexShrink: 1, minWidth: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-all', ...css}
    const lines = Number(css.lines)
    delete (st as any).lines
    if (lines === 1) Object.assign(st, {whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: css.textOverflow || 'clip'})
    else if (lines > 1) Object.assign(st, {display: '-webkit-box', WebkitLineClamp: lines, WebkitBoxOrient: 'vertical', overflow: 'hidden'})
    return st
}

function Sprite({node}: { node: Node }) {
    const frames = node.f ?? []
    const [k, setK] = useState(0)
    const ms = parseInt(node.d ?? '1000') || 1000
    useEffect(() => {
        setK(0)
        if (frames.length < 2) return
        const t = setInterval(() => setK(n => (n + 1) % frames.length), ms / frames.length)
        return () => clearInterval(t)
    }, [node.i, frames.length, ms])
    const src = frames[k % Math.max(1, frames.length)]?.src
    return <img src={src} alt="" className="pixel" draggable={false} style={{...cssOf(node.s), objectFit: 'fill'} as CSSProperties}/>
}

// Pager：手环上是原生 swiper。这里自己处理：按住拖动时页面跟着鼠标走，松手按拖动距离吸附到上一页或下一页。
// 滚轮、触控板滚动不管，留给窗口滚动。插件改了当前页（ix）就动画翻过去。拖动过的那一下不算点击
function PagerView({node, rt}: { node: Node, rt: Runtime }) {
    const vertical = node.vt !== false
    const pages = node.c ?? []
    const ix = Math.min(node.ix ?? 0, Math.max(0, pages.length - 1))
    const size = vertical ? SCREEN.height : SCREEN.width
    const [drag, setDrag] = useState<number | null>(null) // 拖动的距离（设计稿的 px）
    const start = useRef<{ at: number, moved: boolean } | null>(null)

    const goTo = (i: number) => {
        const to = Math.max(0, Math.min(pages.length - 1, i))
        if (to !== ix && (node.e ?? 0) & 4) rt.dispatch(node.i, 'change', to)
    }
    const at = (ev: { clientX: number, clientY: number }) => (vertical ? ev.clientY : ev.clientX) / ZOOM
    const onPointerDown = (ev: PointerEvent<HTMLDivElement>) => {
        start.current = {at: at(ev), moved: false}
    }
    const onPointerMove = (ev: PointerEvent<HTMLDivElement>) => {
        if (!start.current) return
        const d = at(ev) - start.current.at
        // 真的拖起来了才抓住鼠标（拖出预览也继续跟手）；按下就抓的话，点击也会被抓走，里面的元素点不动
        if (!start.current.moved && Math.abs(d) > 4) {
            start.current.moved = true
            ev.currentTarget.setPointerCapture(ev.pointerId)
        }
        if (start.current.moved) setDrag(d)
    }
    const onPointerUp = () => {
        const d = drag ?? 0
        setDrag(null)
        if (Math.abs(d) > size * 0.2) goTo(ix + (d < 0 ? 1 : -1))
        // 拖动过的这一下不要再触发松手位置上的点击（见 onClickCapture）
        setTimeout(() => {
            start.current = null
        }, 0)
    }
    const onClickCapture = (ev: MouseEvent) => {
        if (start.current?.moved) ev.stopPropagation()
    }
    const offset = -ix * size + (drag ?? 0)
    return (
        <div onPointerDown={onPointerDown} onPointerMove={onPointerMove} onPointerUp={onPointerUp} onPointerCancel={onPointerUp}
             onClickCapture={onClickCapture}
             style={{...VIEW_BASE, ...cssOf(node.s), overflow: 'hidden', touchAction: 'none', cursor: drag == null ? 'grab' : 'grabbing'} as CSSProperties}>
            {/* 装页面的这一层不能被拉成一屏高、页面也不能被压缩，不然几页会挤在一屏里叠在一起 */}
            <div style={{
                display: 'flex', flexDirection: vertical ? 'column' : 'row', flexShrink: 0, alignSelf: 'flex-start',
                transform: vertical ? `translateY(${offset}px)` : `translateX(${offset}px)`,
                transition: drag == null ? 'transform .28s ease-out' : 'none',
            }}>
                {pages.map(c => (
                    <div key={c.i} style={{display: 'flex', flexShrink: 0, width: SCREEN.width, height: SCREEN.height}}>
                        <NodeView node={c} rt={rt}/>
                    </div>
                ))}
            </div>
        </div>
    )
}

function NodeView({node, rt}: { node: Node, rt: Runtime }) {
    const e = node.e ?? 0
    const events = {
        onClick: e & 1 ? (ev: MouseEvent) => {
            ev.stopPropagation() // 手环上里外两层都能点时只算最里面的
            rt.dispatch(node.i, 'click')
        } : undefined,
        onContextMenu: e & 2 ? (ev: MouseEvent) => {
            ev.preventDefault()
            ev.stopPropagation()
            rt.dispatch(node.i, 'longpress')
        } : undefined,
        style: e ? {cursor: 'pointer'} : undefined,
    }
    const css = cssOf(node.s)
    switch (node.t) {
        case 'x':
            return <span onClick={events.onClick} onContextMenu={events.onContextMenu} style={{...textStyle(css), ...events.style}}>{node.x}</span>
        case 'm':
            return <img src={node.u} alt="" className="pixel" draggable={false} onClick={events.onClick} onContextMenu={events.onContextMenu}
                        style={{objectFit: 'cover', ...css, ...events.style} as CSSProperties}/>
        case 'a':
            return <Sprite node={node}/>
        case 'w':
            return <PagerView node={node} rt={rt}/>
        default:
            return (
                <div onClick={events.onClick} onContextMenu={events.onContextMenu} style={{...VIEW_BASE, ...css, ...events.style} as CSSProperties}>
                    {(node.c ?? []).map(c => <NodeView key={c.i} node={c} rt={rt}/>)}
                </div>
            )
    }
}

function usePlugin(plugin: PluginInfo | undefined) {
    const [tree, setTree] = useState<Node | null>(null)
    const [error, setError] = useState('')
    const [rt, setRt] = useState<Runtime | null>(null)
    const [flash, setFlash] = useState(false)

    useEffect(() => {
        setTree(null)
        setError('')
        setRt(null)
        if (!plugin) return
        if (plugin.error && !plugin.code) {
            setError(plugin.error)
            return
        }
        let alive = true
        const name = plugin.name
        Promise.all([loadRuntime(), PluginBundle(name), PluginStores()]).then(([createRuntime, bundle, storesJSON]) => {
            if (!alive) return
            const key = (k: string) => `preview:${name}:${k}`
            const runtime = createRuntime({
                scope: name + ':',
                render: (t: Node) => alive && setTree(t),
                error: (msg: string) => alive && setError(msg),
                log: (level: string, msg: string) => console.log(`[${name}] ${level}`, msg),
                asset: (path: string) => {
                    const url = bundle!.assets[String(path).replace(/^\.\//, '')]
                    if (!url) throw new Error('插件里没有这个图片：' + path)
                    return url
                },
                vibrate: () => {
                    setFlash(true)
                    setTimeout(() => setFlash(false), 300)
                },
                storageGet: (k: string) => {
                    try {
                        const v = localStorage.getItem(key(k))
                        return v == null ? undefined : JSON.parse(v)
                    } catch {
                        return undefined
                    }
                },
                storageSet: (k: string, v: unknown) => {
                    try {
                        localStorage.setItem(key(k), JSON.stringify(v))
                    } catch { /* 预览里存不了就算了 */ }
                },
                call: (n: string, args: unknown[]) => {
                    PluginCall(name, n, JSON.stringify(args)).catch(err => setError(`call('${n}') 失败：${err}`))
                },
            }) as Runtime
            const stores = JSON.parse(storesJSON) as Record<string, unknown>
            for (const [k, v] of Object.entries(stores)) runtime.setStore(k, v)
            runtime.setStore('screen', SCREEN)
            try {
                const mod: { exports: any } = {exports: {}}
                const require = (n: string) => {
                    if (n === 'miband') return runtime.api
                    throw new Error('只能 require("miband")，不支持 ' + n)
                }
                new Function('module', 'exports', 'require', 'console', '__h', '__Fragment', bundle!.code)(mod, mod.exports, require, runtime.api.console, runtime.api.h, runtime.api.Fragment)
                if (typeof mod.exports.default !== 'function') throw new Error('插件要 export default 一个组件函数')
                runtime.mount(mod.exports.default)
                setRt(runtime)
            } catch (err) {
                setError('加载出错：' + (err instanceof Error ? err.message : String(err)))
            }
        }).catch(err => alive && setError(String(err)))
        return () => {
            alive = false
        }
    }, [plugin?.name, plugin?.code])

    // Mac 上的数据变了，推给预览里的插件
    useEffect(() => {
        if (!rt) return
        let last: Record<string, string> = {}
        const sync = () => PluginStores().then(json => {
            const stores = JSON.parse(json) as Record<string, unknown>
            for (const [k, v] of Object.entries(stores)) {
                const s = JSON.stringify(v)
                if (last[k] !== s) rt.setStore(k, v)
                last[k] = s
            }
        })
        sync()
        return EventsOn('stores', sync)
    }, [rt])

    return {tree, error, rt, flash}
}

// 左边的手环预览。选哪个插件由 App 管（右边的设置跟着选中的插件走）
export function Preview({state, run, plugin, onSelect}: {
    state: AppState, run: (label: string, fn: () => Promise<unknown>) => void,
    plugin: PluginInfo | undefined, onSelect: (name: string) => void,
}) {
    const plugins = state.plugins ?? []
    const {tree, error, rt, flash} = usePlugin(plugin)

    // 最外层不是 Pager 的界面（比如 Agents 列表用 useSwipe 翻页）：在屏幕上拖一下当成一次滑动。
    // Pager 自己处理拖动（跟手翻页），这里不管
    const gesture = useRef<{ x: number, y: number, moved: boolean } | null>(null)
    const onPointerDown = (ev: PointerEvent<HTMLDivElement>) => {
        if (tree?.t === 'w') return
        gesture.current = {x: ev.clientX, y: ev.clientY, moved: false}
    }
    const onPointerUp = (ev: PointerEvent<HTMLDivElement>) => {
        const g = gesture.current
        if (!g || !rt) return
        const dx = (ev.clientX - g.x) / ZOOM
        const dy = (ev.clientY - g.y) / ZOOM
        if (Math.abs(dy) > 30 && Math.abs(dy) > Math.abs(dx)) {
            g.moved = true
            rt.swipe(dy < 0 ? 'up' : 'down') // 手指往上拖 = 上滑
        } else if (dx < -30) {
            g.moved = true
            rt.swipe('left')
        }
        setTimeout(() => {
            gesture.current = null
        }, 0)
    }
    // 拖过的那一下不算点击
    const onClickCapture = (ev: MouseEvent) => {
        if (gesture.current?.moved) ev.stopPropagation()
    }

    return (
        <div className="flex flex-col items-center gap-4">
            {plugins.length > 1 && (
                <Tabs value={plugin?.name} onValueChange={onSelect}>
                    <TabsList>
                        {plugins.map(p => <TabsTrigger key={p.name} value={p.name}>{p.title}</TabsTrigger>)}
                    </TabsList>
                </Tabs>
            )}
            {/* 画个手环：机身里是屏幕 */}
            <div className="band">
                <div className="band-body">
                    <div className={`preview-screen ${flash ? 'flash' : ''}`} style={{width: SCREEN.width * ZOOM, height: SCREEN.height * ZOOM}}
                         onDragStart={e => e.preventDefault()} onPointerDown={onPointerDown} onPointerUp={onPointerUp}
                         onClickCapture={onClickCapture}>
                        <div style={{width: SCREEN.width, height: SCREEN.height, zoom: ZOOM, overflow: 'hidden', background: '#000', color: '#fff'} as CSSProperties}>
                            {tree && rt && <NodeView node={tree} rt={rt}/>}
                        </div>
                    </div>
                </div>
            </div>
            {error && <p className="w-full whitespace-pre-wrap break-all text-xs text-destructive">{error}</p>}
            {plugin && (
                <div className="flex w-full gap-2">
                    <Button className="flex-1" disabled={!state.connected || !state.packages?.length || (plugin.active && !!state.foreground)}
                            onClick={() => run('切换', () => ShowPlugin(plugin.name))}>
                        <Watch/>{plugin.active && !!state.foreground ? '手环上正在显示' : '在手环上显示'}
                    </Button>
                    {/* 把手环上的 wristkit 打开到前台，手环就亮屏了 */}
                    <Button variant="outline" disabled={!state.connected || !state.packages?.length}
                            onClick={() => run('亮屏', () => Launch(state.packages![0]))}>
                        <Sun/>亮屏
                    </Button>
                </div>
            )}
            <p className="text-center text-xs text-muted-foreground">
                和手环上跑的是同一份代码和数据：可以点，按住拖动当滑动（翻页），右键当长按。
            </p>
        </div>
    )
}
