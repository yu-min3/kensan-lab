import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { computeLineDiff } from '@/lib/diffUtils'
import type { AIContextVersion } from '@/api/services/prompts'

interface VersionDiffDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  oldVersion: AIContextVersion | null
  newVersion: AIContextVersion | null
}

export function VersionDiffDialog({
  open,
  onOpenChange,
  oldVersion,
  newVersion,
}: VersionDiffDialogProps) {
  if (!oldVersion || !newVersion) return null

  const diffLines = computeLineDiff(oldVersion.system_prompt, newVersion.system_prompt)

  const paramChanges: string[] = []
  if (oldVersion.max_turns !== newVersion.max_turns) {
    paramChanges.push(`max_turns: ${oldVersion.max_turns} → ${newVersion.max_turns}`)
  }
  if (oldVersion.temperature !== newVersion.temperature) {
    paramChanges.push(`temperature: ${oldVersion.temperature} → ${newVersion.temperature}`)
  }
  if (JSON.stringify(oldVersion.allowed_tools) !== JSON.stringify(newVersion.allowed_tools)) {
    paramChanges.push('allowed_tools が変更されました')
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl max-h-[80vh]">
        <DialogHeader>
          <DialogTitle>
            v{oldVersion.version_number} → v{newVersion.version_number} の差分
          </DialogTitle>
          <DialogDescription>
            システムプロンプトの変更箇所
          </DialogDescription>
        </DialogHeader>

        {paramChanges.length > 0 && (
          <div className="flex flex-wrap gap-2">
            {paramChanges.map((change, i) => (
              <Badge key={i} variant="outline" className="text-xs">
                {change}
              </Badge>
            ))}
          </div>
        )}

        <ScrollArea className="h-[50vh]">
          <pre className="text-xs leading-relaxed">
            {diffLines.map((line, i) => (
              <div
                key={i}
                className={
                  line.type === 'added'
                    ? 'bg-green-500/15 text-green-700 dark:text-green-400'
                    : line.type === 'removed'
                      ? 'bg-red-500/15 text-red-700 dark:text-red-400'
                      : 'text-foreground'
                }
              >
                <span className="inline-block w-5 select-none text-right text-muted-foreground/50">
                  {line.type === 'added' ? '+' : line.type === 'removed' ? '-' : ' '}
                </span>
                {' '}{line.content}
              </div>
            ))}
          </pre>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}
