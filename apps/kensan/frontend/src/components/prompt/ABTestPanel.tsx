import { useState, useRef, useCallback } from 'react'
import { cn } from '@/lib/utils'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { X, CheckCircle2, Scale, Maximize2, Minimize2 } from 'lucide-react'
import { ChallengeChatPanel, type ChallengeChatPanelHandle } from '@/components/prompt/ChallengeChatPanel'
import { ChatInput } from '@/components/agent/ChatInput'

interface ABTestPanelProps {
  contextId: string
  versionA: number
  versionB: number
  onClose: () => void
  onAdoptVersion: (versionNumber: number) => Promise<void>
}

interface RoundVote {
  winner: 'A' | 'B' | null
}

export function ABTestPanel({
  contextId,
  versionA,
  versionB,
  onClose,
  onAdoptVersion,
}: ABTestPanelProps) {
  // A (current) on left, B (candidate) on right
  const mapping = {
    left: versionA,
    right: versionB,
    leftLabel: 'A' as const,
    rightLabel: 'B' as const,
  }

  const [rounds, setRounds] = useState<RoundVote[]>([])
  const [bothDone, setBothDone] = useState(false)
  const [isMaximized, setIsMaximized] = useState(false)
  const leftRef = useRef<ChallengeChatPanelHandle>(null)
  const rightRef = useRef<ChallengeChatPanelHandle>(null)
  const streamDoneCount = useRef(0)

  const handleStreamEnd = useCallback(() => {
    streamDoneCount.current += 1
    if (streamDoneCount.current >= 2) {
      setBothDone(true)
    }
  }, [])

  const handleSendBoth = useCallback((text: string) => {
    streamDoneCount.current = 0
    setBothDone(false)
    leftRef.current?.sendMessage(text)
    rightRef.current?.sendMessage(text)
  }, [])

  const handleVote = (winner: 'A' | 'B') => {
    setRounds((prev) => [...prev, { winner }])
  }

  const winsA = rounds.filter((r) => r.winner === 'A').length
  const winsB = rounds.filter((r) => r.winner === 'B').length

  const content = (
    <>
      {/* Shared input */}
      <Card>
        <CardContent className="p-3">
          <p className="text-xs text-muted-foreground mb-2">
            同じメッセージを両バージョンに送信します
          </p>
          <ChatInput
            onSend={handleSendBoth}
            disabled={false}
            placeholder="両パネルに送信するメッセージ..."
          />
        </CardContent>
      </Card>

      {/* Chat panels */}
      <div className={cn('grid grid-cols-2 gap-3', isMaximized ? 'flex-1 min-h-0' : 'h-[50vh]')}>
        <ChallengeChatPanel
          ref={leftRef}
          label={`パネル ${mapping.leftLabel} (v${mapping.left})`}
          contextId={contextId}
          versionNumber={mapping.left}
          onStreamEnd={handleStreamEnd}
        />
        <ChallengeChatPanel
          ref={rightRef}
          label={`パネル ${mapping.rightLabel} (v${mapping.right})`}
          contextId={contextId}
          versionNumber={mapping.right}
          onStreamEnd={handleStreamEnd}
        />
      </div>

      {/* Vote */}
      {bothDone && (
        <Card className="shrink-0">
          <CardContent className="p-3 flex items-center justify-center gap-3">
            <span className="text-xs text-muted-foreground">どちらが良かったですか？</span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleVote(mapping.leftLabel as 'A' | 'B')}
            >
              パネル {mapping.leftLabel} (v{mapping.left})
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleVote(mapping.rightLabel as 'A' | 'B')}
            >
              パネル {mapping.rightLabel} (v{mapping.right})
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Score + resolve */}
      {rounds.length > 0 && (
        <Card className="shrink-0">
          <CardContent className="p-3 space-y-2">
            <div className="flex items-center justify-between text-sm">
              <span>スコア: v{versionA} = {winsA}勝 / v{versionB} = {winsB}勝 ({rounds.length}ラウンド)</span>
            </div>
            <div className="flex items-center gap-2">
              <Button
                size="sm"
                className="gap-1.5"
                disabled={winsA === 0}
                onClick={() => onAdoptVersion(versionA)}
              >
                <CheckCircle2 className="h-3.5 w-3.5" />
                v{versionA} を採用
              </Button>
              <Button
                size="sm"
                className="gap-1.5"
                disabled={winsB === 0}
                onClick={() => onAdoptVersion(versionB)}
              >
                <CheckCircle2 className="h-3.5 w-3.5" />
                v{versionB} を採用
              </Button>
              <Button variant="ghost" size="sm" onClick={onClose}>
                閉じる
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
    </>
  )

  const header = (
    <div className="flex items-center justify-between shrink-0">
      <div className="flex items-center gap-2">
        <Scale className="h-4 w-4 text-brand" />
        <h3 className="text-sm font-semibold">A/Bテスト</h3>
        <Badge variant="outline" className="text-[10px]">
          v{versionA} vs v{versionB}
        </Badge>
      </div>
      <div className="flex items-center gap-1">
        <Button
          variant="ghost"
          size="sm"
          className="h-7 w-7 p-0"
          onClick={() => setIsMaximized(!isMaximized)}
          title={isMaximized ? '元に戻す' : '最大化'}
        >
          {isMaximized ? <Minimize2 className="h-4 w-4" /> : <Maximize2 className="h-4 w-4" />}
        </Button>
        <Button variant="ghost" size="sm" className="h-7 w-7 p-0" onClick={onClose}>
          <X className="h-4 w-4" />
        </Button>
      </div>
    </div>
  )

  if (isMaximized) {
    return (
      <div className="fixed inset-0 z-50 bg-background flex flex-col p-4 gap-3">
        {header}
        {content}
      </div>
    )
  }

  return (
    <div className="space-y-3">
      {header}
      {content}
    </div>
  )
}
