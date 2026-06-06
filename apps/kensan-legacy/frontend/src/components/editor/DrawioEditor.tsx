import { useState, useRef, useCallback, useEffect } from 'react'
import { DrawIoEmbed, DrawIoEmbedRef, EventExport } from 'react-drawio'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Maximize2, X, Save, Shapes } from 'lucide-react'

interface DrawioEditorProps {
  value: string
  onChange: (value: string) => void
}

export function DrawioEditor({ value, onChange }: DrawioEditorProps) {
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [previewSvg, setPreviewSvg] = useState<string | null>(null)
  const drawioRef = useRef<DrawIoEmbedRef>(null)

  // XMLからSVGを抽出してプレビュー用に保存
  useEffect(() => {
    if (value) {
      // xmlsvg形式の場合、SVGデータが含まれている
      const svgMatch = value.match(/<svg[^>]*>[\s\S]*<\/svg>/)
      if (svgMatch) {
        setPreviewSvg(svgMatch[0])
      }
    }
  }, [value])

  const handleExport = useCallback((data: EventExport) => {
    if (data.xml) {
      onChange(data.xml)
      // SVGも保存
      if (data.xml.includes('<svg')) {
        const svgMatch = data.xml.match(/<svg[^>]*>[\s\S]*<\/svg>/)
        if (svgMatch) {
          setPreviewSvg(svgMatch[0])
        }
      }
    }
  }, [onChange])

  const handleSave = useCallback(() => {
    drawioRef.current?.exportDiagram({
      format: 'xmlsvg',
    })
  }, [])

  const handleSaveAndClose = useCallback(() => {
    handleSave()
    // 少し待ってから閉じる（エクスポート完了を待つ）
    setTimeout(() => {
      setIsFullscreen(false)
    }, 300)
  }, [handleSave])

  // ESCキーで閉じる
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
      {/* プレビューカード */}
      <Card className="overflow-hidden">
        <div className="flex items-center justify-between p-2 border-b bg-muted/30">
          <span className="text-xs text-muted-foreground">draw.io 図</span>
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
          className="h-[200px] bg-white dark:bg-slate-900 flex items-center justify-center cursor-pointer hover:bg-muted/50 transition-colors"
          onClick={() => setIsFullscreen(true)}
        >
          {previewSvg ? (
            <div
              className="w-full h-full p-4 flex items-center justify-center"
              dangerouslySetInnerHTML={{ __html: previewSvg }}
              style={{ maxHeight: '100%', overflow: 'hidden' }}
            />
          ) : value ? (
            <div className="w-full h-full flex items-center justify-center bg-primary/5 border-2 border-dashed border-primary/20 rounded-sm">
              <div className="text-center">
                <div className="inline-flex items-center justify-center w-14 h-14 rounded-full bg-primary/10 mb-2">
                  <Shapes className="h-7 w-7 text-primary" />
                </div>
                <p className="text-sm font-medium">図が保存されています</p>
                <p className="text-xs text-muted-foreground mt-1">クリックして編集・プレビュー</p>
              </div>
            </div>
          ) : (
            <div className="text-center text-muted-foreground">
              <Shapes className="h-12 w-12 mx-auto mb-2 opacity-30" />
              <p className="text-sm">クリックして図を作成</p>
            </div>
          )}
        </div>
      </Card>

      {/* フルスクリーンモーダル */}
      {isFullscreen && (
        <div className="fixed inset-0 z-50 bg-background">
          {/* ヘッダー */}
          <div className="absolute top-0 left-0 right-0 h-12 bg-muted/80 backdrop-blur border-b flex items-center justify-between px-4 z-10">
            <span className="font-medium">draw.io エディタ</span>
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

          {/* エディタ */}
          <div className="absolute top-12 left-0 right-0 bottom-0">
            <DrawIoEmbed
              ref={drawioRef}
              xml={value || undefined}
              onExport={handleExport}
              onSave={handleSave}
              urlParameters={{
                ui: 'kennedy',
                spin: true,
                libraries: true,
                saveAndExit: false,
                noSaveBtn: false,
                noExitBtn: true,
              }}
            />
          </div>
        </div>
      )}
    </>
  )
}
