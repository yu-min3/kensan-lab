import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuthStore } from '@/stores/useAuthStore'
import { Loader2, AlertCircle, Star } from 'lucide-react'

type Mode = 'login' | 'register'

interface PersonaInfo {
  id: string
  name: string
  role: string
  description: string
  color: string
  recommended?: boolean
}

const PERSONAS: PersonaInfo[] = [
  {
    id: 'tanaka',
    name: '田中翔太',
    role: 'バックエンドエンジニア',
    description: '30歳・Go+Google Cloud 5年目。AIが学習パターンをある程度把握',
    color: 'bg-blue-500',
    recommended: true,
  },
  {
    id: 'misaki',
    name: '鈴木美咲',
    role: 'フロントエンドエンジニア',
    description: '25歳・個人開発と技術ブログに注力',
    color: 'bg-pink-500',
  },
  {
    id: 'takuya',
    name: '山田拓也',
    role: 'SIer ジュニア',
    description: '23歳・2年目、応用情報技術者を目指す',
    color: 'bg-green-500',
  },
  {
    id: 'aya',
    name: '高橋彩',
    role: 'エンジニアリングマネージャー',
    description: '35歳・IC からマネジメントへ転身中',
    color: 'bg-purple-500',
  },
]

export function LoginPage() {
  const navigate = useNavigate()
  const { login, demoLogin, register, isLoading, error, clearError } = useAuthStore()

  const [mode, setMode] = useState<Mode>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [name, setName] = useState('')
  const [loadingPersona, setLoadingPersona] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    clearError()

    try {
      if (mode === 'login') {
        await login(email, password)
      } else {
        await register(email, password, name)
      }
      navigate('/')
    } catch {
      // Error is already set in the store
    }
  }

  const handleDemoLogin = async (personaId: string) => {
    clearError()
    setLoadingPersona(personaId)
    try {
      await demoLogin(personaId)
      navigate('/')
    } catch {
      setLoadingPersona(null)
    }
  }

  const toggleMode = () => {
    setMode(mode === 'login' ? 'register' : 'login')
    clearError()
  }

  return (
    <div className="min-h-screen bg-muted/30 flex items-center justify-center p-4">
      <div className="w-full max-w-lg space-y-6">
        <div className="text-center">
          <h1 className="text-3xl font-bold">Kensan</h1>
          <p className="text-muted-foreground mt-1">自己研鑽プラットフォーム</p>
        </div>

        {/* Demo Persona Cards */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">デモアカウントで体験</CardTitle>
            <CardDescription>ペルソナを選択すると、サンプルデータ付きでログインできます</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 gap-3">
              {PERSONAS.map((persona) => {
                const isThisLoading = loadingPersona === persona.id
                const isDisabled = isLoading || loadingPersona !== null
                return (
                  <button
                    key={persona.id}
                    onClick={() => handleDemoLogin(persona.id)}
                    disabled={isDisabled}
                    className={`relative flex items-start gap-3 p-3 rounded-lg border transition-colors text-left disabled:opacity-50 disabled:cursor-not-allowed ${persona.recommended ? 'border-primary/50 bg-primary/5 hover:bg-primary/10 ring-1 ring-primary/20' : 'border-border hover:border-primary/50 hover:bg-accent/50'}`}
                  >
                    {persona.recommended && (
                      <span className="absolute -top-2.5 right-2 flex items-center gap-0.5 rounded-full bg-primary px-2 py-0.5 text-[10px] font-medium text-primary-foreground">
                        <Star className="h-2.5 w-2.5 fill-current" />
                        おすすめ
                      </span>
                    )}
                    <div className={`${persona.color} w-9 h-9 rounded-full flex items-center justify-center text-white font-bold text-sm shrink-0 mt-0.5`}>
                      {isThisLoading ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        persona.name.charAt(0)
                      )}
                    </div>
                    <div className="min-w-0">
                      <div className="font-medium text-sm">{persona.name}</div>
                      <div className="text-xs text-muted-foreground">{persona.role}</div>
                      <div className="text-xs text-muted-foreground/70 mt-0.5">{persona.description}</div>
                    </div>
                  </button>
                )
              })}
            </div>

            {error && loadingPersona !== null && (
              <div className="flex items-center gap-2 text-red-600 text-sm p-3 bg-red-50 dark:bg-red-950/20 rounded-lg mt-3">
                <AlertCircle className="h-4 w-4 flex-shrink-0" />
                <span>{error}</span>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Divider */}
        <div className="relative">
          <div className="absolute inset-0 flex items-center">
            <span className="w-full border-t" />
          </div>
          <div className="relative flex justify-center text-xs uppercase">
            <span className="bg-muted/30 px-2 text-muted-foreground">または</span>
          </div>
        </div>

        {/* Login / Register Form */}
        <Card>
          <CardHeader>
            <CardTitle>{mode === 'login' ? 'ログイン' : 'アカウント登録'}</CardTitle>
            <CardDescription>
              {mode === 'login'
                ? 'メールアドレスとパスワードでログイン'
                : '新しいアカウントを作成します'}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              {mode === 'register' && (
                <div className="space-y-2">
                  <Label htmlFor="name">名前</Label>
                  <Input
                    id="name"
                    type="text"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="山田 太郎"
                    required
                  />
                </div>
              )}

              <div className="space-y-2">
                <Label htmlFor="email">メールアドレス</Label>
                <Input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="you@example.com"
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="password">パスワード</Label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="********"
                  required
                  minLength={8}
                />
                {mode === 'register' && (
                  <p className="text-xs text-muted-foreground">
                    8文字以上で入力してください
                  </p>
                )}
              </div>

              {error && loadingPersona === null && (
                <div className="flex items-center gap-2 text-red-600 text-sm p-3 bg-red-50 dark:bg-red-950/20 rounded-lg">
                  <AlertCircle className="h-4 w-4 flex-shrink-0" />
                  <span>{error}</span>
                </div>
              )}

              <Button type="submit" className="w-full" disabled={isLoading}>
                {isLoading && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                {mode === 'login' ? 'ログイン' : '登録'}
              </Button>
            </form>

            <div className="mt-4 text-center text-sm">
              <span className="text-muted-foreground">
                {mode === 'login' ? 'アカウントをお持ちでない方は' : 'すでにアカウントをお持ちの方は'}
              </span>
              <button
                type="button"
                onClick={toggleMode}
                className="ml-1 text-primary hover:underline"
              >
                {mode === 'login' ? '新規登録' : 'ログイン'}
              </button>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
