import { useRef, useCallback } from 'react'

/**
 * Mac Chrome では IME の変換確定 Enter 時に compositionend → keydown の順で
 * イベントが発火し、keydown 時点で isComposing=false になるため、
 * 通常の Enter と区別できない問題がある。
 *
 * このフックは compositionend 直後の keydown を無視するためのガードを提供する。
 */
export function useCompositionGuard() {
  const isComposingRef = useRef(false)

  const onCompositionStart = useCallback(() => {
    isComposingRef.current = true
  }, [])

  const onCompositionEnd = useCallback(() => {
    // compositionend 後の同フレーム内で発火する keydown をブロックするため、
    // requestAnimationFrame でリセットを遅延させる
    requestAnimationFrame(() => {
      isComposingRef.current = false
    })
  }, [])

  return { isComposingRef, onCompositionStart, onCompositionEnd }
}
