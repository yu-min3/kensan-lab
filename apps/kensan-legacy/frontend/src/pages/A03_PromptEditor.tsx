import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { Card, CardContent } from '@/components/ui/card'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Loader2, FileCode2, Wand2 } from 'lucide-react'
import { toast } from 'sonner'
import { Select, SelectContent, SelectItem, SelectTrigger } from '@/components/ui/select'
import { usePromptStore } from '@/stores/usePromptStore'
import { PromptSidebar } from '@/components/prompt/PromptSidebar'
import { PromptEditor } from '@/components/prompt/PromptEditor'
import { VersionHistory } from '@/components/prompt/VersionHistory'
import { VersionDetail } from '@/components/prompt/VersionDetail'
import { ABTestPanel } from '@/components/prompt/ABTestPanel'
import { PageGuide } from '@/components/guide/PageGuide'
import {
  adoptVersion,
  rejectVersion,
  runOptimization,
  type AIContextVersion,
  type AIContextUpdateInput,
} from '@/api/services/prompts'

export function A03PromptEditor() {
  const {
    contexts,
    versions,
    isLoading,
    fetchContexts,
    updateContext,
    fetchVersions,
    rollback,
  } = usePromptStore()

  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState('editor')
  const [selectedVersion, setSelectedVersion] = useState<AIContextVersion | null>(null)
  const [abTestMode, setAbTestMode] = useState<{ versionA: number; versionB: number } | null>(null)
  const [isOptimizing, setIsOptimizing] = useState(false)
  const [optimizeElapsed, setOptimizeElapsed] = useState(0)
  const optimizeTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // Elapsed timer for optimization
  useEffect(() => {
    if (isOptimizing) {
      setOptimizeElapsed(0)
      optimizeTimerRef.current = setInterval(() => setOptimizeElapsed((s) => s + 1), 1000)
    } else {
      if (optimizeTimerRef.current) {
        clearInterval(optimizeTimerRef.current)
        optimizeTimerRef.current = null
      }
    }
    return () => {
      if (optimizeTimerRef.current) clearInterval(optimizeTimerRef.current)
    }
  }, [isOptimizing])

  useEffect(() => {
    fetchContexts()
  }, [fetchContexts])

  // Auto-select first context when loaded
  useEffect(() => {
    if (contexts.length > 0 && !selectedId) {
      setSelectedId(contexts[0].id)
    }
  }, [contexts, selectedId])

  // Fetch versions when selection changes
  useEffect(() => {
    if (selectedId) {
      fetchVersions(selectedId)
    }
  }, [selectedId, fetchVersions])

  // Fetch versions when switching to optimize tab
  useEffect(() => {
    if (activeTab === 'optimize' && selectedId) {
      fetchVersions(selectedId)
    }
  }, [activeTab, selectedId, fetchVersions])

  const selectedContext = contexts.find((c) => c.id === selectedId)
  const selectedVersions = selectedId ? (versions[selectedId] ?? []) : []

  const pendingCandidateCount = useMemo(
    () => contexts.reduce((sum, ctx) => sum + (ctx.pending_candidate_count ?? 0), 0),
    [contexts],
  )

  // Get the active version's prompt for diff display
  const activeVersionPrompt = useMemo(() => {
    if (!selectedContext?.active_version) return null
    const v = selectedVersions.find((v) => v.version_number === selectedContext.active_version)
    return v?.system_prompt ?? null
  }, [selectedContext?.active_version, selectedVersions])

  const handleSave = async (data: AIContextUpdateInput) => {
    if (!selectedId) return
    await updateContext(selectedId, data)
    await fetchVersions(selectedId)
  }

  const handleRollback = async (versionNumber: number) => {
    if (!selectedId) return
    await rollback(selectedId, versionNumber)
    setSelectedVersion(null)
  }

  const handleAdopt = useCallback(async (versionNumber: number) => {
    if (!selectedId) return
    await adoptVersion(selectedId, versionNumber)
    await fetchContexts()
    await fetchVersions(selectedId)
    setSelectedVersion(null)
    toast.success(`v${versionNumber} を採用しました`)
  }, [selectedId, fetchContexts, fetchVersions])

  const handleReject = useCallback(async (versionNumber: number) => {
    if (!selectedId) return
    await rejectVersion(selectedId, versionNumber)
    await fetchContexts()
    await fetchVersions(selectedId)
    setSelectedVersion(null)
    toast.success(`v${versionNumber} を却下しました`)
  }, [selectedId, fetchContexts, fetchVersions])

  const handleRunOptimization = useCallback(async () => {
    setIsOptimizing(true)
    try {
      const result = await runOptimization(true)
      if (result.candidates_created > 0) {
        toast.success(`${result.candidates_created}件の最適化提案を生成しました`)
        await fetchContexts()
        if (selectedId) await fetchVersions(selectedId)
      } else {
        toast.info('改善が必要なコンテキストはありませんでした')
      }
    } catch {
      toast.error('最適化の実行に失敗しました')
    } finally {
      setIsOptimizing(false)
    }
  }, [fetchContexts, fetchVersions, selectedId])

  const handleStartABTest = useCallback((versionNumber: number) => {
    if (!selectedContext?.active_version) return
    setAbTestMode({ versionA: selectedContext.active_version, versionB: versionNumber })
    setSelectedVersion(null)
  }, [selectedContext?.active_version])

  const handleABAdoptVersion = useCallback(async (versionNumber: number) => {
    if (!selectedId) return
    // If it's the active version, nothing to do
    if (versionNumber === selectedContext?.active_version) {
      toast.info('現在のバージョンがそのまま維持されます')
      setAbTestMode(null)
      return
    }
    // Check if the version is a pending candidate
    const version = selectedVersions.find((v) => v.version_number === versionNumber)
    if (version?.candidate_status === 'pending') {
      await adoptVersion(selectedId, versionNumber)
    } else {
      await rollback(selectedId, versionNumber)
    }
    await fetchContexts()
    await fetchVersions(selectedId)
    setAbTestMode(null)
    toast.success(`v${versionNumber} を採用しました`)
  }, [selectedId, selectedContext?.active_version, selectedVersions, fetchContexts, fetchVersions, rollback])

  if (isLoading && contexts.length === 0) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <PageGuide pageId="prompts" />

      {/* Header */}
      <div className="space-y-2">
        <div className="flex items-center gap-3">
          <FileCode2 className="h-5 w-5 text-brand" />
          <h1 className="text-xl font-bold">プロンプト管理</h1>
        </div>
        <p className="text-sm text-muted-foreground">
          AIエージェントの振る舞いを制御するシステムプロンプトの管理画面です。
          コンテキストを選択 → プロンプトを編集 → 変更メモを入力して保存。
          変更は自動でバージョン管理され、履歴の確認とロールバックが可能です。
          persona は全エージェント共通設定として自動適用されます。
        </p>
      </div>

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList data-guide="prompt-tabs">
          <TabsTrigger value="editor">プロンプト編集</TabsTrigger>
          <TabsTrigger value="optimize" className="gap-1.5" data-guide="prompt-tab-optimize">
            最適化
            {pendingCandidateCount > 0 && (
              <Badge variant="secondary" className="ml-1 h-5 min-w-5 px-1.5 text-[10px]">
                {pendingCandidateCount}
              </Badge>
            )}
          </TabsTrigger>
        </TabsList>

        {/* Editor Tab */}
        <TabsContent value="editor">
          <div className="grid gap-4 lg:grid-cols-3">
            {/* Left: Context list */}
            <Card data-guide="prompt-sidebar" className="lg:col-span-1 lg:self-start">
              <CardContent className="p-3">
                <PromptSidebar
                  contexts={contexts}
                  selectedId={selectedId}
                  onSelect={setSelectedId}
                />
              </CardContent>
            </Card>

            {/* Right: Editor */}
            <div className="space-y-4 lg:col-span-2">
              {selectedContext ? (
                <Card data-guide="prompt-editor">
                  <CardContent className="p-5">
                    <PromptEditor
                      context={selectedContext}
                      isLoading={isLoading}
                      onSave={handleSave}
                    />
                  </CardContent>
                </Card>
              ) : (
                <Card>
                  <CardContent className="flex items-center justify-center py-20">
                    <p className="text-sm text-muted-foreground">
                      左のリストからコンテキストを選択してください
                    </p>
                  </CardContent>
                </Card>
              )}
            </div>
          </div>
        </TabsContent>

        {/* Optimization Tab */}
        <TabsContent value="optimize">
          <div className="grid gap-4 lg:grid-cols-3">
            {/* Left: Optimize button + version history */}
            <Card className="lg:col-span-1">
              <CardContent className="p-3 space-y-3">
                {/* AI Optimize trigger */}
                <div className="space-y-1.5">
                  <Button
                    variant="outline"
                    size="sm"
                    className="w-full gap-1.5"
                    disabled={isOptimizing}
                    onClick={handleRunOptimization}
                  >
                    {isOptimizing ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Wand2 className="h-3.5 w-3.5" />
                    )}
                    {isOptimizing
                      ? `実行中... (${optimizeElapsed}秒)`
                      : 'AI最適化を実行'}
                  </Button>
                  <p className="text-center text-[10px] text-muted-foreground">
                    {isOptimizing
                      ? 'コンテキストの評価・改善プロンプト生成中です'
                      : 'コンテキスト数に応じて1〜2分程度かかります'}
                  </p>
                </div>

                {/* Context selector + version history */}
                <div className="border-t pt-3">
                  <Select value={selectedId ?? ''} onValueChange={(v) => { setSelectedId(v); setSelectedVersion(null); setAbTestMode(null) }}>
                    <SelectTrigger className="mb-2 h-8 text-xs">
                      <span className="truncate">
                        {selectedContext
                          ? `${selectedContext.name}${selectedContext.active_version ? ` v${selectedContext.active_version}` : ''}`
                          : 'コンテキストを選択'}
                      </span>
                    </SelectTrigger>
                    <SelectContent>
                      {contexts.map((ctx) => (
                        <SelectItem key={ctx.id} value={ctx.id} className="text-xs">
                          <span className="flex items-center gap-1.5">
                            {ctx.name}{ctx.active_version ? ` v${ctx.active_version}` : ''}
                            {ctx.pending_candidate_count > 0 && (
                              <Badge variant="outline" className="text-[9px] px-1 py-0 bg-yellow-500/15 text-yellow-700 dark:text-yellow-400 border-yellow-500/30">
                                {ctx.pending_candidate_count}
                              </Badge>
                            )}
                          </span>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  {selectedContext ? (
                    <VersionHistory
                      versions={selectedVersions}
                      activeVersion={selectedContext.active_version}
                      isLoading={isLoading}
                      onSelectVersion={setSelectedVersion}
                      onRollback={handleRollback}
                      onStartABTest={handleStartABTest}
                      selectedVersionNumber={selectedVersion?.version_number ?? null}
                    />
                  ) : (
                    <p className="py-4 text-center text-xs text-muted-foreground">
                      コンテキストを選択してください
                    </p>
                  )}
                </div>
              </CardContent>
            </Card>

            {/* Right: Detail panel */}
            <div className="lg:col-span-2" data-guide="version-detail">
              {abTestMode && selectedId ? (
                <ABTestPanel
                  contextId={selectedId}
                  versionA={abTestMode.versionA}
                  versionB={abTestMode.versionB}
                  onClose={() => setAbTestMode(null)}
                  onAdoptVersion={handleABAdoptVersion}
                />
              ) : selectedVersion ? (
                <Card>
                  <CardContent className="p-5">
                    <VersionDetail
                      version={selectedVersion}
                      activeVersion={selectedContext?.active_version ?? null}
                      activePrompt={activeVersionPrompt}
                      onAdopt={handleAdopt}
                      onReject={handleReject}
                      onRollback={handleRollback}
                      onStartABTest={handleStartABTest}
                    />
                  </CardContent>
                </Card>
              ) : (
                <Card>
                  <CardContent className="flex flex-col items-center justify-center gap-2 py-20 text-center">
                    <Wand2 className="h-8 w-8 text-muted-foreground/40" />
                    <p className="text-sm text-muted-foreground">
                      バージョンを選択するか、AI最適化を実行してください
                    </p>
                  </CardContent>
                </Card>
              )}
            </div>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  )
}
