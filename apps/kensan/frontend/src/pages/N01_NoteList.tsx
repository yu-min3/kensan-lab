import { useState, useEffect, useMemo } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { GoalBadge } from '@/components/common/GoalBadge'
import { Badge } from '@/components/ui/badge'
import { useNoteStore } from '@/stores/useNoteStore'
import { useNoteTypeStore } from '@/stores/useNoteTypeStore'
import { useTaskManagerStore } from '@/stores/useTaskManagerStore'
import { useNoteTagStore } from '@/stores/useNoteTagStore'
import { getNoteTypeIcon } from '@/lib/noteTypeIcons'
import { formatDateIso } from '@/lib/dateFormat'
import type { NoteType } from '@/types'
import {
  FileText,
  Shapes,
  Plus,
  Search,
  StickyNote,
  Archive,
  Tag,
} from 'lucide-react'
import { PageGuide } from '@/components/guide/PageGuide'

export function N01NoteList() {
  const [searchParams, setSearchParams] = useSearchParams()
  const { items, isLoading, fetchNotes, search, searchResults, clearSearchResults } = useNoteStore()
  const { types } = useNoteTypeStore()
  const { goals } = useTaskManagerStore()
  const tags = useNoteTagStore((s) => s.items)

  // Get initial filter from URL
  const initialType = searchParams.get('type') as NoteType | null
  const initialGoalId = searchParams.get('goalId')
  const initialArchived = searchParams.get('archived') === 'true'

  const initialTagIds = searchParams.get('tagIds')?.split(',').filter(Boolean) || []

  const [typeFilter, setTypeFilter] = useState<NoteType | 'all'>(initialType || 'all')
  const [goalFilter, setGoalFilter] = useState<string>(initialGoalId || 'all')
  const [tagFilter, setTagFilter] = useState<string[]>(initialTagIds)
  const [searchQuery, setSearchQuery] = useState('')
  const [tagSearchQuery, setTagSearchQuery] = useState('')
  const [showArchived, setShowArchived] = useState(initialArchived)

  // Fetch notes on mount and when filters change
  useEffect(() => {
    const filter: { types?: NoteType[]; goalId?: string; tagIds?: string[]; archived?: boolean } = {}

    if (typeFilter !== 'all') {
      filter.types = [typeFilter]
    }
    if (goalFilter !== 'all') {
      filter.goalId = goalFilter
    }
    if (tagFilter.length > 0) {
      filter.tagIds = tagFilter
    }
    filter.archived = showArchived

    fetchNotes(filter)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [typeFilter, goalFilter, tagFilter, showArchived])

  // Handle search
  useEffect(() => {
    if (searchQuery.trim()) {
      const filter: { types?: NoteType[]; archived?: boolean } = {}
      if (typeFilter !== 'all') {
        filter.types = [typeFilter]
      }
      filter.archived = showArchived
      search(searchQuery, filter)
    } else {
      clearSearchResults()
    }
  }, [searchQuery, typeFilter, showArchived, search, clearSearchResults])

  // Update URL when filters change
  useEffect(() => {
    const params = new URLSearchParams()
    if (typeFilter !== 'all') params.set('type', typeFilter)
    if (goalFilter !== 'all') params.set('goalId', goalFilter)
    if (tagFilter.length > 0) params.set('tagIds', tagFilter.join(','))
    if (showArchived) params.set('archived', 'true')
    setSearchParams(params, { replace: true })
  }, [typeFilter, goalFilter, tagFilter, showArchived, setSearchParams])

  // Types that should sort by date instead of createdAt
  const dateOrderTypes: NoteType[] = ['diary', 'learning']
  const useDateSort = typeFilter !== 'all' && dateOrderTypes.includes(typeFilter as NoteType)

  // Display search results if searching, otherwise show filtered items with sort
  const displayItems = useMemo(() => {
    const raw = searchQuery.trim()
      ? searchResults.map((r) => r.note)
      : items

    if (!useDateSort || searchQuery.trim()) return raw

    return [...raw].sort((a, b) => {
      // Sort by date desc for diary/learning
      if (a.date && b.date) return b.date.localeCompare(a.date)
      if (a.date) return -1
      if (b.date) return 1
      return b.createdAt.getTime() - a.createdAt.getTime()
    })
  }, [searchQuery, searchResults, items, useDateSort])

  const getCreateLink = () => {
    if (typeFilter === 'all') return '/notes/new'
    return `/notes/new?type=${typeFilter}`
  }

  // Get display info for a note type
  const getTypeDisplayName = (slug: string) => {
    const config = types.find((t) => t.slug === slug)
    return config?.displayName ?? slug
  }

  const getTypeIcon = (slug: string) => {
    const config = types.find((t) => t.slug === slug)
    return getNoteTypeIcon(config?.icon ?? 'file-text')
  }

  return (
    <div className="space-y-6">
      <PageGuide pageId="notes" />

      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <StickyNote className="h-8 w-8 text-slate-500" />
          <h1 className="text-2xl font-bold">ノート</h1>
        </div>
        <Link to={getCreateLink()}>
          <Button className="gap-2">
            <Plus className="h-4 w-4" />
            新規作成
          </Button>
        </Link>
      </div>

      {/* Type tabs - dynamically generated */}
      <Tabs value={typeFilter} onValueChange={(v) => setTypeFilter(v as NoteType | 'all')}>
        <TabsList>
          <TabsTrigger value="all">すべて</TabsTrigger>
          {types.map((noteType) => {
            const Icon = getNoteTypeIcon(noteType.icon)
            return (
              <TabsTrigger key={noteType.slug} value={noteType.slug} className="gap-2">
                <Icon className="h-4 w-4" />
                {noteType.displayName}
              </TabsTrigger>
            )
          })}
        </TabsList>
      </Tabs>

      {/* Filters */}
      <div className="flex gap-4 flex-wrap">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="検索..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-10"
          />
        </div>
        <Select value={goalFilter} onValueChange={setGoalFilter}>
          <SelectTrigger className="w-[180px]">
            <SelectValue placeholder="目標" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">すべての目標</SelectItem>
            {goals
              .filter((g) => g.status !== 'archived')
              .map((goal) => (
                <SelectItem key={goal.id} value={goal.id}>
                  <div className="flex items-center gap-2">
                    <span
                      className="w-2 h-2 rounded-full"
                      style={{ backgroundColor: goal.color }}
                    />
                    {goal.name}
                  </div>
                </SelectItem>
              ))}
          </SelectContent>
        </Select>
        {/* Tag filter */}
        <Popover onOpenChange={(open) => { if (!open) setTagSearchQuery('') }}>
          <PopoverTrigger asChild>
            <Button variant="outline" size="sm" className="gap-2">
              <Tag className="h-4 w-4" />
              タグ
              {tagFilter.length > 0 && (
                <Badge variant="secondary" className="ml-1 px-1.5 text-xs">
                  {tagFilter.length}
                </Badge>
              )}
            </Button>
          </PopoverTrigger>
          <PopoverContent className="w-64 p-0" align="start">
            <div className="p-2 border-b">
              <Input
                placeholder="タグを検索..."
                value={tagSearchQuery}
                onChange={(e) => setTagSearchQuery(e.target.value)}
                className="h-8 text-sm"
                autoFocus
              />
            </div>
            <div className="max-h-60 overflow-y-auto p-1">
              {(() => {
                const q = tagSearchQuery.toLowerCase()
                const unselected = tags.filter(
                  (t) => !tagFilter.includes(t.id) && (!q || t.name.toLowerCase().includes(q))
                )
                const selected = tags.filter((t) => tagFilter.includes(t.id))

                if (tags.length === 0) {
                  return <p className="text-sm text-muted-foreground p-2">タグがありません</p>
                }

                return (
                  <>
                    {/* Selected tags first */}
                    {selected.map((tag) => (
                      <button
                        key={tag.id}
                        type="button"
                        className="flex items-center gap-2 w-full text-left rounded px-2 py-1.5 hover:bg-accent"
                        onClick={() => setTagFilter((prev) => prev.filter((id) => id !== tag.id))}
                      >
                        <span
                          className="w-2.5 h-2.5 rounded-full shrink-0 ring-2 ring-primary"
                          style={{ backgroundColor: tag.color }}
                        />
                        <span className="text-sm truncate flex-1">{tag.name}</span>
                        <span className="text-xs text-muted-foreground">✓</span>
                      </button>
                    ))}
                    {selected.length > 0 && unselected.length > 0 && (
                      <div className="border-t my-1" />
                    )}
                    {/* Unselected tags matching search */}
                    {unselected.map((tag) => (
                      <button
                        key={tag.id}
                        type="button"
                        className="flex items-center gap-2 w-full text-left rounded px-2 py-1.5 hover:bg-accent"
                        onClick={() => {
                          setTagFilter((prev) => [...prev, tag.id])
                          setTagSearchQuery('')
                        }}
                      >
                        <span
                          className="w-2.5 h-2.5 rounded-full shrink-0"
                          style={{ backgroundColor: tag.color }}
                        />
                        <span className="text-sm truncate">{tag.name}</span>
                      </button>
                    ))}
                    {q && unselected.length === 0 && (
                      <p className="text-xs text-muted-foreground p-2">該当するタグがありません</p>
                    )}
                  </>
                )
              })()}
            </div>
            {tagFilter.length > 0 && (
              <div className="border-t p-1">
                <Button
                  variant="ghost"
                  size="sm"
                  className="w-full h-7 text-xs"
                  onClick={() => setTagFilter([])}
                >
                  すべて解除
                </Button>
              </div>
            )}
          </PopoverContent>
        </Popover>

        <Button
          variant={showArchived ? 'default' : 'outline'}
          size="sm"
          onClick={() => setShowArchived(!showArchived)}
          className="gap-2"
        >
          <Archive className="h-4 w-4" />
          {showArchived ? 'アーカイブ済み' : 'アーカイブ'}
        </Button>
      </div>

      {/* Active tag filter badges */}
      {tagFilter.length > 0 && (
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-xs text-muted-foreground">タグ絞り込み:</span>
          {tagFilter.map((tagId) => {
            const tag = tags.find((t) => t.id === tagId)
            if (!tag) return null
            return (
              <Badge
                key={tagId}
                variant="outline"
                className="text-xs cursor-pointer gap-1"
                style={{ borderColor: tag.color, color: tag.color }}
                onClick={() => setTagFilter((prev) => prev.filter((id) => id !== tagId))}
              >
                {tag.name} ×
              </Badge>
            )
          })}
        </div>
      )}

      {/* Loading state */}
      {isLoading && (
        <div className="text-center py-12">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary mx-auto" />
          <p className="text-muted-foreground mt-4">読み込み中...</p>
        </div>
      )}

      {/* Notes grid */}
      {!isLoading && displayItems.length > 0 && (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {displayItems.map((note) => {
            const TypeIcon = getTypeIcon(note.type)
            const FormatIcon = note.format === 'markdown' ? FileText : Shapes

            return (
              <Link key={note.id} to={`/notes/${note.id}`}>
                <Card className="h-full hover:border-primary transition-colors cursor-pointer">
                  <CardContent className="p-4">
                    <div className="flex items-start gap-3">
                      <div className="p-2 rounded-lg bg-sky-100 dark:bg-sky-900/30">
                        <FormatIcon className="h-5 w-5 text-sky-600 dark:text-sky-400" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 mb-1">
                          <Badge variant="secondary" className="text-xs gap-1">
                            <TypeIcon className="h-3 w-3" />
                            {getTypeDisplayName(note.type)}
                          </Badge>
                          {note.archived && (
                            <Badge variant="outline" className="text-xs">
                              アーカイブ済み
                            </Badge>
                          )}
                        </div>
                        <h3 className="font-medium truncate">
                          {note.title || '無題'}
                        </h3>
                        {note.date && (
                          <p className="text-sm text-muted-foreground">
                            {note.date}
                          </p>
                        )}
                        <p className="text-xs text-muted-foreground mt-1">
                          作成: {formatDateIso(note.createdAt)}
                        </p>
                        {(note.goalName || note.milestoneName) && (
                          <div className="flex items-center gap-2 mt-2 flex-wrap">
                            {note.goalName && note.goalColor && (
                              <GoalBadge name={note.goalName} color={note.goalColor} size="sm" />
                            )}
                            {note.milestoneName && (
                              <span className="text-xs text-muted-foreground">
                                {note.milestoneName}
                              </span>
                            )}
                          </div>
                        )}
                        {note.tagIds && note.tagIds.length > 0 && (
                          <div className="flex items-center gap-1 mt-2 flex-wrap">
                            {note.tagIds.slice(0, 3).map((tagId) => {
                              const tag = tags.find((t) => t.id === tagId)
                              return tag ? (
                                <Badge
                                  key={tagId}
                                  variant="outline"
                                  className="text-xs"
                                  style={{ borderColor: tag.color, color: tag.color }}
                                >
                                  {tag.name}
                                </Badge>
                              ) : null
                            })}
                            {note.tagIds.length > 3 && (
                              <span className="text-xs text-muted-foreground">
                                +{note.tagIds.length - 3}
                              </span>
                            )}
                          </div>
                        )}
                      </div>
                    </div>
                  </CardContent>
                </Card>
              </Link>
            )
          })}
        </div>
      )}

      {/* Empty state */}
      {!isLoading && displayItems.length === 0 && (
        <div className="text-center py-12">
          <StickyNote className="h-12 w-12 mx-auto text-muted-foreground/50" />
          <p className="text-muted-foreground mt-4">
            {searchQuery
              ? '該当するノートが見つかりません'
              : showArchived
              ? 'アーカイブされたノートはありません'
              : 'ノートがまだありません'}
          </p>
          <Link to={getCreateLink()}>
            <Button variant="outline" className="mt-4 gap-2">
              <Plus className="h-4 w-4" />
              新規作成
            </Button>
          </Link>
        </div>
      )}
    </div>
  )
}
