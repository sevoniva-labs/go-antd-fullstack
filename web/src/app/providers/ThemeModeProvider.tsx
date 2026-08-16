import { ConfigProvider, theme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { runtimeConfig, type ThemeMode } from '../config/runtime'
import { createForgeTheme } from '../../theme'

interface ThemeModeState {
  mode: ThemeMode
  compact: boolean
  toggleMode: () => void
  setCompact: (value: boolean) => void
}

const ThemeModeContext = createContext<ThemeModeState | null>(null)

function initialMode(): ThemeMode {
  const stored = localStorage.getItem('forge.theme')
  return stored === 'dark' || stored === 'light' ? stored : runtimeConfig.defaultTheme
}

export function ThemeModeProvider({ children }: { children: ReactNode }) {
  const [mode, setMode] = useState<ThemeMode>(initialMode)
  const [compact, setCompactState] = useState(() => {
    const stored = localStorage.getItem('forge.compact')
    return stored == null ? runtimeConfig.compactMode : stored === 'true'
  })

  useEffect(() => {
    document.documentElement.style.setProperty('--forge-primary', runtimeConfig.primaryColor)
    document.documentElement.dataset.theme = mode
  }, [mode])

  const value = useMemo<ThemeModeState>(() => ({
    mode,
    compact,
    toggleMode: () => setMode((current) => {
      const next = current === 'light' ? 'dark' : 'light'
      localStorage.setItem('forge.theme', next)
      return next
    }),
    setCompact: (next) => {
      localStorage.setItem('forge.compact', String(next))
      setCompactState(next)
    },
  }), [mode, compact])

  const algorithms = [mode === 'dark' ? theme.darkAlgorithm : theme.defaultAlgorithm]
  if (compact) algorithms.push(theme.compactAlgorithm)

  return (
    <ThemeModeContext.Provider value={value}>
      <ConfigProvider locale={zhCN} theme={{ ...createForgeTheme(mode), algorithm: algorithms }}>
        {children}
      </ConfigProvider>
    </ThemeModeContext.Provider>
  )
}

export function useThemeMode() {
  const value = useContext(ThemeModeContext)
  if (!value) throw new Error('useThemeMode must be used inside ThemeModeProvider')
  return value
}
