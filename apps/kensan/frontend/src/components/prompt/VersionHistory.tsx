import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { RotateCcw, Scale, Loader2, Wand2, Pencil } from 'lucide-react'
import type { AIContextVersion } from '@/api/services/prompts'

interface VersionHistoryProps {
  versions: AIContextVersion[]
  activeVersion: number | null
  isLoading: boolean
  onSelectVersion: (version: AIContextVersion) => void
  onRollback: (versionNumber: number) => Promise<void>
  onStartABTest?: (versionNumber: number) => void
  selectedVersionNumber?: number | null
}

function formatDate(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleDateString('ja-JP', { month: 'short', day: 'numeric' }) +
    ' ' + d.toLocaleTimeString('ja-JP', { hour: '2-digit', minute: '2-digit' })
}

function SourceIcon({ source }: { source: string }) {
  switch (source) {
    case 'ai':
      return <Wand2 className="h-3 w-3 text-purple-500" />
    case 'rollback':
      return <RotateCcw className="h-3 w-3 text-orange-500" />
    default:
      return <Pencil className="h-3 w-3 text-muted-foreground" />
  }
}

export function VersionHistory({
  versions,
  activeVersion,
  isLoading,
  onSelectVersion,
  onRollback,
  onStartABTest,
  selectedVersionNumber,
}: VersionHistoryProps) {
  const [rollingBack, setRollingBack] = useState<number | null>(null)

  const handleRollback = async (e: React.MouseEvent, versionNumber: number) => {
    e.stopPropagation()
    setRollingBack(versionNumber)
    try {
      await onRollback(versionNumber)
    } finally {
      setRollingBack(null)
    }
  }

  if (versions.length === 0) {
    return (
      <div className="py-4 text-center text-sm text-muted-foreground">
        バージョン履歴がありません
      </div>
    )
  }

  return (
    <div className="space-y-3">
      <h3 className="text-sm font-medium">バージョン履歴</h3>
      <div className="space-y-1.5">
        {versions.map((version) => {
          const isActive = version.version_number === activeVersion
          const isSelected = version.version_number === selectedVersionNumber

          return (
            <button
              key={version.id}
              onClick={() => onSelectVersion(version)}
              className={`flex w-full items-start gap-3 rounded-lg border p-3 text-sm text-left transition-colors ${
                isSelected
                  ? 'bg-brand/10 border-brand/30'
                  : 'hover:bg-muted/50'
              }`}
            >
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-1.5 flex-wrap">
                  <SourceIcon source={version.source} />
                  <span className="font-medium">v{version.version_number}</span>
                  {isActive && (
                    <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
                      現在
                    </Badge>
                  )}
                  {version.candidate_status === 'pending' && (
                    <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-yellow-500/15 text-yellow-700 dark:text-yellow-400 border-yellow-500/30">
                      候補
                    </Badge>
                  )}
                  {version.candidate_status === 'adopted' && (
                    <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-green-500/15 text-green-700 dark:text-green-400 border-green-500/30">
                      採用
                    </Badge>
                  )}
                  {version.candidate_status === 'rejected' && (
                    <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-gray-500/15 text-gray-600 dark:text-gray-400 border-gray-500/30">
                      却下
                    </Badge>
                  )}
                  <span className="text-xs text-muted-foreground ml-auto shrink-0">
                    {formatDate(version.created_at)}
                  </span>
                </div>
                {version.changelog && (
                  <p className="mt-0.5 text-xs text-muted-foreground truncate">
                    {version.changelog}
                  </p>
                )}
              </div>
              <div className="flex items-center gap-1 shrink-0" onClick={(e) => e.stopPropagation()}>
                {!isActive && onStartABTest && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 px-2"
                    onClick={(e) => {
                      e.stopPropagation()
                      onStartABTest(version.version_number)
                    }}
                    title="A/Bテスト"
                  >
                    <Scale className="h-3.5 w-3.5" />
                  </Button>
                )}
                {!isActive && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 px-2"
                    onClick={(e) => handleRollback(e, version.version_number)}
                    disabled={isLoading || rollingBack !== null}
                    title="このバージョンにロールバック"
                  >
                    {rollingBack === version.version_number ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <RotateCcw className="h-3.5 w-3.5" />
                    )}
                  </Button>
                )}
              </div>
            </button>
          )
        })}
      </div>
    </div>
  )
}
