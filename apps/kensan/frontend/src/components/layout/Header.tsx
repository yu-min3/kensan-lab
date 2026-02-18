import { Settings, Sun, Moon, LogOut, Sparkles } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { TimerWidget } from '@/components/common/TimerWidget'
import { KensanLogo } from '@/components/common/KensanLogo'
import { useAuthStore } from '@/stores/useAuthStore'
import { useSettingsStore } from '@/stores/useSettingsStore'
import { useTheme } from '@/hooks/useTheme'
import { useChatStore } from '@/stores/useChatStore'

export function Header() {
  const navigate = useNavigate()
  const { logout, user } = useAuthStore()
  const { userName } = useSettingsStore()
  const { theme, resolvedTheme, toggleTheme } = useTheme()

  const displayName = user?.name || userName

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  return (
    <header className="h-14 border-b bg-background px-4 flex items-center justify-between">
      <div className="flex items-center gap-4">
        <Link to="/" className="flex items-center gap-2">
          <KensanLogo size={28} />
          <span className="text-xl font-bold text-primary">Kensan</span>
        </Link>
      </div>

      <div className="flex items-center gap-2">
        <div data-guide="header-timer">
          <TimerWidget />
        </div>

        <div className="h-6 w-px bg-border mx-1" />

        <Button variant="ghost" size="icon" onClick={toggleTheme} title={`テーマ: ${theme}`}>
          {resolvedTheme === 'dark' ? (
            <Sun className="h-5 w-5" />
          ) : (
            <Moon className="h-5 w-5" />
          )}
        </Button>

        <button
          data-guide="header-ai-button"
          className="inline-flex items-center gap-1.5 px-3 h-8 text-sm font-medium rounded-md transition-opacity hover:opacity-90 cursor-pointer"
          style={{
            backgroundColor: 'hsl(var(--brand))',
            color: 'hsl(var(--brand-foreground))',
          }}
          onClick={() => useChatStore.getState().toggle()}
          title="AI Assistant"
        >
          <Sparkles className="h-4 w-4" />
          AI
        </button>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button className="flex items-center gap-2 ml-2 pl-2 border-l cursor-pointer hover:opacity-80">
              <div className="w-8 h-8 rounded-full bg-primary flex items-center justify-center text-primary-foreground text-sm font-medium">
                {displayName.charAt(0).toUpperCase()}
              </div>
              <span className="text-sm font-medium">{displayName}</span>
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-48">
            <DropdownMenuItem asChild>
              <Link to="/settings" className="cursor-pointer">
                <Settings className="h-4 w-4 mr-2" />
                設定
              </Link>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={handleLogout} className="cursor-pointer text-red-600">
              <LogOut className="h-4 w-4 mr-2" />
              ログアウト
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  )
}
