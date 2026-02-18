import {
  BookOpen,
  TrendingUp,
  CalendarDays,
  Target,
} from 'lucide-react'

interface SuggestionItem {
  icon: React.ReactNode
  label: string
  message: string
}

const suggestions: SuggestionItem[] = [
  {
    icon: <TrendingUp className="h-4 w-4 shrink-0" />,
    label: '最近の傾向を分析して',
    message: '最近の行動パターンや感情の傾向を分析して、気になる点があれば率直に教えて',
  },
  {
    icon: <CalendarDays className="h-4 w-4 shrink-0" />,
    label: '今週の予定を立てて',
    message: '今週のスケジュールを立てて。タスクの優先度と生産性のピーク時間を考慮して',
  },
  {
    icon: <Target className="h-4 w-4 shrink-0" />,
    label: '目標の進め方にフィードバック',
    message: '目標の進捗状況を見て、進め方に率直なフィードバックをちょうだい',
  },
  {
    icon: <BookOpen className="h-4 w-4 shrink-0" />,
    label: '最近の学習記録を振り返って',
    message: '最近の学習記録を振り返って、何が身についたか、目標とのつながりを教えて',
  },
]

interface WelcomeMessageProps {
  onSend?: (message: string) => void
}

export function WelcomeMessage({ onSend }: WelcomeMessageProps) {
  return (
    <div className="flex flex-col h-full justify-center px-4 py-6 overflow-y-auto">
      {onSend && (
        <div className="flex flex-col gap-2">
          {suggestions.map((s) => (
            <button
              key={s.label}
              onClick={() => onSend(s.message)}
              className="flex items-center gap-2.5 px-3 py-2.5 text-left text-sm rounded-lg border border-border/60 bg-muted/30 hover:bg-muted/60 hover:border-border transition-colors"
            >
              <span className="text-muted-foreground">{s.icon}</span>
              <span>{s.label}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
