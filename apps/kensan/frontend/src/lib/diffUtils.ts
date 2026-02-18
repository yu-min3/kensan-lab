export interface InlineSegment {
  text: string
  changed: boolean
}

export interface DiffLine {
  type: 'added' | 'removed' | 'unchanged'
  content: string
  inlineChanges?: InlineSegment[]
}

/**
 * Compute word-level inline diff between two lines.
 * Splits on word boundaries and uses LCS to identify changed segments.
 */
function computeInlineDiff(oldLine: string, newLine: string): { oldSegments: InlineSegment[]; newSegments: InlineSegment[] } {
  const splitWords = (s: string): string[] => {
    const result: string[] = []
    let current = ''
    for (const ch of s) {
      if (/\s/.test(ch)) {
        if (current) { result.push(current); current = '' }
        result.push(ch)
      } else {
        current += ch
      }
    }
    if (current) result.push(current)
    return result
  }

  const oldWords = splitWords(oldLine)
  const newWords = splitWords(newLine)

  const m = oldWords.length
  const n = newWords.length
  const dp: number[][] = Array.from({ length: m + 1 }, () => Array(n + 1).fill(0))

  for (let i = 1; i <= m; i++) {
    for (let j = 1; j <= n; j++) {
      if (oldWords[i - 1] === newWords[j - 1]) {
        dp[i][j] = dp[i - 1][j - 1] + 1
      } else {
        dp[i][j] = Math.max(dp[i - 1][j], dp[i][j - 1])
      }
    }
  }

  // Backtrack to build segments
  let i = m
  let j = n
  const oldStack: InlineSegment[] = []
  const newStack: InlineSegment[] = []

  while (i > 0 || j > 0) {
    if (i > 0 && j > 0 && oldWords[i - 1] === newWords[j - 1]) {
      oldStack.push({ text: oldWords[i - 1], changed: false })
      newStack.push({ text: newWords[j - 1], changed: false })
      i--; j--
    } else if (j > 0 && (i === 0 || dp[i][j - 1] >= dp[i - 1][j])) {
      newStack.push({ text: newWords[j - 1], changed: true })
      j--
    } else {
      oldStack.push({ text: oldWords[i - 1], changed: true })
      i--
    }
  }

  oldStack.reverse()
  newStack.reverse()

  // Merge consecutive segments with same changed status
  const merge = (segs: InlineSegment[]): InlineSegment[] => {
    const merged: InlineSegment[] = []
    for (const seg of segs) {
      const last = merged[merged.length - 1]
      if (last && last.changed === seg.changed) {
        last.text += seg.text
      } else {
        merged.push({ ...seg })
      }
    }
    return merged
  }

  return { oldSegments: merge(oldStack), newSegments: merge(newStack) }
}

export function computeLineDiff(oldText: string, newText: string): DiffLine[] {
  const oldLines = oldText.split('\n')
  const newLines = newText.split('\n')

  // LCS-based diff with trimEnd normalization for comparison
  const m = oldLines.length
  const n = newLines.length
  const dp: number[][] = Array.from({ length: m + 1 }, () => Array(n + 1).fill(0))

  for (let i = 1; i <= m; i++) {
    for (let j = 1; j <= n; j++) {
      if (oldLines[i - 1].trimEnd() === newLines[j - 1].trimEnd()) {
        dp[i][j] = dp[i - 1][j - 1] + 1
      } else {
        dp[i][j] = Math.max(dp[i - 1][j], dp[i][j - 1])
      }
    }
  }

  // Backtrack to build diff
  const diffs: DiffLine[] = []
  let i = m
  let j = n
  while (i > 0 || j > 0) {
    if (i > 0 && j > 0 && oldLines[i - 1].trimEnd() === newLines[j - 1].trimEnd()) {
      diffs.push({ type: 'unchanged', content: oldLines[i - 1] })
      i--
      j--
    } else if (j > 0 && (i === 0 || dp[i][j - 1] >= dp[i - 1][j])) {
      diffs.push({ type: 'added', content: newLines[j - 1] })
      j--
    } else {
      diffs.push({ type: 'removed', content: oldLines[i - 1] })
      i--
    }
  }

  const result = diffs.reverse()

  // Post-process: find consecutive removed+added pairs and add inline diff
  for (let k = 0; k < result.length - 1; k++) {
    if (result[k].type === 'removed' && result[k + 1].type === 'added') {
      const { oldSegments, newSegments } = computeInlineDiff(result[k].content, result[k + 1].content)
      // Only add inline diff if there are actual partial changes (not all changed)
      const hasUnchangedOld = oldSegments.some(s => !s.changed)
      const hasUnchangedNew = newSegments.some(s => !s.changed)
      if (hasUnchangedOld && hasUnchangedNew) {
        result[k].inlineChanges = oldSegments
        result[k + 1].inlineChanges = newSegments
      }
    }
  }

  return result
}
