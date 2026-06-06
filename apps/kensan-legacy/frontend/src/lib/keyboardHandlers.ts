import type React from 'react'

export function handleSubmitOrCancel(
  onSubmit: () => void,
  onCancel: () => void,
): React.KeyboardEventHandler {
  return (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault()
      onSubmit()
    }
    if (e.key === 'Escape') {
      e.preventDefault()
      onCancel()
    }
  }
}
