import { useState, useRef, useCallback, useEffect } from 'react'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Maximize2, X, Save, GitFork } from 'lucide-react'

/* eslint-disable @typescript-eslint/no-explicit-any */

interface MindmapEditorProps {
  value: string
  onChange: (value: string) => void
}

const DEFAULT_DATA = JSON.stringify({
  data: {
    data: { text: 'メインテーマ' },
    children: [],
  },
})

const FONT_FAMILY = '-apple-system, BlinkMacSystemFont, "Segoe UI", "Hiragino Sans", "Noto Sans JP", sans-serif'

const MINDMAP_OPTIONS = {
  theme: 'default',
  themeConfig: {
    root: { fontFamily: FONT_FAMILY },
    second: { fontFamily: FONT_FAMILY },
    node: { fontFamily: FONT_FAMILY },
  },
  layout: 'logicalStructure',
  mouseScaleCenterUseMousePosition: true,
  enableFreeDrag: false,
  isShowExpandNum: true,
  defaultInsertSecondLevelNodeText: 'トピック',
  defaultInsertBelowSecondLevelNodeText: 'サブトピック',
  defaultGeneralizationText: '概要',
  defaultAssociativeLineText: '関連',
}

function parseData(data: string) {
  try {
    const parsed = JSON.parse(data)
    return parsed.data || parsed
  } catch {
    return JSON.parse(DEFAULT_DATA).data
  }
}

export function MindmapEditor({ value, onChange }: MindmapEditorProps) {
  const [isFullscreen, setIsFullscreen] = useState(false)
  const fullscreenContainerRef = useRef<HTMLDivElement>(null)
  const previewContainerRef = useRef<HTMLDivElement>(null)
  const fullscreenMapRef = useRef<any>(null)
  const previewMapRef = useRef<any>(null)
  const isInitializingRef = useRef(false)
  const isPreviewInitRef = useRef(false)
  // Guard: prevent data_change during destroy from wiping data
  const isDestroyingRef = useRef(false)
  // Keep latest onChange ref to avoid stale closures in event listeners
  const onChangeRef = useRef(onChange)
  onChangeRef.current = onChange

  const initData = value || DEFAULT_DATA

  const loadPlugins = async () => {
    const MindMap = (await import('simple-mind-map')).default
    // @ts-expect-error -- plugin modules have no type declarations
    const Drag = (await import('simple-mind-map/src/plugins/Drag.js')).default
    // @ts-expect-error -- plugin modules have no type declarations
    const Select = (await import('simple-mind-map/src/plugins/Select.js')).default
    // @ts-expect-error -- plugin modules have no type declarations
    const KeyboardNavigation = (await import('simple-mind-map/src/plugins/KeyboardNavigation.js')).default

    MindMap.usePlugin(Drag)
    MindMap.usePlugin(Select)
    MindMap.usePlugin(KeyboardNavigation)
    return MindMap
  }

  // ---- Preview (read-only miniature) ----
  const initPreview = useCallback(async () => {
    if (isPreviewInitRef.current || !previewContainerRef.current) return
    isPreviewInitRef.current = true
    try {
      const MindMap = await loadPlugins()
      const instance = new MindMap({
        ...MINDMAP_OPTIONS,
        el: previewContainerRef.current,
        data: parseData(initData),
        readonly: true,
        enableShortcutOnlyWhenMouseInSvg: true,
      } as any)
      // Fit content to preview area
      setTimeout(() => {
        try { (instance as any).view?.fit?.() } catch { /* ignore */ }
      }, 100)
      previewMapRef.current = instance
    } catch (err) {
      console.error('Failed to initialize preview MindMap:', err)
    } finally {
      isPreviewInitRef.current = false
    }
  }, [initData])

  const destroyPreview = useCallback(() => {
    if (previewMapRef.current) {
      try { previewMapRef.current.destroy() } catch { /* ignore */ }
      previewMapRef.current = null
    }
    isPreviewInitRef.current = false
  }, [])

  // Init/destroy preview when not fullscreen and we have data
  useEffect(() => {
    if (!isFullscreen && value) {
      initPreview()
    }
    return () => {
      destroyPreview()
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isFullscreen, value])

  // ---- Fullscreen (editable) ----
  const initFullscreen = useCallback(async (container: HTMLElement) => {
    if (isInitializingRef.current) return
    isInitializingRef.current = true
    isDestroyingRef.current = false
    try {
      const MindMap = await loadPlugins()
      const instance = new MindMap({
        ...MINDMAP_OPTIONS,
        el: container,
        data: parseData(initData),
      } as any)

      instance.on('data_change', (rootNode: any) => {
        // Skip onChange during destroy to prevent data wipe
        if (isDestroyingRef.current) return
        if (!rootNode) return
        try {
          onChangeRef.current(JSON.stringify({ data: rootNode }))
        } catch { /* ignore */ }
      })

      fullscreenMapRef.current = instance
    } catch (err) {
      console.error('Failed to initialize MindMap:', err)
    } finally {
      isInitializingRef.current = false
    }
  }, [initData])

  const destroyFullscreen = useCallback(() => {
    // Set flag BEFORE destroy to block any data_change events
    isDestroyingRef.current = true
    if (fullscreenMapRef.current) {
      try { fullscreenMapRef.current.destroy() } catch { /* ignore */ }
      fullscreenMapRef.current = null
    }
  }, [])

  useEffect(() => {
    if (isFullscreen && fullscreenContainerRef.current) {
      initFullscreen(fullscreenContainerRef.current)
    }
    return () => {
      if (isFullscreen) destroyFullscreen()
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isFullscreen])

  const handleSaveAndClose = useCallback(() => {
    setIsFullscreen(false)
  }, [])

  // ESC key to close
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isFullscreen) {
        handleSaveAndClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isFullscreen, handleSaveAndClose])

  return (
    <>
      {/* Preview card */}
      <Card className="overflow-hidden">
        <div className="flex items-center justify-between p-2 border-b bg-muted/30">
          <span className="text-xs text-muted-foreground">マインドマップ</span>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setIsFullscreen(true)}
            className="gap-1"
          >
            <Maximize2 className="h-4 w-4" />
            全画面で編集
          </Button>
        </div>
        <div
          className="h-[200px] bg-white dark:bg-slate-900 cursor-pointer hover:bg-muted/50 transition-colors relative"
          onClick={() => setIsFullscreen(true)}
        >
          {value ? (
            <div
              ref={previewContainerRef}
              className="w-full h-full pointer-events-none"
            />
          ) : (
            <div className="w-full h-full flex items-center justify-center text-center text-muted-foreground">
              <div>
                <GitFork className="h-12 w-12 mx-auto mb-2 opacity-30" />
                <p className="text-sm">クリックしてマインドマップを作成</p>
              </div>
            </div>
          )}
        </div>
      </Card>

      {/* Fullscreen modal */}
      {isFullscreen && (
        <div className="fixed inset-0 z-50 bg-background">
          {/* Header */}
          <div className="absolute top-0 left-0 right-0 h-12 bg-muted/80 backdrop-blur border-b flex items-center justify-between px-4 z-10">
            <div className="flex items-center gap-4">
              <span className="font-medium">マインドマップエディタ</span>
              <div className="hidden sm:flex items-center gap-3 text-xs text-muted-foreground">
                <span><kbd className="px-1 py-0.5 rounded bg-muted border text-[10px]">Tab</kbd> 子ノード</span>
                <span><kbd className="px-1 py-0.5 rounded bg-muted border text-[10px]">Enter</kbd> 兄弟ノード</span>
                <span><kbd className="px-1 py-0.5 rounded bg-muted border text-[10px]">Del</kbd> 削除</span>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={handleSaveAndClose}
                className="gap-1"
              >
                <Save className="h-4 w-4" />
                保存して閉じる
              </Button>
              <Button
                variant="ghost"
                size="icon"
                onClick={handleSaveAndClose}
              >
                <X className="h-5 w-5" />
              </Button>
            </div>
          </div>

          {/* Mind map container */}
          <div
            ref={fullscreenContainerRef}
            className="absolute top-12 left-0 right-0 bottom-0"
          />
        </div>
      )}
    </>
  )
}
