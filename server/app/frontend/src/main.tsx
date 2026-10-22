import React from 'react'
import {createRoot} from 'react-dom/client'
import './index.css'
import App from './App'

// 跟着系统的深色 / 浅色（shadcn/ui 用 html 上的 dark 类切换）
const dark = window.matchMedia('(prefers-color-scheme: dark)')
const applyTheme = () => document.documentElement.classList.toggle('dark', dark.matches)
applyTheme()
dark.addEventListener('change', applyTheme)

createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
        <App/>
    </React.StrictMode>
)
