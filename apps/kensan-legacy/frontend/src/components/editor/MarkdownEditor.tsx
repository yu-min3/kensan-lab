import { useEffect, useCallback } from 'react'
import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import Link from '@tiptap/extension-link'
import Image from '@tiptap/extension-image'
import { Markdown } from 'tiptap-markdown'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { InputPopover } from '@/components/common/InputPopover'
import {
  Bold,
  Italic,
  Strikethrough,
  Code,
  Heading1,
  Heading2,
  Heading3,
  List,
  ListOrdered,
  Quote,
  Minus,
  Undo,
  Redo,
  Link as LinkIcon,
  Code2,
  ChevronRight,
} from 'lucide-react'
import { Details, DetailsSummary } from './extensions/details'

interface MarkdownEditorProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  onImageUpload?: (file: File) => Promise<string>
}

function ToolbarButton({
  onClick,
  icon: Icon,
  title,
  isActive = false,
  disabled = false,
}: {
  onClick: () => void
  icon: React.ComponentType<{ className?: string }>
  title: string
  isActive?: boolean
  disabled?: boolean
}) {
  return (
    <Button
      variant="ghost"
      size="icon"
      className={`h-8 w-8 ${isActive ? 'bg-muted' : ''}`}
      onClick={onClick}
      title={title}
      type="button"
      disabled={disabled}
    >
      <Icon className="h-4 w-4" />
    </Button>
  )
}

function readFileAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = reject
    reader.readAsDataURL(file)
  })
}

export function MarkdownEditor({ value, onChange, placeholder, onImageUpload }: MarkdownEditorProps) {
  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        heading: {
          levels: [1, 2, 3],
        },
      }),
      Placeholder.configure({
        placeholder: placeholder || 'Markdownで記述...',
      }),
      Link.configure({
        openOnClick: false,
      }),
      Image.configure({
        inline: false,
        allowBase64: true,
      }),
      Markdown.configure({
        html: true,
      }),
      Details,
      DetailsSummary,
    ],
    content: value,
    onUpdate: ({ editor }) => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const storage = editor.storage as any
      const markdown = storage.markdown.getMarkdown()
      onChange(markdown)
    },
  })

  // handlePaste needs editor reference, so define after useEditor
  const handlePaste = useCallback(
    async (event: ClipboardEvent) => {
      if (!onImageUpload || !editor) return false

      const items = event.clipboardData?.items
      if (!items) return false

      for (const item of items) {
        if (item.type.startsWith('image/')) {
          const file = item.getAsFile()
          if (!file) continue

          event.preventDefault()

          // Show image immediately using data URL
          const dataUrl = await readFileAsDataURL(file)
          editor.chain().focus().setImage({ src: dataUrl, alt: 'Uploading...' }).run()

          // Upload in background, then replace data URL with real URL
          try {
            const url = await onImageUpload(file)
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            const storage = editor.storage as any
            const currentMarkdown = storage.markdown.getMarkdown() as string
            const updatedMarkdown = currentMarkdown.replace(dataUrl, url)
            editor.commands.setContent(updatedMarkdown)
            onChange(updatedMarkdown)
          } catch (error) {
            console.error('Image upload failed:', error)
            // Keep the data URL preview so user doesn't lose the image
          }

          return true
        }
      }
      return false
    },
    [onImageUpload, onChange, editor]
  )

  // Register paste handler via DOM event since editorProps can't be updated after init
  useEffect(() => {
    if (!editor) return
    const element = editor.view.dom
    const handler = (event: Event) => {
      handlePaste(event as ClipboardEvent)
    }
    element.addEventListener('paste', handler)
    return () => element.removeEventListener('paste', handler)
  }, [editor, handlePaste])

  // 外部からvalueが変更された場合に同期
  useEffect(() => {
    if (!editor) return
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const storage = editor.storage as any
    if (value !== storage.markdown.getMarkdown()) {
      editor.commands.setContent(value)
    }
  }, [value, editor])

  if (!editor) {
    return null
  }

  const addLink = (url: string) => {
    editor.chain().focus().setLink({ href: url }).run()
  }

  return (
    <Card className="overflow-hidden">
      <div className="flex flex-wrap items-center gap-1 p-2 border-b bg-muted/30">
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleHeading({ level: 1 }).run()}
          icon={Heading1}
          title="見出し1"
          isActive={editor.isActive('heading', { level: 1 })}
        />
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}
          icon={Heading2}
          title="見出し2"
          isActive={editor.isActive('heading', { level: 2 })}
        />
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()}
          icon={Heading3}
          title="見出し3"
          isActive={editor.isActive('heading', { level: 3 })}
        />
        <div className="w-px h-4 bg-border mx-1" />
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleBold().run()}
          icon={Bold}
          title="太字"
          isActive={editor.isActive('bold')}
        />
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleItalic().run()}
          icon={Italic}
          title="斜体"
          isActive={editor.isActive('italic')}
        />
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleStrike().run()}
          icon={Strikethrough}
          title="取り消し線"
          isActive={editor.isActive('strike')}
        />
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleCode().run()}
          icon={Code}
          title="インラインコード"
          isActive={editor.isActive('code')}
        />
        <div className="w-px h-4 bg-border mx-1" />
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleBulletList().run()}
          icon={List}
          title="箇条書き"
          isActive={editor.isActive('bulletList')}
        />
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleOrderedList().run()}
          icon={ListOrdered}
          title="番号付きリスト"
          isActive={editor.isActive('orderedList')}
        />
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleBlockquote().run()}
          icon={Quote}
          title="引用"
          isActive={editor.isActive('blockquote')}
        />
        <ToolbarButton
          onClick={() => editor.chain().focus().toggleCodeBlock().run()}
          icon={Code2}
          title="コードブロック"
          isActive={editor.isActive('codeBlock')}
        />
        <ToolbarButton
          onClick={() => editor.chain().focus().setHorizontalRule().run()}
          icon={Minus}
          title="水平線"
        />
        <ToolbarButton
          onClick={() => editor.chain().focus().insertDetails().run()}
          icon={ChevronRight}
          title="トグル"
        />
        <div className="w-px h-4 bg-border mx-1" />
        <InputPopover
          label="URLを入力"
          placeholder="https://example.com"
          confirmLabel="挿入"
          onConfirm={addLink}
          inputType="url"
        >
          <ToolbarButton
            onClick={() => {}}
            icon={LinkIcon}
            title="リンク"
            isActive={editor.isActive('link')}
          />
        </InputPopover>
        <div className="w-px h-4 bg-border mx-1" />
        <ToolbarButton
          onClick={() => editor.chain().focus().undo().run()}
          icon={Undo}
          title="元に戻す"
          disabled={!editor.can().undo()}
        />
        <ToolbarButton
          onClick={() => editor.chain().focus().redo().run()}
          icon={Redo}
          title="やり直す"
          disabled={!editor.can().redo()}
        />
        <div className="flex-1" />
        <span className="text-xs text-muted-foreground">Markdown</span>
      </div>
      <EditorContent editor={editor} className="tiptap-editor min-h-[300px] p-4" />
    </Card>
  )
}
