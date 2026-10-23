import {useEffect, useState} from 'react'

// App 自己的猫（连接向导、「菜单栏的猫」；菜单栏图标也从这里裁）：public/pet，和手环上 Agent 猫插件的帧分开放。
// 两套都由 tools/gen_pet.py 画出来，App 这套只有用到的几种动作
export type Mood = 'celebrate' | 'work' | 'sleep'
export type Skin = 'orange' | 'gray' | 'white'

export const SKINS: { value: Skin, label: string }[] = [
    {value: 'orange', label: '橘猫'},
    {value: 'gray', label: '灰猫'},
    {value: 'white', label: '白猫'},
]

const FRAMES: Record<Mood, { count: number, duration: number }> = {
    work: {count: 6, duration: 900},
    celebrate: {count: 6, duration: 800},
    sleep: {count: 8, duration: 2400},
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
            className="pixel"
            src={`/pet/${skin}/${mood}_${frame % count}.png`}
            width={size}
            height={size}
            alt=""
            draggable={false}
        />
    )
}
