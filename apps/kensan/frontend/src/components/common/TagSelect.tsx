import { useState, useRef, useEffect } from 'react'
import { Plus, Check } from 'lucide-react'
import { cn } from '@/lib/utils'
import { TagBadge } from '@/components/common/TagBadge'
import type { Tag } from '@/types'

interface TagSelectProps {
  tags: Tag[]
  selectedTagIds: string[]
  onSelectedTagIdsChange: (tagIds: string[]) => void
  placeholder?: string
}

export function TagSelect({
  tags,
  selectedTagIds,
  onSelectedTagIdsChange,
  placeholder = 'タグを追加',
}: TagSelectProps) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  // Close on outside click
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    if (open) {
      document.addEventListener('mousedown', handleClickOutside)
      return () => document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [open])

  const selectedTags = tags.filter(t => selectedTagIds.includes(t.id))

  const handleToggleTag = (tagId: string) => {
    if (selectedTagIds.includes(tagId)) {
      onSelectedTagIdsChange(selectedTagIds.filter(id => id !== tagId))
    } else {
      onSelectedTagIdsChange([...selectedTagIds, tagId])
    }
  }

  const handleRemoveTag = (tagId: string) => {
    onSelectedTagIdsChange(selectedTagIds.filter(id => id !== tagId))
  }

  return (
    <div ref={containerRef} className="relative">
      {/* Selected tags and trigger */}
      <div
        className={cn(
          'flex flex-wrap gap-1.5 min-h-9 w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm',
          'focus-within:ring-1 focus-within:ring-ring cursor-pointer'
        )}
        onClick={() => setOpen(!open)}
      >
        {selectedTags.map(tag => (
          <TagBadge
            key={tag.id}
            name={tag.name}
            color={tag.color}
            onRemove={() => handleRemoveTag(tag.id)}
          />
        ))}
        {selectedTags.length === 0 && (
          <span className="text-muted-foreground flex items-center gap-1">
            <Plus className="h-3.5 w-3.5" />
            {placeholder}
          </span>
        )}
      </div>

      {/* Dropdown */}
      {open && (
        <div className="absolute z-50 mt-1 w-full rounded-md border bg-popover text-popover-foreground shadow-md">
          <div className="max-h-48 overflow-y-auto p-1">
            {tags.length === 0 ? (
              <div className="py-4 text-center text-sm text-muted-foreground">
                タグがありません
              </div>
            ) : (
              tags.map(tag => {
                const isSelected = selectedTagIds.includes(tag.id)
                return (
                  <div
                    key={tag.id}
                    onClick={(e) => {
                      e.stopPropagation()
                      handleToggleTag(tag.id)
                    }}
                    className={cn(
                      'flex items-center gap-2 rounded-sm px-2 py-1.5 text-sm cursor-pointer',
                      'hover:bg-accent hover:text-accent-foreground',
                      isSelected && 'bg-accent/50'
                    )}
                  >
                    <span
                      className="w-3 h-3 rounded-full flex-shrink-0"
                      style={{ backgroundColor: tag.color }}
                    />
                    <span className="flex-1">{tag.name}</span>
                    {isSelected && <Check className="h-4 w-4" />}
                  </div>
                )
              })
            )}
          </div>
        </div>
      )}
    </div>
  )
}
