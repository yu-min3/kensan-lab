import * as React from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useCompositionGuard } from '@/hooks/useCompositionGuard'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'

interface InputPopoverProps {
  children: React.ReactNode
  label: string
  placeholder?: string
  confirmLabel?: string
  cancelLabel?: string
  onConfirm: (value: string) => void
  disabled?: boolean
  defaultValue?: string
  inputType?: 'text' | 'url'
}

export function InputPopover({
  children,
  label,
  placeholder,
  confirmLabel = '確定',
  cancelLabel = 'キャンセル',
  onConfirm,
  disabled = false,
  defaultValue = '',
  inputType = 'text',
}: InputPopoverProps) {
  const [open, setOpen] = React.useState(false)
  const [value, setValue] = React.useState(defaultValue)
  const inputRef = React.useRef<HTMLInputElement>(null)

  React.useEffect(() => {
    if (open) {
      setValue(defaultValue)
      // フォーカスは少し遅延させて確実にポップオーバーが表示された後に
      setTimeout(() => {
        inputRef.current?.focus()
        inputRef.current?.select()
      }, 50)
    }
  }, [open, defaultValue])

  const handleConfirm = () => {
    if (value.trim()) {
      onConfirm(value.trim())
      setOpen(false)
    }
  }

  const { isComposingRef, onCompositionStart, onCompositionEnd } = useCompositionGuard()

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (isComposingRef.current) return
    if (e.key === 'Enter') {
      e.preventDefault()
      handleConfirm()
    }
    if (e.key === 'Escape') {
      setOpen(false)
    }
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild disabled={disabled}>
        {children}
      </PopoverTrigger>
      <PopoverContent
        className="w-80 p-3"
        align="start"
        side="bottom"
        sideOffset={8}
      >
        <div className="space-y-2">
          <p className="text-sm font-medium">{label}</p>
          <Input
            ref={inputRef}
            type={inputType}
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={handleKeyDown}
            onCompositionStart={onCompositionStart}
            onCompositionEnd={onCompositionEnd}
            placeholder={placeholder}
            className="h-9"
          />
        </div>
        <div className="flex justify-end gap-2 mt-3">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setOpen(false)}
          >
            {cancelLabel}
          </Button>
          <Button
            size="sm"
            onClick={handleConfirm}
            disabled={!value.trim()}
          >
            {confirmLabel}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}
