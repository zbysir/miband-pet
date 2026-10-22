import {useEffect, useRef, useState} from 'react'
import type {Agent} from './types'

// 和手环上的 Agent 猫插件同一套帧（tools/gen_pet.py 生成到 server/plugins/agent-cat/assets，make app 复制到 public/pet）
export type Mood = 'alert' | 'celebrate' | 'work' | 'sleep'
export type Skin = 'orange' | 'gray' | 'white'

export const SKINS: { value: Skin, label: string }[] = [
    {value: 'orange', label: '橘猫'},
    {value: 'gray', label: '灰猫'},
    {value: 'white', label: '白猫'},
]

const FRAMES: Record<Mood, { count: number, duration: number }> = {
    work: {count: 6, duration: 900},
    alert: {count: 6, duration: 700},
    celebrate: {count: 6, duration: 800},
    sleep: {count: 8, duration: 2400},
}

const CELEBRATE_MS = 8000

// 和快应用一样的心情逻辑：有卡住的 → 举牌；刚有 agent 从 working 变成 done/idle → 庆祝 8 秒；有在跑的 → 干活；否则睡觉
export function useMood(agents: Agent[]): Mood {
    const prev = useRef<Record<string, string>>({})
    const [celebrateUntil, setCelebrateUntil] = useState(0)
    const [, tick] = useState(0)

    useEffect(() => {
        let finished = false
        for (const a of agents) {
            const before = prev.current[a.id]
            if (before === 'working' && (a.status === 'done' || a.status === 'idle')) finished = true
            prev.current[a.id] = a.status
        }
        if (finished) {
            setCelebrateUntil(Date.now() + CELEBRATE_MS)
            const t = setTimeout(() => tick(n => n + 1), CELEBRATE_MS + 50)
            return () => clearTimeout(t)
        }
    }, [agents])

    if (agents.some(a => a.status === 'blocked')) return 'alert'
    if (Date.now() < celebrateUntil) return 'celebrate'
    if (agents.some(a => a.status === 'working')) return 'work'
    return 'sleep'
}

export function Pet({mood, size, skin = 'orange'}: { mood: Mood, size: number, skin?: Skin }) {
    const [frame, setFrame] = useState(0)
    const {count, duration} = FRAMES[mood]

    useEffect(() => {
        setFrame(0)
        const t = setInterval(() => setFrame(f => (f + 1) % count), duration / count)
        return () => clearInterval(t)
    }, [mood, count, duration])

    return (
        <img
            className="pet"
            src={`/pet/${skin}/${mood}_${frame % count}.png`}
            width={size}
            height={size}
            alt=""
            draggable={false}
        />
    )
}
