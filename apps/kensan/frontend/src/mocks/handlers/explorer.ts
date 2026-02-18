// MSW handlers for AI Explorer (kensan-ai /explorer/interactions)
import { http, HttpResponse } from 'msw'

const AI_BASE = 'http://localhost:8089/api/v1'

function generateMockInteraction(index: number, baseTime: Date) {
  const traceId = `trace-${Date.now()}-${index}`
  const timestamp = new Date(baseTime.getTime() - index * 5 * 60 * 1000)
  const outcomes = ['success', 'success', 'success', 'error', 'max_turns_reached']
  const contexts = ['チャット', 'デイリーアドバイス', 'レビュー']
  const models = ['gemini-2.0-flash', 'claude-sonnet-4-20250514']

  const outcome = outcomes[index % outcomes.length]
  const totalTurns = Math.floor(Math.random() * 5) + 1
  const inputTokens = Math.floor(Math.random() * 3000) + 500
  const outputTokens = Math.floor(Math.random() * 1500) + 200

  return {
    traceId,
    timestamp: timestamp.toISOString(),
    outcome,
    model: models[index % models.length],
    totalTurns,
    totalInputTokens: inputTokens,
    totalOutputTokens: outputTokens,
    pendingActionCount: outcome === 'success' ? Math.floor(Math.random() * 3) : 0,
    userMessage: ['今日のタスクを教えて', '週次レビューをお願い', 'GCPの勉強計画を立てて', '集中力が上がらない'][index % 4],
    contextId: `ctx-${index % 3}`,
    contextName: contexts[index % contexts.length],
    contextVersion: '1',
    experimentId: '',
    systemPromptLength: Math.floor(Math.random() * 2000) + 500,
    systemPromptSections: { '基本設定': 500, 'ツール定義': 800 },
    toolCount: Math.floor(Math.random() * 5) + 1,
    toolNames: ['get_tasks', 'search_notes', 'get_time_entries'].slice(0, Math.floor(Math.random() * 3) + 1),
    toolDefinitionsLength: Math.floor(Math.random() * 3000) + 500,
    events: [
      {
        event: 'agent.prompt',
        traceId,
        timestamp: timestamp.toISOString(),
        model: models[index % models.length],
        user_message: 'hello',
      },
      {
        event: 'agent.complete',
        traceId,
        timestamp: new Date(timestamp.getTime() + 3000).toISOString(),
        outcome,
        total_turns: totalTurns,
        total_input_tokens: inputTokens,
        total_output_tokens: outputTokens,
      },
    ],
  }
}

export const explorerHandlers = [
  http.get(`${AI_BASE}/explorer/interactions`, ({ request }) => {
    const url = new URL(request.url)
    const endTs = url.searchParams.get('end_timestamp')

    const end = endTs ? new Date(endTs) : new Date()
    const count = Math.floor(Math.random() * 8) + 3

    const interactions = Array.from({ length: count }, (_, i) =>
      generateMockInteraction(i, end),
    )

    return HttpResponse.json({ interactions })
  }),
]
