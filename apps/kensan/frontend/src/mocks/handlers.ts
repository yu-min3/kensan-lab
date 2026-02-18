// Aggregate all MSW handlers
import { authHandlers } from './handlers/auth'
import { taskHandlers } from './handlers/tasks'
import { timeblockHandlers } from './handlers/timeblocks'
import { timerHandlers } from './handlers/timer'
import { analyticsHandlers } from './handlers/analytics'
import { memoHandlers } from './handlers/memos'
import { noteHandlers } from './handlers/notes'
import { agentHandlers } from './handlers/agent'
import { explorerHandlers } from './handlers/explorer'

export const handlers = [
  ...authHandlers,
  ...taskHandlers,
  ...timeblockHandlers,
  ...timerHandlers,
  ...analyticsHandlers,
  ...memoHandlers,
  ...noteHandlers,
  ...agentHandlers,
  ...explorerHandlers,
]
