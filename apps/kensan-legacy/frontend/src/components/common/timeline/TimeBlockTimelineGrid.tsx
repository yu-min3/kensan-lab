import type { TimeBlockTimelineGridProps } from './types'

/**
 * TimeBlockTimelineGrid - Renders the hour labels and gridlines
 */
export function TimeBlockTimelineGrid({ hours, hourHeight }: TimeBlockTimelineGridProps) {
  return (
    <div className="w-16 flex-shrink-0">
      {hours.map((hour) => (
        <div
          key={hour}
          className="border-t text-xs text-muted-foreground flex items-start pt-1 pr-2 justify-end"
          style={{ height: `${hourHeight}px` }}
        >
          {hour === 24 ? '0:00' : `${hour}:00`}
        </div>
      ))}
    </div>
  )
}
