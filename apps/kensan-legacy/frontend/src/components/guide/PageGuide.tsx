import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { PlayCircle } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useGuideStore } from './useGuideStore'
import { getGuideContent } from './guideContent'
import { SpotlightTour } from './SpotlightTour'

interface PageGuideProps {
  pageId: string
  className?: string
}

export function PageGuide({ pageId, className }: PageGuideProps) {
  const { isCardDismissed } = useGuideStore()
  const [isTourOpen, setIsTourOpen] = useState(false)

  const content = getGuideContent(pageId)
  if (!content?.hasTour || isCardDismissed(pageId)) return null

  const handleStartTour = () => {
    setIsTourOpen(true)
  }

  const handleTourComplete = () => {
    setIsTourOpen(false)
  }

  return (
    <>
      <div className={cn('flex', className)}>
        <Button
          variant="outline"
          size="sm"
          className="gap-1.5"
          onClick={handleStartTour}
        >
          <PlayCircle className="h-3.5 w-3.5" />
          機能説明ツアー
        </Button>
      </div>

      <SpotlightTour
        pageId={pageId}
        isOpen={isTourOpen}
        onComplete={handleTourComplete}
      />
    </>
  )
}
