import { useState, useEffect, useCallback } from 'react'
import { createPortal } from 'react-dom'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useGuideStore } from './useGuideStore'
import { getTourSteps, type TourStep } from './tourSteps'
import { SpotlightOverlay, useTargetRect } from './SpotlightOverlay'

interface SpotlightTourProps {
  pageId: string
  isOpen: boolean
  onComplete: () => void
}

export function SpotlightTour({ pageId, isOpen, onComplete }: SpotlightTourProps) {
  const { completeTour } = useGuideStore()
  const [stepIndex, setStepIndex] = useState(0)

  const steps = getTourSteps(pageId)

  // Reset step on open
  useEffect(() => {
    if (isOpen) setStepIndex(0)
  }, [isOpen])

  const currentStep = steps?.[stepIndex]

  // Scroll target into view when step changes
  useEffect(() => {
    if (!isOpen || !currentStep) return
    const el = document.querySelector(`[data-guide="${currentStep.target}"]`)
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  }, [isOpen, currentStep])

  const handleNext = useCallback(() => {
    if (!steps) return
    if (stepIndex < steps.length - 1) {
      setStepIndex(stepIndex + 1)
    } else {
      completeTour(pageId)
      onComplete()
    }
  }, [steps, stepIndex, pageId, completeTour, onComplete])

  const handlePrev = useCallback(() => {
    if (stepIndex > 0) setStepIndex(stepIndex - 1)
  }, [stepIndex])

  const handleSkip = useCallback(() => {
    onComplete()
  }, [onComplete])

  if (!isOpen || !steps || !currentStep) return null

  return createPortal(
    <>
      <SpotlightOverlay targetSelector={currentStep.target} onClick={handleSkip} />
      <TourCard
        step={currentStep}
        stepIndex={stepIndex}
        totalSteps={steps.length}
        onNext={handleNext}
        onPrev={handlePrev}
        onSkip={handleSkip}
      />
    </>,
    document.body
  )
}

interface TourCardProps {
  step: TourStep
  stepIndex: number
  totalSteps: number
  onNext: () => void
  onPrev: () => void
  onSkip: () => void
}

function TourCard({ step, stepIndex, totalSteps, onNext, onPrev, onSkip }: TourCardProps) {
  const rect = useTargetRect(step.target)
  const isLast = stepIndex === totalSteps - 1

  if (!rect) return null

  const style = getCardPosition(rect, step.placement)

  return (
    <Card className="fixed z-[101] w-72 shadow-lg animate-in fade-in-0 zoom-in-95 duration-200" style={style}>
      <CardContent className="p-4">
        <div className="flex items-center justify-between mb-1">
          <span className="text-xs text-muted-foreground">
            {stepIndex + 1} / {totalSteps}
          </span>
          <button
            onClick={onSkip}
            className="text-xs text-muted-foreground hover:text-foreground"
          >
            スキップ
          </button>
        </div>
        <h4 className="font-semibold text-sm mb-1">{step.title}</h4>
        <p className="text-sm text-muted-foreground mb-3">{step.description}</p>
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            className={cn('text-xs', stepIndex === 0 && 'invisible')}
            onClick={onPrev}
          >
            戻る
          </Button>
          <div className="flex-1" />
          <Button size="sm" className="text-xs" onClick={onNext}>
            {isLast ? '完了' : '次へ'}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

function getCardPosition(
  targetRect: DOMRect,
  placement: TourStep['placement']
): React.CSSProperties {
  const GAP = 12
  const CARD_WIDTH = 288 // w-72 = 18rem = 288px

  switch (placement) {
    case 'bottom':
      return {
        top: targetRect.bottom + GAP,
        left: Math.max(8, Math.min(targetRect.left, window.innerWidth - CARD_WIDTH - 8)),
      }
    case 'top':
      return {
        bottom: window.innerHeight - targetRect.top + GAP,
        left: Math.max(8, Math.min(targetRect.left, window.innerWidth - CARD_WIDTH - 8)),
      }
    case 'right':
      return {
        top: targetRect.top,
        left: Math.min(targetRect.right + GAP, window.innerWidth - CARD_WIDTH - 8),
      }
    case 'left':
      return {
        top: targetRect.top,
        left: Math.max(8, targetRect.left - CARD_WIDTH - GAP),
      }
  }
}
