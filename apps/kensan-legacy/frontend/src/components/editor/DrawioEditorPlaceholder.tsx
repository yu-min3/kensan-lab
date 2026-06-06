import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Shapes, Square, Circle, ArrowRight, Type, Undo, Redo, ZoomIn, ZoomOut } from 'lucide-react'

export function DrawioEditorPlaceholder() {
  return (
    <Card className="overflow-hidden">
      {/* ツールバー */}
      <div className="flex items-center gap-1 p-2 border-b bg-muted/30">
        <Button variant="ghost" size="icon" className="h-8 w-8">
          <Square className="h-4 w-4" />
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8">
          <Circle className="h-4 w-4" />
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8">
          <ArrowRight className="h-4 w-4" />
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8">
          <Type className="h-4 w-4" />
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8">
          <Shapes className="h-4 w-4" />
        </Button>
        <div className="w-px h-4 bg-border mx-1" />
        <Button variant="ghost" size="icon" className="h-8 w-8">
          <Undo className="h-4 w-4" />
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8">
          <Redo className="h-4 w-4" />
        </Button>
        <div className="w-px h-4 bg-border mx-1" />
        <Button variant="ghost" size="icon" className="h-8 w-8">
          <ZoomOut className="h-4 w-4" />
        </Button>
        <span className="text-xs text-muted-foreground mx-1">100%</span>
        <Button variant="ghost" size="icon" className="h-8 w-8">
          <ZoomIn className="h-4 w-4" />
        </Button>
        <div className="flex-1" />
        <span className="text-xs text-muted-foreground">
          react-drawio (プレースホルダー)
        </span>
      </div>

      {/* キャンバスエリア */}
      <div className="h-[400px] bg-muted/50 flex items-center justify-center">
        <div className="text-center text-muted-foreground">
          <Shapes className="h-16 w-16 mx-auto mb-4 opacity-20" />
          <p className="text-sm">drawioエディタ（プレースホルダー）</p>
          <p className="text-xs mt-1">
            実際の実装ではreact-drawioを統合します
          </p>
        </div>
      </div>
    </Card>
  )
}
