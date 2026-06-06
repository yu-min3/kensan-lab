import { useMemo } from 'react'
import { Check } from 'lucide-react'
import { cn } from '@/lib/utils'
import {
  TimelineCore,
  type TimelineBlock,
  type BlockRenderContext,
  getMinutesFromTime,
} from '@/components/common/timeline'
import { getLocalTime } from '@/lib/timezone'
import type { ActionItem } from '@/stores/useChatStore'
import type { TimeBlock as DomainTimeBlock } from '@/types'

interface TimeBlockAction {
  action: ActionItem
  date: string
  startTime: string
  endTime: string
  taskName: string
  goalColor?: string
  goalName?: string
}

interface ProposalTimelineProps {
  blocks: TimeBlockAction[]
  existingBlocks: DomainTimeBlock[]
  selected: Set<string>
  onToggle: (actionId: string) => void
  disabled?: boolean
  timezone: string
}

export function ProposalTimeline({
  blocks,
  existingBlocks,
  selected,
  onToggle,
  disabled,
  timezone,
}: ProposalTimelineProps) {
  const { timelineBlocks, columns, startHour, endHour, initialScrollHour } = useMemo(() => {
    // Convert existing blocks to TimelineBlock → "existing" column
    const existing: TimelineBlock[] = existingBlocks.map((tb) => ({
      id: `existing-${tb.id}`,
      columnId: 'existing',
      startTime: getLocalTime(tb.startDatetime, timezone),
      endTime: getLocalTime(tb.endDatetime, timezone),
      label: tb.taskName,
      sublabel: tb.goalName,
      color: tb.goalColor,
    }))

    // Convert proposal blocks to TimelineBlock → "proposal" column
    const proposals: TimelineBlock[] = blocks.map((tb) => ({
      id: `proposal-${tb.action.id}`,
      columnId: 'proposal',
      startTime: tb.startTime,
      endTime: tb.endTime,
      label: tb.taskName,
      sublabel: tb.goalName,
      color: tb.goalColor,
    }))

    const allBlocks = [...existing, ...proposals]

    // Calculate hour range from data
    let minHour = 24
    let maxHour = 0
    for (const b of allBlocks) {
      const startMin = getMinutesFromTime(b.startTime)
      const endMin = getMinutesFromTime(b.endTime)
      minHour = Math.min(minHour, Math.floor(startMin / 60))
      maxHour = Math.max(maxHour, Math.ceil(endMin / 60))
    }

    // Default range when no blocks exist
    if (allBlocks.length === 0) {
      minHour = 8
      maxHour = 18
    }

    // Add 1h padding, clamp to valid range
    const sh = Math.max(0, minHour - 1)
    const eh = Math.min(24, maxHour + 1)

    // initialScrollHour = first proposal block - 1h
    let scroll = sh
    if (proposals.length > 0) {
      const firstProposalMin = Math.min(
        ...proposals.map((p) => getMinutesFromTime(p.startTime))
      )
      scroll = Math.max(sh, Math.floor(firstProposalMin / 60) - 1)
    }

    // Always show both columns so the user can see the comparison
    const cols = [
      { id: 'existing', header: <span className="text-[10px] text-muted-foreground">現在</span> },
      { id: 'proposal', header: <span className="text-[10px] text-muted-foreground">提案</span> },
    ]

    return {
      timelineBlocks: allBlocks,
      columns: cols,
      startHour: sh,
      endHour: eh,
      initialScrollHour: scroll,
    }
  }, [blocks, existingBlocks, timezone])

  const proposalIds = useMemo(
    () => new Set(blocks.map((b) => `proposal-${b.action.id}`)),
    [blocks]
  )

  const renderBlock = useMemo(() => {
    return (ctx: BlockRenderContext) => {
      const { block, displayTimes, getTopPx, getHeightPx } = ctx
      const top = getTopPx(displayTimes.startTime)
      const height = getHeightPx(displayTimes.startTime, displayTimes.endTime)
      const isProposal = proposalIds.has(block.id)

      if (!isProposal) {
        // Existing block: full opacity, no interaction
        return (
          <div
            data-block
            className="absolute left-0.5 right-0.5 rounded-sm overflow-hidden text-[9px] px-1 leading-tight pointer-events-none"
            style={{
              top,
              height,
              backgroundColor: block.color || 'hsl(var(--muted))',
              borderLeft: block.color ? `3px solid ${block.color}` : undefined,
            }}
          >
            <div className="truncate font-medium pt-0.5 text-foreground">
              {block.label}
            </div>
            {height >= 28 && block.sublabel && (
              <div className="truncate opacity-70 text-[8px]">
                {block.sublabel}
              </div>
            )}
          </div>
        )
      }

      // Proposal block: checkbox, dashed border, toggle state
      const actionId = block.id.replace('proposal-', '')
      const isSelected = selected.has(actionId)

      return (
        <div
          data-block
          className={cn(
            'absolute left-0.5 right-0.5 rounded-sm overflow-hidden text-[9px] px-1.5 leading-tight border-2 border-dashed transition-opacity cursor-pointer',
            isSelected
              ? 'opacity-100 bg-primary/10 border-primary/50'
              : 'opacity-40 bg-muted/30 border-muted-foreground/30'
          )}
          style={{ top, height }}
          onClick={(e) => {
            e.stopPropagation()
            if (!disabled) onToggle(actionId)
          }}
        >
          <div className="flex items-start gap-1 pt-0.5">
            <div
              className={cn(
                'shrink-0 mt-px w-3 h-3 rounded-sm border flex items-center justify-center',
                isSelected
                  ? 'bg-primary border-primary text-primary-foreground'
                  : 'border-muted-foreground/50'
              )}
            >
              {isSelected && <Check className="h-2 w-2" />}
            </div>
            <div className="min-w-0 flex-1">
              <div
                className={cn(
                  'truncate font-medium',
                  !isSelected && 'line-through'
                )}
              >
                {displayTimes.startTime}-{displayTimes.endTime} {block.label}
              </div>
              {height >= 28 && block.sublabel && (
                <div className="truncate opacity-70 text-[8px]">
                  {block.sublabel}
                </div>
              )}
            </div>
          </div>
        </div>
      )
    }
  }, [proposalIds, selected, onToggle, disabled])

  return (
    <TimelineCore
      columns={columns}
      blocks={timelineBlocks}
      startHour={startHour}
      endHour={endHour}
      baseHourHeight={32}
      showZoomControls={false}
      initialScrollHour={initialScrollHour}
      renderBlock={renderBlock}
    />
  )
}
