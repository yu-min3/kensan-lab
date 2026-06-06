import { Card } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import { Button } from '@/components/ui/button'
import { Bold, Italic, Code, Link, Image, List, Heading } from 'lucide-react'

interface MarkdownEditorPlaceholderProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
}

export function MarkdownEditorPlaceholder({
  value,
  onChange,
  placeholder = 'Markdownで記述...',
}: MarkdownEditorPlaceholderProps) {
  return (
    <Card className="overflow-hidden">
      {/* ツールバー */}
      <div className="flex items-center gap-1 p-2 border-b bg-muted/30">
        <Button variant="ghost" size="icon" className="h-8 w-8">
          <Heading className="h-4 w-4" />
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8">
          <Bold className="h-4 w-4" />
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8">
          <Italic className="h-4 w-4" />
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8">
          <Code className="h-4 w-4" />
        </Button>
        <div className="w-px h-4 bg-border mx-1" />
        <Button variant="ghost" size="icon" className="h-8 w-8">
          <List className="h-4 w-4" />
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8">
          <Link className="h-4 w-4" />
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8">
          <Image className="h-4 w-4" />
        </Button>
        <div className="flex-1" />
        <span className="text-xs text-muted-foreground">
          Milkdown (プレースホルダー)
        </span>
      </div>

      {/* エディタエリア */}
      <Textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="min-h-[300px] border-0 rounded-none focus-visible:ring-0 resize-none font-mono text-sm"
      />
    </Card>
  )
}
