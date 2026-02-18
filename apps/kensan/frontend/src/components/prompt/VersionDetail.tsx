import { useState } from 'react'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Loader2, CheckCircle2, XCircle, RotateCcw, Scale, Wand2, Pencil } from 'lucide-react'
import { ChallengePromptDiff } from '@/components/prompt/ChallengePromptDiff'
import type { AIContextVersion, EvalSummary } from '@/api/services/prompts'

interface VersionDetailProps {
  version: AIContextVersion
  activeVersion: number | null
  activePrompt: string | null
  onAdopt: (versionNumber: number) => Promise<void>
  onReject: (versionNumber: number) => Promise<void>
  onRollback: (versionNumber: number) => Promise<void>
  onStartABTest: (versionNumber: number) => void
}

function sourceLabel(source: string): { text: string; className: string } {
  switch (source) {
    case 'ai':
      return { text: 'AI最適化', className: 'bg-purple-500/15 text-purple-700 dark:text-purple-400 border-purple-500/30' }
    case 'rollback':
      return { text: 'ロールバック', className: 'bg-orange-500/15 text-orange-700 dark:text-orange-400 border-orange-500/30' }
    default:
      return { text: '手動編集', className: 'bg-gray-500/15 text-gray-700 dark:text-gray-400 border-gray-500/30' }
  }
}

function candidateLabel(status: string | null): { text: string; className: string } | null {
  switch (status) {
    case 'pending':
      return { text: '候補', className: 'bg-yellow-500/15 text-yellow-700 dark:text-yellow-400 border-yellow-500/30' }
    case 'adopted':
      return { text: '採用', className: 'bg-green-500/15 text-green-700 dark:text-green-400 border-green-500/30' }
    case 'rejected':
      return { text: '却下', className: 'bg-gray-500/15 text-gray-600 dark:text-gray-400 border-gray-500/30' }
    default:
      return null
  }
}

function formatDate(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleDateString('ja-JP', { year: 'numeric', month: 'short', day: 'numeric' }) +
    ' ' + d.toLocaleTimeString('ja-JP', { hour: '2-digit', minute: '2-digit' })
}

function EvalSummaryCard({ summary }: { summary: EvalSummary }) {
  return (
    <Card className="border-purple-200 dark:border-purple-800">
      <CardContent className="p-4 space-y-3">
        <h4 className="text-sm font-medium flex items-center gap-1.5">
          <Wand2 className="h-3.5 w-3.5 text-purple-500" />
          AI評価サマリー
        </h4>
        <div className="grid grid-cols-2 gap-3 text-sm">
          {summary.interaction_count != null && (
            <div>
              <span className="text-xs text-muted-foreground">対話数</span>
              <p className="font-medium">{summary.interaction_count}</p>
            </div>
          )}
          {summary.avg_rating != null && (
            <div>
              <span className="text-xs text-muted-foreground">平均評価</span>
              <p className="font-medium">{summary.avg_rating.toFixed(1)} / 5.0</p>
            </div>
          )}
        </div>
        {summary.strengths && summary.strengths.length > 0 && (
          <div>
            <span className="text-xs font-medium text-green-700 dark:text-green-400">強み</span>
            <ul className="mt-1 space-y-0.5">
              {summary.strengths.map((s, i) => (
                <li key={i} className="text-xs text-muted-foreground flex items-start gap-1.5">
                  <span className="text-green-500 mt-0.5 shrink-0">+</span>
                  {s}
                </li>
              ))}
            </ul>
          </div>
        )}
        {summary.weaknesses && summary.weaknesses.length > 0 && (
          <div>
            <span className="text-xs font-medium text-red-700 dark:text-red-400">改善点</span>
            <ul className="mt-1 space-y-0.5">
              {summary.weaknesses.map((w, i) => (
                <li key={i} className="text-xs text-muted-foreground flex items-start gap-1.5">
                  <span className="text-red-500 mt-0.5 shrink-0">-</span>
                  {w}
                </li>
              ))}
            </ul>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

export function VersionDetail({
  version,
  activeVersion,
  activePrompt,
  onAdopt,
  onReject,
  onRollback,
  onStartABTest,
}: VersionDetailProps) {
  const [acting, setActing] = useState<string | null>(null)

  const isActive = version.version_number === activeVersion
  const isPending = version.candidate_status === 'pending'
  const source = sourceLabel(version.source)
  const candidate = candidateLabel(version.candidate_status)

  const handleAction = async (action: string, fn: () => Promise<void>) => {
    setActing(action)
    try {
      await fn()
    } finally {
      setActing(null)
    }
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="space-y-2">
        <div className="flex items-center gap-2 flex-wrap">
          <h3 className="text-lg font-semibold">v{version.version_number}</h3>
          {isActive && (
            <Badge variant="secondary" className="text-[10px]">現在</Badge>
          )}
          <Badge variant="outline" className={`text-[10px] ${source.className}`}>
            {version.source === 'ai' && <Wand2 className="h-2.5 w-2.5 mr-0.5" />}
            {version.source === 'rollback' && <RotateCcw className="h-2.5 w-2.5 mr-0.5" />}
            {version.source === 'manual' && <Pencil className="h-2.5 w-2.5 mr-0.5" />}
            {source.text}
          </Badge>
          {candidate && (
            <Badge variant="outline" className={`text-[10px] ${candidate.className}`}>
              {candidate.text}
            </Badge>
          )}
        </div>
        <p className="text-xs text-muted-foreground">{formatDate(version.created_at)}</p>
        {version.changelog && (
          <p className="text-sm text-muted-foreground">{version.changelog}</p>
        )}
      </div>

      {/* Eval summary for AI candidates */}
      {version.eval_summary && <EvalSummaryCard summary={version.eval_summary} />}

      {/* Prompt diff against active version */}
      {activePrompt && !isActive && (
        <ChallengePromptDiff
          promptA={activePrompt}
          promptB={version.system_prompt}
          labelA={`v${activeVersion ?? '?'} (現在)`}
          labelB={`v${version.version_number}`}
        />
      )}

      {/* Actions */}
      {!isActive && (
        <div className="flex items-center gap-2 flex-wrap">
          {isPending && (
            <>
              <Button
                size="sm"
                className="gap-1.5"
                disabled={acting !== null}
                onClick={() => handleAction('adopt', () => onAdopt(version.version_number))}
              >
                {acting === 'adopt' ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <CheckCircle2 className="h-3.5 w-3.5" />}
                採用
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="gap-1.5"
                disabled={acting !== null}
                onClick={() => handleAction('reject', () => onReject(version.version_number))}
              >
                {acting === 'reject' ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <XCircle className="h-3.5 w-3.5" />}
                却下
              </Button>
            </>
          )}
          {!isPending && (
            <Button
              variant="outline"
              size="sm"
              className="gap-1.5"
              disabled={acting !== null}
              onClick={() => handleAction('rollback', () => onRollback(version.version_number))}
            >
              {acting === 'rollback' ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RotateCcw className="h-3.5 w-3.5" />}
              ロールバック
            </Button>
          )}
          <Button
            variant="outline"
            size="sm"
            className="gap-1.5"
            disabled={acting !== null}
            onClick={() => onStartABTest(version.version_number)}
          >
            <Scale className="h-3.5 w-3.5" />
            A/Bテスト
          </Button>
        </div>
      )}

      {isActive && (
        <p className="text-xs text-muted-foreground">
          現在アクティブなバージョンです。
        </p>
      )}
    </div>
  )
}
