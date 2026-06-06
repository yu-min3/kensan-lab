import { useState, useRef, useEffect, KeyboardEvent } from 'react'
import { useCompositionGuard } from '@/hooks/useCompositionGuard'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { X, Tag as TagIcon, Pin, Plus, Pencil } from 'lucide-react'
import { TagDialog, type TagFormData } from '@/components/task/TagDialog'
import { useDialogState } from '@/hooks/useDialogState'
import type { Tag, TagCategory } from '@/types'

const CATEGORY_LABELS: Record<TagCategory, string> = {
  general: '',
  trait: '🏷️',
  tech: '💻',
  project: '📁',
}

interface TagInputProps {
  tags: Tag[]
  selectedTagIds: string[]
  onChange: (tagIds: string[]) => void
  onCreateTag?: (name: string, color: string) => Promise<Tag>
  onUpdateTag?: (id: string, data: { name: string; color: string }) => Promise<Tag>
  label?: string
  placeholder?: string
}

export function TagInput({
  tags,
  selectedTagIds,
  onChange,
  onCreateTag,
  onUpdateTag,
  label = 'タグ',
  placeholder = 'タグを検索または作成...',
}: TagInputProps) {
  const [inputValue, setInputValue] = useState('')
  const [isOpen, setIsOpen] = useState(false)
  const [highlightedIndex, setHighlightedIndex] = useState(-1)
  const inputRef = useRef<HTMLInputElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  const tagDialog = useDialogState<TagFormData>({ name: '', color: '#6B7280' })

  // Filter and sort tags
  const filteredTags = tags
    .filter((tag) => {
      // Exclude already selected tags
      if (selectedTagIds.includes(tag.id)) return false
      // Filter by input value
      if (inputValue.trim()) {
        return tag.name.toLowerCase().includes(inputValue.toLowerCase().trim())
      }
      return true
    })
    .sort((a, b) => {
      // Pinned first
      if (a.pinned !== b.pinned) return a.pinned ? -1 : 1
      // Then by usage count
      if (a.usageCount !== b.usageCount) return b.usageCount - a.usageCount
      // Then by name
      return a.name.localeCompare(b.name)
    })

  // Selected tags
  const selectedTags = selectedTagIds
    .map((id) => tags.find((t) => t.id === id))
    .filter((t): t is Tag => t !== undefined)

  // Check if input matches an existing tag exactly
  const exactMatch = tags.find(
    (t) => t.name.toLowerCase() === inputValue.toLowerCase().trim()
  )
  const canCreate = inputValue.trim() && !exactMatch && onCreateTag

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const handleSelect = (tagId: string) => {
    onChange([...selectedTagIds, tagId])
    setInputValue('')
    setIsOpen(false)
    setHighlightedIndex(-1)
  }

  const handleRemove = (tagId: string) => {
    onChange(selectedTagIds.filter((id) => id !== tagId))
  }

  const handleCreate = async () => {
    if (!canCreate) return
    try {
      const color = '#6B7280'
      const newTag = await onCreateTag(inputValue.trim(), color)
      onChange([...selectedTagIds, newTag.id])
      setInputValue('')
      setIsOpen(false)
    } catch (error) {
      console.error('Failed to create tag:', error)
    }
  }

  const handleEditTag = (e: React.MouseEvent, tag: Tag) => {
    e.stopPropagation()
    tagDialog.openEdit(tag.id, { name: tag.name, color: tag.color })
  }

  const handleTagDialogSave = async (data: TagFormData, editingId: string | null) => {
    if (!editingId) return
    if (onUpdateTag) {
      await onUpdateTag(editingId, { name: data.name, color: data.color })
    }
  }

  const { isComposingRef, onCompositionStart, onCompositionEnd } = useCompositionGuard()

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (isComposingRef.current) return
    const totalItems = filteredTags.length + (canCreate ? 1 : 0)

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        setHighlightedIndex((prev) => (prev < totalItems - 1 ? prev + 1 : 0))
        break
      case 'ArrowUp':
        e.preventDefault()
        setHighlightedIndex((prev) => (prev > 0 ? prev - 1 : totalItems - 1))
        break
      case 'Enter':
        e.preventDefault()
        if (highlightedIndex >= 0) {
          if (highlightedIndex < filteredTags.length) {
            handleSelect(filteredTags[highlightedIndex].id)
          } else if (canCreate) {
            handleCreate()
          }
        } else if (canCreate) {
          handleCreate()
        }
        break
      case 'Escape':
        setIsOpen(false)
        setHighlightedIndex(-1)
        break
      case 'Backspace':
        if (!inputValue && selectedTagIds.length > 0) {
          handleRemove(selectedTagIds[selectedTagIds.length - 1])
        }
        break
    }
  }

  // Pinned tags (not selected)
  const pinnedTags = tags.filter((t) => t.pinned && !selectedTagIds.includes(t.id))

  return (
    <div className="space-y-2" ref={containerRef}>
      <Label className="flex items-center gap-2">
        <TagIcon className="h-4 w-4" />
        {label}
      </Label>

      {/* Selected tags */}
      <div className="flex flex-wrap gap-2">
        {selectedTags.map((tag) => (
          <Badge
            key={tag.id}
            variant="default"
            className="cursor-pointer group"
            style={{ backgroundColor: tag.color, borderColor: tag.color }}
          >
            {tag.pinned && <Pin className="h-3 w-3 mr-1" />}
            {tag.name}
            {onUpdateTag && (
              <Pencil
                className="h-3 w-3 ml-1 opacity-0 group-hover:opacity-100 transition-opacity"
                onClick={(e) => handleEditTag(e, tag)}
              />
            )}
            <X
              className="h-3 w-3 ml-1"
              onClick={(e) => {
                e.stopPropagation()
                handleRemove(tag.id)
              }}
            />
          </Badge>
        ))}
      </div>

      {/* Pinned tags quick access */}
      {pinnedTags.length > 0 && (
        <div className="flex flex-wrap gap-1">
          <span className="text-xs text-muted-foreground flex items-center gap-1 mr-1">
            <Pin className="h-3 w-3" />
            ピン留め:
          </span>
          {pinnedTags.map((tag) => (
            <Badge
              key={tag.id}
              variant="outline"
              className="cursor-pointer text-xs"
              style={{ borderColor: tag.color, color: tag.color }}
              onClick={() => handleSelect(tag.id)}
            >
              {tag.name}
            </Badge>
          ))}
        </div>
      )}

      {/* Input with autocomplete */}
      <div className="relative">
        <Input
          ref={inputRef}
          value={inputValue}
          onChange={(e) => {
            setInputValue(e.target.value)
            setIsOpen(true)
            setHighlightedIndex(-1)
          }}
          onFocus={() => setIsOpen(true)}
          onKeyDown={handleKeyDown}
          onCompositionStart={onCompositionStart}
          onCompositionEnd={onCompositionEnd}
          placeholder={placeholder}
          className="w-full"
        />

        {/* Dropdown */}
        {isOpen && (filteredTags.length > 0 || canCreate) && (
          <div className="absolute z-50 w-full mt-1 bg-popover border rounded-md shadow-lg max-h-60 overflow-auto">
            {filteredTags.map((tag, index) => (
              <div
                key={tag.id}
                className={`flex items-center gap-2 px-3 py-2 cursor-pointer ${
                  index === highlightedIndex ? 'bg-accent' : 'hover:bg-accent'
                }`}
                onClick={() => handleSelect(tag.id)}
              >
                <div
                  className="w-3 h-3 rounded-full"
                  style={{ backgroundColor: tag.color }}
                />
                <span className="flex-1">
                  {tag.name}
                  {tag.category && tag.category !== 'general' && (
                    <span className="ml-1 text-xs">{CATEGORY_LABELS[tag.category]}</span>
                  )}
                </span>
                {tag.pinned && <Pin className="h-3 w-3 text-muted-foreground" />}
                <span className="text-xs text-muted-foreground">
                  {tag.usageCount}回使用
                </span>
                {onUpdateTag && (
                  <Pencil
                    className="h-3 w-3 text-muted-foreground hover:text-foreground"
                    onClick={(e) => handleEditTag(e, tag)}
                  />
                )}
              </div>
            ))}
            {canCreate && (
              <div
                className={`flex items-center gap-2 px-3 py-2 cursor-pointer border-t ${
                  highlightedIndex === filteredTags.length
                    ? 'bg-accent'
                    : 'hover:bg-accent'
                }`}
                onClick={handleCreate}
              >
                <Plus className="h-4 w-4 text-primary" />
                <span className="text-primary">
                  「{inputValue.trim()}」を作成
                </span>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Tag edit dialog */}
      {onUpdateTag && (
        <TagDialog dialog={tagDialog} onSave={handleTagDialogSave} />
      )}
    </div>
  )
}
