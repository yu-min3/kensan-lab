import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  Activity,
  RefreshCw,
  Loader2,
  Zap,
  Hash,
  ArrowDownUp,
} from 'lucide-react'
import { InteractionTable } from '@/components/interactions/InteractionTable'
import { PageGuide } from '@/components/guide/PageGuide'
import { fetchInteractions, type Interaction } from '@/api/services/observability'

type TimeRange = '30m' | '1h' | '3h' | '6h' | '24h'

const TIME_RANGE_OPTIONS: { value: TimeRange; label: string }[] = [
  { value: '30m', label: '30 min' },
  { value: '1h', label: '1 hour' },
  { value: '3h', label: '3 hours' },
  { value: '6h', label: '6 hours' },
  { value: '24h', label: '24 hours' },
]

function timeRangeToMs(range: TimeRange): number {
  const map: Record<TimeRange, number> = {
    '30m': 30 * 60 * 1000,
    '1h': 60 * 60 * 1000,
    '3h': 3 * 60 * 60 * 1000,
    '6h': 6 * 60 * 60 * 1000,
    '24h': 24 * 60 * 60 * 1000,
  }
  return map[range]
}

export function O01InteractionExplorer() {
  const [interactions, setInteractions] = useState<Interaction[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [timeRange, setTimeRange] = useState<TimeRange>('3h')
  const [outcomeFilter, setOutcomeFilter] = useState<string>('all')

  const loadData = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const end = new Date()
      const start = new Date(end.getTime() - timeRangeToMs(timeRange))
      const data = await fetchInteractions(start, end)
      setInteractions(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load interactions')
    } finally {
      setLoading(false)
    }
  }, [timeRange])

  useEffect(() => {
    loadData()
  }, [loadData])

  const filteredInteractions = outcomeFilter === 'all'
    ? interactions
    : interactions.filter((i) => i.outcome === outcomeFilter)

  // Stats
  const totalInteractions = interactions.length
  const successCount = interactions.filter((i) => i.outcome === 'success').length
  const avgTokens = totalInteractions > 0
    ? Math.round(interactions.reduce((sum, i) => sum + i.totalInputTokens + i.totalOutputTokens, 0) / totalInteractions)
    : 0
  const avgTurns = totalInteractions > 0
    ? (interactions.reduce((sum, i) => sum + i.totalTurns, 0) / totalInteractions).toFixed(1)
    : '0'

  return (
    <div className="space-y-4">
      <PageGuide pageId="interactions" />

      {/* Header */}
      <div data-guide="explorer-header" className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Activity className="h-5 w-5" />
          <h1 className="text-xl font-semibold">AI Interaction Explorer</h1>
        </div>
        <div className="flex items-center gap-2">
          <Select value={outcomeFilter} onValueChange={setOutcomeFilter}>
            <SelectTrigger className="w-[140px] h-8 text-xs">
              <SelectValue placeholder="Outcome" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All outcomes</SelectItem>
              <SelectItem value="success">success</SelectItem>
              <SelectItem value="error">error</SelectItem>
              <SelectItem value="max_turns_reached">max_turns</SelectItem>
            </SelectContent>
          </Select>
          <Select value={timeRange} onValueChange={(v) => setTimeRange(v as TimeRange)}>
            <SelectTrigger className="w-[110px] h-8 text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {TIME_RANGE_OPTIONS.map((opt) => (
                <SelectItem key={opt.value} value={opt.value}>
                  {opt.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button variant="outline" size="sm" onClick={loadData} disabled={loading}>
            {loading ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <RefreshCw className="h-4 w-4" />
            )}
          </Button>
        </div>
      </div>

      {/* Stats */}
      <div data-guide="explorer-stats" className="grid grid-cols-4 gap-3">
        <Card>
          <CardHeader className="pb-1 pt-3 px-4">
            <CardTitle className="text-xs font-medium text-muted-foreground flex items-center gap-1">
              <Hash className="h-3 w-3" />
              Interactions
            </CardTitle>
          </CardHeader>
          <CardContent className="px-4 pb-3">
            <div className="text-2xl font-bold tabular-nums">{totalInteractions}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-1 pt-3 px-4">
            <CardTitle className="text-xs font-medium text-muted-foreground flex items-center gap-1">
              <Zap className="h-3 w-3" />
              Success Rate
            </CardTitle>
          </CardHeader>
          <CardContent className="px-4 pb-3">
            <div className="text-2xl font-bold tabular-nums">
              {totalInteractions > 0 ? Math.round((successCount / totalInteractions) * 100) : 0}%
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-1 pt-3 px-4">
            <CardTitle className="text-xs font-medium text-muted-foreground flex items-center gap-1">
              <ArrowDownUp className="h-3 w-3" />
              Avg Tokens
            </CardTitle>
          </CardHeader>
          <CardContent className="px-4 pb-3">
            <div className="text-2xl font-bold tabular-nums">{avgTokens.toLocaleString()}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-1 pt-3 px-4">
            <CardTitle className="text-xs font-medium text-muted-foreground flex items-center gap-1">
              <RefreshCw className="h-3 w-3" />
              Avg Turns
            </CardTitle>
          </CardHeader>
          <CardContent className="px-4 pb-3">
            <div className="text-2xl font-bold tabular-nums">{avgTurns}</div>
          </CardContent>
        </Card>
      </div>

      {/* Error */}
      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-950/50 p-3 text-sm text-red-800 dark:text-red-300">
          {error}
        </div>
      )}

      {/* Interaction table */}
      <div data-guide="explorer-table">
      {loading && interactions.length === 0 ? (
        <div className="flex items-center justify-center py-16">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : (
        <InteractionTable interactions={filteredInteractions} />
      )}
      </div>
    </div>
  )
}
