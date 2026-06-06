import { useNavigate } from 'react-router-dom'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useSettingsStore } from '@/stores/useSettingsStore'
import { PageGuide } from '@/components/guide/PageGuide'
import { useGuideStore } from '@/components/guide/useGuideStore'
import { RotateCcw } from 'lucide-react'

export function S01Settings() {
  const navigate = useNavigate()
  const {
    timezone,
    theme,
    isConfigured,
    setTimezone,
    setTheme,
    setIsConfigured,
    saveSettings,
  } = useSettingsStore()
  const resetGuide = useGuideStore((s) => s.resetAll)

  const handleSave = async () => {
    setIsConfigured(true)
    await saveSettings()
    navigate('/')
  }

  return (
    <div className="min-h-screen bg-muted/30 flex items-center justify-center p-4">
      <div className="w-full max-w-lg space-y-6">
        <div className="text-center">
          <h1 className="text-3xl font-bold">Kensan</h1>
          <p className="text-muted-foreground mt-1">自己研鑽プラットフォーム</p>
        </div>

        <PageGuide pageId="settings" />

        <Card>
          <CardHeader>
            <CardTitle>設定</CardTitle>
            <CardDescription>
              アプリケーションの基本設定を行います
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {/* Timezone */}
            <div className="space-y-2">
              <Label htmlFor="timezone">タイムゾーン</Label>
              <Select value={timezone} onValueChange={setTimezone}>
                <SelectTrigger>
                  <SelectValue placeholder="タイムゾーンを選択" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="Asia/Tokyo">Asia/Tokyo (JST)</SelectItem>
                  <SelectItem value="UTC">UTC</SelectItem>
                  <SelectItem value="America/New_York">America/New_York (EST)</SelectItem>
                  <SelectItem value="Europe/London">Europe/London (GMT)</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {/* Theme */}
            <div className="space-y-2">
              <Label htmlFor="theme">テーマ</Label>
              <Select value={theme} onValueChange={(v) => setTheme(v as 'light' | 'dark' | 'system')}>
                <SelectTrigger>
                  <SelectValue placeholder="テーマを選択" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="system">システム設定に従う</SelectItem>
                  <SelectItem value="light">ライト</SelectItem>
                  <SelectItem value="dark">ダーク</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <Button
              className="w-full"
              onClick={handleSave}
            >
              設定を保存{isConfigured ? '' : 'して始める'}
            </Button>

            {isConfigured && (
              <Button
                variant="outline"
                className="w-full gap-2"
                onClick={resetGuide}
              >
                <RotateCcw className="h-4 w-4" />
                ページガイドをリセット
              </Button>
            )}

            {isConfigured && (
              <Button
                variant="ghost"
                className="w-full"
                onClick={() => navigate('/')}
              >
                キャンセル
              </Button>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
