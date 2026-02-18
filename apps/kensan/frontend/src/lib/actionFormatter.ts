/**
 * Format action descriptions for human-readable display.
 * Converts tool_name + input into concise Japanese descriptions.
 */

type Input = Record<string, unknown>

function str(input: Input, key: string): string {
  return (input[key] as string) ?? ''
}

function formatTime(iso: string): string {
  try {
    const d = new Date(iso)
    if (isNaN(d.getTime())) return iso
    const month = d.getMonth() + 1
    const day = d.getDate()
    const hours = String(d.getHours()).padStart(2, '0')
    const minutes = String(d.getMinutes()).padStart(2, '0')
    return `${month}/${day} ${hours}:${minutes}`
  } catch {
    return iso
  }
}

function formatDate(dateStr: string): string {
  try {
    const d = new Date(dateStr + 'T00:00:00')
    if (isNaN(d.getTime())) return dateStr
    return `${d.getMonth() + 1}/${d.getDate()}`
  } catch {
    return dateStr
  }
}

function truncate(text: string, maxLen = 30): string {
  if (text.length <= maxLen) return text
  return text.slice(0, maxLen) + '...'
}

const formatters: Record<string, (input: Input) => string> = {
  create_time_block: (input) => {
    const date = str(input, 'date')
    const startTime = str(input, 'start_time') || str(input, 'start_timestamp')
    const endTime = str(input, 'end_time') || str(input, 'end_timestamp')
    const name = str(input, 'task_name') || str(input, 'name') || str(input, 'title')

    // Handle HH:MM format (from AI service) vs ISO datetime
    const isShortTime = (t: string) => /^\d{1,2}:\d{2}$/.test(t)
    let timeRange = ''
    if (startTime && endTime) {
      if (isShortTime(startTime) && isShortTime(endTime)) {
        const datePrefix = date ? `${formatDate(date)} ` : ''
        timeRange = `${datePrefix}${startTime}-${endTime}`
      } else {
        const startStr = formatTime(startTime)
        const endStr = formatTime(endTime)
        timeRange = `${startStr}-${endStr.split(' ').pop()}`
      }
    }
    return timeRange
      ? `${timeRange} ${name} のタイムブロックを作成`
      : `タイムブロックを作成: ${name}`
  },

  update_time_block: (input) => {
    const name = str(input, 'task_name') || str(input, 'name')
    return name ? `タイムブロックを更新: ${name}` : 'タイムブロックを更新'
  },

  delete_time_block: (input) => {
    const name = str(input, 'task_name') || str(input, 'name')
    const startTime = str(input, 'start_time')
    const endTime = str(input, 'end_time')
    if (name && startTime && endTime) {
      return `タイムブロックを削除: ${startTime}-${endTime} ${name}`
    }
    if (name) {
      return `タイムブロックを削除: ${name}`
    }
    return 'タイムブロックを削除'
  },

  create_task: (input) => {
    const name = str(input, 'name')
    const milestone = str(input, 'milestone_name')
    return milestone ? `タスクを作成: ${name} (${milestone})` : `タスクを作成: ${name}`
  },

  update_task: (input) => {
    const name = str(input, 'name') || str(input, 'task_name')
    const completed = input['completed']
    if (completed !== undefined) {
      const status = completed ? '完了' : '未完了'
      return name ? `タスクを更新: ${name} → ${status}` : `タスクを更新 → ${status}`
    }
    return name ? `タスクを更新: ${name}` : 'タスクを更新'
  },

  delete_task: (input) => {
    const name = str(input, 'name') || str(input, 'task_name')
    return name ? `タスクを削除: ${name}` : 'タスクを削除'
  },

  create_memo: (input) => {
    const content = str(input, 'content')
    return `メモを作成: ${truncate(content)}`
  },

  update_memo: (input) => {
    const content = str(input, 'content')
    return content ? `メモを更新: ${truncate(content)}` : 'メモを更新'
  },

  delete_memo: () => 'メモを削除',

  create_note: (input) => {
    const title = str(input, 'title')
    const noteType = str(input, 'type')
    return noteType ? `ノートを作成: ${title} (${noteType})` : `ノートを作成: ${title}`
  },

  update_note: (input) => {
    const title = str(input, 'title')
    return title ? `ノートを更新: ${title}` : 'ノートを更新'
  },

  delete_note: (input) => {
    const title = str(input, 'title')
    return title ? `ノートを削除: ${title}` : 'ノートを削除'
  },

  create_goal: (input) => {
    const name = str(input, 'name')
    return `目標を作成: ${name}`
  },

  update_goal: (input) => {
    const name = str(input, 'name')
    return name ? `目標を更新: ${name}` : '目標を更新'
  },

  create_milestone: (input) => {
    const name = str(input, 'name')
    return `マイルストーンを作成: ${name}`
  },

  update_milestone: (input) => {
    const name = str(input, 'name')
    return name ? `マイルストーンを更新: ${name}` : 'マイルストーンを更新'
  },

  generate_review: () => 'レビューを生成',

  create_time_entry: (input) => {
    const name = str(input, 'task_name') || str(input, 'name')
    return name ? `時間記録を作成: ${name}` : '時間記録を作成'
  },
}

export function formatActionDescription(
  toolName: string,
  input: Record<string, unknown>
): string {
  const formatter = formatters[toolName]
  if (formatter) {
    try {
      return formatter(input)
    } catch {
      // Fall through to default
    }
  }
  return `${toolName} を実行`
}
