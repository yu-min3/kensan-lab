import { useState, useMemo } from 'react'
import { Card, CardContent } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Collapsible,
  CollapsibleTrigger,
  CollapsibleContent,
} from '@/components/ui/collapsible'
import { ChevronDown } from 'lucide-react'
import { cn } from '@/lib/utils'
import { computeLineDiff } from '@/lib/diffUtils'
import type { DiffLine } from '@/lib/diffUtils'

function InlineDiffLine({ line }: { line: DiffLine }) {
  const baseClass =
    line.type === 'added'
      ? 'bg-green-500/15 text-green-700 dark:text-green-400'
      : line.type === 'removed'
        ? 'bg-red-500/15 text-red-700 dark:text-red-400'
        : 'text-foreground'

  const prefix = line.type === 'added' ? '+' : line.type === 'removed' ? '-' : ' '

  if (line.inlineChanges) {
    return (
      <div className={baseClass}>
        <span className="inline-block w-5 select-none text-right text-muted-foreground/50">
          {prefix}
        </span>
        {' '}
        {line.inlineChanges.map((seg, i) => (
          <span
            key={i}
            className={seg.changed ? (line.type === 'added' ? 'bg-green-500/30' : 'bg-red-500/30') : undefined}
          >
            {seg.text}
          </span>
        ))}
      </div>
    )
  }

  return (
    <div className={baseClass}>
      <span className="inline-block w-5 select-none text-right text-muted-foreground/50">
        {prefix}
      </span>
      {' '}{line.content}
    </div>
  )
}

interface ChallengePromptDiffProps {
  promptA: string
  promptB: string
  labelA?: string
  labelB?: string
}

export function ChallengePromptDiff({ promptA, promptB, labelA = 'A', labelB = 'B' }: ChallengePromptDiffProps) {
  const [diffOpen, setDiffOpen] = useState(false)

  const diffLines = useMemo(() => {
    return computeLineDiff(promptA, promptB)
  }, [promptA, promptB])

  return (
    <Collapsible open={diffOpen} onOpenChange={setDiffOpen}>
      <Card>
        <CardContent className="p-4">
          <CollapsibleTrigger className="flex w-full items-center justify-between">
            <h4 className="text-sm font-medium">プロンプト差分 ({labelA} → {labelB})</h4>
            <ChevronDown className={cn('h-4 w-4 transition-transform', diffOpen && 'rotate-180')} />
          </CollapsibleTrigger>
          <CollapsibleContent>
            <ScrollArea className="mt-3 max-h-[40vh]">
              <pre className="text-xs leading-relaxed">
                {diffLines.map((line, i) => (
                  <InlineDiffLine key={i} line={line} />
                ))}
              </pre>
            </ScrollArea>
          </CollapsibleContent>
        </CardContent>
      </Card>
    </Collapsible>
  )
}
