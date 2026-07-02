import { useEffect } from 'react'
import { useSettingsStore } from '@/stores/useSettingsStore'
import type { Theme } from '@/types'

/**
 * テーマを管理するカスタムフック
 * - ストアのテーマ設定をDOMに反映
 * - systemの場合はOSの設定を監視
 */
export function useTheme() {
  const { theme, setTheme } = useSettingsStore()

  // テーマをDOMに適用
  useEffect(() => {
    const applyTheme = (resolvedTheme: 'light' | 'dark') => {
      if (resolvedTheme === 'dark') {
        document.documentElement.classList.add('dark')
      } else {
        document.documentElement.classList.remove('dark')
      }
    }

    if (theme === 'system') {
      // システム設定を確認
      const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
      applyTheme(mediaQuery.matches ? 'dark' : 'light')

      // システム設定の変更を監視
      const handleChange = (e: MediaQueryListEvent) => {
        applyTheme(e.matches ? 'dark' : 'light')
      }
      mediaQuery.addEventListener('change', handleChange)

      return () => {
        mediaQuery.removeEventListener('change', handleChange)
      }
    } else {
      applyTheme(theme)
    }
  }, [theme])

  // 現在の実効テーマを取得
  const resolvedTheme = (): 'light' | 'dark' => {
    if (theme === 'system') {
      return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
    }
    return theme
  }

  // テーマをトグル（light <-> dark、systemの場合は現在の実効値の逆）
  const toggleTheme = () => {
    const current = resolvedTheme()
    const newTheme: Theme = current === 'light' ? 'dark' : 'light'
    setTheme(newTheme)
  }

  // テーマを循環（light -> dark -> system -> light）
  const cycleTheme = () => {
    const order: Theme[] = ['light', 'dark', 'system']
    const currentIndex = order.indexOf(theme)
    const nextIndex = (currentIndex + 1) % order.length
    setTheme(order[nextIndex])
  }

  return {
    theme,
    setTheme,
    resolvedTheme: resolvedTheme(),
    toggleTheme,
    cycleTheme,
  }
}
