import { useState, useEffect, useCallback, useRef } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate, useLocation } from 'react-router-dom'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { KensanLogo } from '@/components/common/KensanLogo'
import { Clock, BookOpen, BarChart3, PlayCircle, MessageSquare, RefreshCw, Sparkles, ChevronDown } from 'lucide-react'
import { useGuideStore } from './useGuideStore'
import { SpotlightOverlay, useTargetRect } from './SpotlightOverlay'
import { demoTourSteps, type DemoTourStep } from './tourSteps'
import { useChatStore } from '@/stores/useChatStore'
import { useAuthStore } from '@/stores/useAuthStore'

const POLL_INTERVAL_MS = 200
const POLL_MAX_MS = 8000

type Phase = 'welcome' | 'touring'

export function DemoTour() {
  const { isDemoTourActive, startDemoTour, completeDemoTour, skipDemoTour, isDemoTourCompletedFor } = useGuideStore()
  const userId = useAuthStore((s) => s.user?.id)
  const navigate = useNavigate()
  const location = useLocation()
  const [phase, setPhase] = useState<Phase>('welcome')
  const [stepIndex, setStepIndex] = useState(0)
  const [isReady, setIsReady] = useState(false)
  const [targetFound, setTargetFound] = useState(false)
  const cancelRef = useRef(false)

  const steps = demoTourSteps
  const currentStep = steps[stepIndex] as DemoTourStep | undefined

  // Auto-start demo tour on first visit (per user)
  useEffect(() => {
    if (!userId) return
    if (!isDemoTourCompletedFor(userId) && !isDemoTourActive) {
      startDemoTour()
    }
  }, [userId, isDemoTourActive, startDemoTour, isDemoTourCompletedFor])

  // Reset phase when tour starts
  useEffect(() => {
    if (isDemoTourActive) {
      setPhase('welcome')
      setStepIndex(0)
      setIsReady(false)
      setTargetFound(false)
    }
  }, [isDemoTourActive])

  // Handle page navigation and DOM readiness (touring phase only)
  useEffect(() => {
    if (!isDemoTourActive || phase !== 'touring' || !currentStep) return

    cancelRef.current = false
    setIsReady(false)
    setTargetFound(false)

    // Execute action
    if (currentStep.action === 'open-chat') {
      useChatStore.getState().open()
    } else if (currentStep.action === 'close-chat') {
      useChatStore.getState().close()
    } else if (currentStep.action === 'switch-optimize-tab') {
      const tabTrigger = document.querySelector('[data-guide="prompt-tab-optimize"]') as HTMLElement | null
      if (tabTrigger) tabTrigger.click()
    }

    // Navigate if needed
    const needsNav = location.pathname !== currentStep.page
    if (needsNav) {
      navigate(currentStep.page)
    }

    // No target -> center card, ready after short delay
    if (!currentStep.target) {
      const t = setTimeout(() => {
        if (!cancelRef.current) {
          setTargetFound(false)
          setIsReady(true)
        }
      }, needsNav ? 300 : 100)
      return () => { cancelRef.current = true; clearTimeout(t) }
    }

    // Poll for target element to appear in DOM
    const startedAt = Date.now()
    const poll = () => {
      if (cancelRef.current) return
      const el = document.querySelector(`[data-guide="${currentStep.target}"]`)
      if (el) {
        // Scroll target into view
        el.scrollIntoView({ behavior: 'smooth', block: 'center' })
        // Small delay after scroll to let layout settle
        setTimeout(() => {
          if (!cancelRef.current) {
            setTargetFound(true)
            setIsReady(true)
          }
        }, 300)
        return
      }
      if (Date.now() - startedAt > POLL_MAX_MS) {
        // Target never appeared — fallback to center card
        setTargetFound(false)
        setIsReady(true)
        return
      }
      setTimeout(poll, POLL_INTERVAL_MS)
    }
    const initDelay = setTimeout(poll, needsNav ? 300 : 50)
    return () => { cancelRef.current = true; clearTimeout(initDelay) }
  }, [isDemoTourActive, phase, stepIndex, currentStep, location.pathname, navigate])

  const handleStartTour = useCallback(() => {
    setPhase('touring')
  }, [])

  const handleNext = useCallback(() => {
    if (stepIndex < steps.length - 1) {
      setStepIndex(stepIndex + 1)
    } else {
      useChatStore.getState().close()
      if (userId) completeDemoTour(userId)
    }
  }, [stepIndex, steps.length, completeDemoTour, userId])

  const handlePrev = useCallback(() => {
    if (stepIndex > 0) setStepIndex(stepIndex - 1)
  }, [stepIndex])

  const handleSkip = useCallback(() => {
    useChatStore.getState().close()
    if (userId) skipDemoTour(userId)
  }, [skipDemoTour, userId])

  if (!isDemoTourActive) return null

  // Welcome phase
  if (phase === 'welcome') {
    return createPortal(<WelcomeScreen onStart={handleStartTour} onSkip={handleSkip} />, document.body)
  }

  // Touring phase
  if (!currentStep || !isReady) return null

  // Use spotlight only when target actually exists in DOM
  const useSpotlight = targetFound && !!currentStep.target

  return createPortal(
    <>
      {useSpotlight ? (
        <SpotlightOverlay targetSelector={currentStep.target!} onClick={handleSkip} />
      ) : (
        <div
          className="fixed inset-0 z-[100]"
          style={{ backgroundColor: 'rgba(0, 0, 0, 0.5)' }}
          onClick={handleSkip}
        />
      )}
      {useSpotlight ? (
        <TargetedDemoCard
          step={currentStep}
          stepIndex={stepIndex}
          totalSteps={steps.length}
          onNext={handleNext}
          onPrev={handlePrev}
          onSkip={handleSkip}
        />
      ) : currentStep.customCard === 'ai-intro' ? (
        <AIIntroCard
          step={currentStep}
          stepIndex={stepIndex}
          totalSteps={steps.length}
          onNext={handleNext}
          onPrev={handlePrev}
          onSkip={handleSkip}
        />
      ) : (
        <CenteredDemoCard
          step={currentStep}
          stepIndex={stepIndex}
          totalSteps={steps.length}
          onNext={handleNext}
          onPrev={handlePrev}
          onSkip={handleSkip}
        />
      )}
    </>,
    document.body
  )
}

// ---------- Welcome Screen ----------

const FEATURES = [
  {
    icon: Clock,
    title: '時間管理 & 目標達成',
    desc: '時間を記録し、目標とタスクの管理で達成をサポートします',
  },
  {
    icon: BookOpen,
    title: 'ナレッジ & ノート',
    desc: '学習記録や日記など、さまざまな形でナレッジを整理できます',
  },
  {
    icon: BarChart3,
    title: '分析 & 週次レビュー',
    desc: '活動データを可視化し、週次レビューで成長を振り返ります',
  },
]

interface WelcomeScreenProps {
  onStart: () => void
  onSkip: () => void
}

function WelcomeScreen({ onStart, onSkip }: WelcomeScreenProps) {
  const [visible, setVisible] = useState(false)

  useEffect(() => {
    requestAnimationFrame(() => setVisible(true))
  }, [])

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center">
      {/* Backdrop */}
      <div
        className="absolute inset-0 transition-opacity duration-500"
        style={{
          backgroundColor: 'rgba(0, 0, 0, 0.6)',
          backdropFilter: 'blur(4px)',
          opacity: visible ? 1 : 0,
        }}
        onClick={onSkip}
      />

      {/* Card */}
      <div
        className={cn(
          'relative z-10 w-full max-w-md mx-4 rounded-2xl border bg-card text-card-foreground shadow-2xl overflow-hidden transition-all duration-500',
          visible ? 'opacity-100 scale-100 translate-y-0' : 'opacity-0 scale-95 translate-y-4'
        )}
      >
        {/* Brand gradient header */}
        <div className="relative px-8 pt-10 pb-8 text-center overflow-hidden">
          {/* Subtle gradient background */}
          <div
            className="absolute inset-0 opacity-[0.07]"
            style={{
              background: 'linear-gradient(135deg, hsl(var(--brand)) 0%, transparent 60%)',
            }}
          />

          <div className="relative">
            <div className="flex justify-center mb-4">
              <div
                className="rounded-2xl p-3"
                style={{
                  backgroundColor: 'hsl(var(--brand) / 0.1)',
                }}
              >
                <KensanLogo size={56} />
              </div>
            </div>
            <h1 className="text-3xl font-bold tracking-tight mb-1">Kensan</h1>
            <p className="text-sm text-muted-foreground leading-relaxed">
              時間管理・ナレッジ・分析を組み合わせ
              <br />
              あなた専用のAIが、すべての研鑽を支えます
            </p>
          </div>
        </div>

        {/* Features */}
        <div className="px-8 pb-2 space-y-3">
          {FEATURES.map((f, i) => (
            <div
              key={i}
              className={cn(
                'flex items-start gap-3 transition-all duration-500',
                visible ? 'opacity-100 translate-x-0' : 'opacity-0 -translate-x-4',
              )}
              style={{ transitionDelay: `${300 + i * 100}ms` }}
            >
              <div
                className="mt-0.5 rounded-lg p-1.5 shrink-0"
                style={{ backgroundColor: 'hsl(var(--brand) / 0.1)' }}
              >
                <f.icon className="h-4 w-4" style={{ color: 'hsl(var(--brand))' }} />
              </div>
              <div>
                <p className="text-sm font-medium">{f.title}</p>
                <p className="text-xs text-muted-foreground leading-relaxed">{f.desc}</p>
              </div>
            </div>
          ))}
        </div>

        {/* Actions */}
        <div className="px-8 pt-6 pb-8 flex flex-col gap-2">
          <Button
            className="w-full gap-2"
            size="lg"
            onClick={onStart}
            style={{
              backgroundColor: 'hsl(var(--brand))',
              color: 'hsl(var(--brand-foreground))',
            }}
          >
            <PlayCircle className="h-4 w-4" />
            機能説明ツアー
          </Button>
          <button
            onClick={onSkip}
            className="text-xs text-muted-foreground hover:text-foreground transition-colors py-1"
          >
            スキップして始める
          </button>
        </div>
      </div>
    </div>
  )
}

// ---------- Tour Step Cards ----------

interface DemoCardProps {
  step: DemoTourStep
  stepIndex: number
  totalSteps: number
  onNext: () => void
  onPrev: () => void
  onSkip: () => void
}

function TargetedDemoCard({ step, stepIndex, totalSteps, onNext, onPrev, onSkip }: DemoCardProps) {
  const rect = useTargetRect(step.target!)
  const isLast = stepIndex === totalSteps - 1

  if (!rect) return null

  const style = getCardPosition(rect, step.placement as 'top' | 'bottom' | 'left' | 'right')

  return (
    <Card className="fixed z-[101] w-80 shadow-lg animate-in fade-in-0 zoom-in-95 duration-200" style={style}>
      <CardContent className="p-4">
        <DemoCardContent
          step={step}
          stepIndex={stepIndex}
          totalSteps={totalSteps}
          isLast={isLast}
          onNext={onNext}
          onPrev={onPrev}
          onSkip={onSkip}
        />
      </CardContent>
    </Card>
  )
}

function CenteredDemoCard({ step, stepIndex, totalSteps, onNext, onPrev, onSkip }: DemoCardProps) {
  const isLast = stepIndex === totalSteps - 1

  return (
    <Card className="fixed z-[101] w-80 shadow-lg animate-in fade-in-0 zoom-in-95 duration-200 top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2">
      <CardContent className="p-4">
        <DemoCardContent
          step={step}
          stepIndex={stepIndex}
          totalSteps={totalSteps}
          isLast={isLast}
          onNext={onNext}
          onPrev={onPrev}
          onSkip={onSkip}
        />
      </CardContent>
    </Card>
  )
}

interface DemoCardContentProps {
  step: DemoTourStep
  stepIndex: number
  totalSteps: number
  isLast: boolean
  onNext: () => void
  onPrev: () => void
  onSkip: () => void
}

function DemoCardContent({ step, stepIndex, totalSteps, isLast, onNext, onPrev, onSkip }: DemoCardContentProps) {
  return (
    <>
      <div className="flex items-center justify-between mb-1">
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">
            {stepIndex + 1} / {totalSteps}
          </span>
          {step.sectionLabel && (
            <span
              className="text-[10px] px-1.5 py-0.5 rounded-full font-semibold"
              style={{ backgroundColor: 'hsl(var(--brand) / 0.12)', color: 'hsl(var(--brand))' }}
            >
              {step.sectionLabel}
            </span>
          )}
        </div>
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
    </>
  )
}

// ---------- AI Intro Card ----------

const AI_FLOW_NODES = [
  {
    icon: MessageSquare,
    label: '会話ログ',
    desc: 'あなたとの対話を蓄積',
    color: 'text-blue-500',
    bg: 'bg-blue-500/10',
  },
  {
    icon: RefreshCw,
    label: 'フィードバック',
    desc: '評価から好みや傾向を学習',
    color: 'text-violet-500',
    bg: 'bg-violet-500/10',
  },
  {
    icon: Sparkles,
    label: '自律学習・最適化',
    desc: '継続的にトレーニング',
    color: 'brand',
    bg: 'brand',
  },
]

function AIIntroCard({ step, stepIndex, totalSteps, onNext, onPrev, onSkip }: DemoCardProps) {
  const isLast = stepIndex === totalSteps - 1
  const [visible, setVisible] = useState(false)

  useEffect(() => {
    requestAnimationFrame(() => setVisible(true))
  }, [])

  return (
    <Card className="fixed z-[101] w-[380px] shadow-2xl animate-in fade-in-0 zoom-in-95 duration-200 top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2">
      <CardContent className="p-5">
        {/* Header */}
        <div className="flex items-center justify-between mb-1">
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground">
              {stepIndex + 1} / {totalSteps}
            </span>
            {step.sectionLabel && (
              <span
                className="text-[10px] px-1.5 py-0.5 rounded-full font-semibold"
                style={{ backgroundColor: 'hsl(var(--brand) / 0.12)', color: 'hsl(var(--brand))' }}
              >
                {step.sectionLabel}
              </span>
            )}
          </div>
          <button
            onClick={onSkip}
            className="text-xs text-muted-foreground hover:text-foreground"
          >
            スキップ
          </button>
        </div>
        <h4 className="font-semibold text-base mb-1">{step.title}</h4>
        <p className="text-sm text-muted-foreground mb-4">{step.description}</p>

        {/* Flow Diagram */}
        <div className="flex flex-col items-center gap-0 mb-4">
          {AI_FLOW_NODES.map((node, i) => (
            <div key={i} className="contents">
              {/* Node */}
              <div
                className={cn(
                  'flex items-center gap-3 w-full rounded-lg border p-3 transition-all duration-500',
                  visible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-2',
                  node.color === 'brand' ? '' : 'bg-muted/50',
                )}
                style={{
                  transitionDelay: `${i * 120}ms`,
                  ...(node.color === 'brand' ? {
                    backgroundColor: 'hsl(var(--brand) / 0.08)',
                    borderColor: 'hsl(var(--brand) / 0.3)',
                  } : {}),
                }}
              >
                <div
                  className={cn('rounded-full p-2 shrink-0', node.color !== 'brand' && node.bg)}
                  style={node.color === 'brand' ? { backgroundColor: 'hsl(var(--brand) / 0.15)' } : {}}
                >
                  <node.icon
                    className={cn('h-4 w-4', node.color !== 'brand' && node.color)}
                    style={node.color === 'brand' ? { color: 'hsl(var(--brand))' } : {}}
                  />
                </div>
                <div>
                  <p className="text-xs font-semibold">{node.label}</p>
                  <p className="text-[11px] text-muted-foreground leading-snug">{node.desc}</p>
                </div>
              </div>
              {/* Arrow between nodes */}
              {i < AI_FLOW_NODES.length - 1 && (
                <div
                  className={cn(
                    'flex items-center justify-center h-5 transition-all duration-500',
                    visible ? 'opacity-100' : 'opacity-0',
                  )}
                  style={{ transitionDelay: `${i * 120 + 60}ms` }}
                >
                  <ChevronDown className="h-4 w-4 text-muted-foreground/60" />
                </div>
              )}
            </div>
          ))}

          {/* Result arrow */}
          <div
            className={cn(
              'flex items-center justify-center h-5 transition-all duration-500',
              visible ? 'opacity-100' : 'opacity-0',
            )}
            style={{ transitionDelay: `${AI_FLOW_NODES.length * 120}ms` }}
          >
            <ChevronDown className="h-4 w-4 text-muted-foreground/60" />
          </div>

          {/* Output: personalized AI */}
          <div
            className={cn(
              'w-full rounded-lg border-2 p-3 text-center transition-all duration-500',
              visible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-2',
            )}
            style={{
              transitionDelay: `${AI_FLOW_NODES.length * 120 + 60}ms`,
              borderColor: 'hsl(var(--brand) / 0.5)',
              background: 'linear-gradient(135deg, hsl(var(--brand) / 0.06) 0%, hsl(var(--brand) / 0.12) 100%)',
            }}
          >
            <p className="text-sm font-bold" style={{ color: 'hsl(var(--brand))' }}>
              あなただけのAIアシスタント
            </p>
            <p className="text-[11px] text-muted-foreground mt-0.5">
              パーソナライズされた提案・分析・レビュー
            </p>
          </div>
        </div>

        {/* Navigation */}
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
  placement: 'top' | 'bottom' | 'left' | 'right'
): React.CSSProperties {
  const GAP = 12
  const CARD_WIDTH = 320

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
