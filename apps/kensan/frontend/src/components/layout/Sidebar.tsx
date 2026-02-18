import { useEffect, useMemo } from 'react'
import { NavLink } from 'react-router-dom'
import {
  CalendarDays,
  CalendarRange,
  StickyNote,
  FolderKanban,
  BarChart3,
  Activity,
  FileCode2,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/useAuthStore'
import { usePromptStore } from '@/stores/usePromptStore'

interface NavItem {
  to: string
  icon: React.ComponentType<{ className?: string }>
  label: string
  badge?: string
  guideId?: string
  notifyCount?: number
}

interface NavSection {
  label?: string
  items: NavItem[]
}

const navSections: NavSection[] = [
  {
    items: [
      { to: '/', icon: CalendarDays, label: 'Daily', guideId: 'sidebar-daily' },
      { to: '/weekly', icon: CalendarRange, label: 'Weekly' },
    ],
  },
  {
    label: '記録・管理',
    items: [
      { to: '/notes', icon: StickyNote, label: 'ノート' },
      { to: '/tasks', icon: FolderKanban, label: 'タスク管理', guideId: 'sidebar-tasks' },
      { to: '/analytics', icon: BarChart3, label: '分析・レポート', guideId: 'sidebar-analytics' },
    ],
  },
  {
    label: 'AI',
    items: [
      { to: '/interactions', icon: Activity, label: 'AI Explorer' },
      { to: '/prompts', icon: FileCode2, label: 'プロンプト管理' },
    ],
  },
]

function NavItemLink({ item }: { item: NavItem }) {
  return (
    <NavLink
      to={item.to}
      end={item.to === '/'}
      data-guide={item.guideId}
      className={({ isActive }) =>
        cn(
          'flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors relative',
          isActive
            ? 'bg-brand/15 dark:bg-brand/20 text-brand dark:text-brand font-medium before:absolute before:left-0 before:top-1 before:bottom-1 before:w-1 before:bg-brand/60 before:rounded-full'
            : 'text-muted-foreground hover:bg-muted hover:text-foreground'
        )
      }
    >
      <item.icon className="h-4 w-4" />
      <span className="flex-1">{item.label}</span>
      {item.badge && (
        <span className="text-[10px] px-1.5 py-0.5 rounded bg-muted-foreground/20 text-muted-foreground">
          {item.badge}
        </span>
      )}
      {item.notifyCount != null && item.notifyCount > 0 && (
        <span className="flex h-4 min-w-4 items-center justify-center rounded-full bg-brand px-1 text-[10px] font-medium text-white">
          {item.notifyCount}
        </span>
      )}
    </NavLink>
  )
}

export function Sidebar() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const { contexts, fetchContexts } = usePromptStore()

  useEffect(() => {
    if (isAuthenticated) {
      fetchContexts()
    }
  }, [isAuthenticated, fetchContexts])

  const pendingCandidateCount = useMemo(
    () => contexts.reduce((sum, ctx) => sum + (ctx.pending_candidate_count ?? 0), 0),
    [contexts],
  )

  // Inject dynamic notification counts into static nav config
  const sections = navSections.map((section) => ({
    ...section,
    items: section.items.map((item) =>
      item.to === '/prompts' ? { ...item, notifyCount: pendingCandidateCount } : item,
    ),
  }))

  return (
    <aside className="w-60 border-r bg-muted/40 h-full">
      <nav role="navigation" aria-label="Main navigation" className="flex flex-col gap-4 p-4">
        {sections.map((section, i) => (
          <div key={i} className="flex flex-col gap-1">
            {section.label && (
              <span className="px-3 pb-1 text-[11px] font-medium uppercase tracking-wider text-muted-foreground/60">
                {section.label}
              </span>
            )}
            {section.items.map((item) => (
              <NavItemLink key={item.to} item={item} />
            ))}
          </div>
        ))}
      </nav>
    </aside>
  )
}
