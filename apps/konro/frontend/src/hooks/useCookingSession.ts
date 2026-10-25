import {useCallback, useEffect, useRef, useState} from 'react';
import {loadSession, saveSession, Session} from '../session';
import {alarm, primeAudio} from '../alerts';

/** One owner for persistence and alarms, independent of the visible page. */
export function useCookingSession() {
  const [session, setSession] = useState(loadSession);
  const current = useRef(session);
  const [error, setError] = useState('');
  const replace = useCallback((next: Session | null) => {
    try {
      saveSession(next);
      current.current = next;
      setSession(next);
      setError('');
      return true;
    } catch {
      setError('調理の状態を保存できません。端末の空き容量やブラウザの保存設定を確認してください。変更は保存されていません。');
      return false;
    }
  }, []);
  const update = useCallback((next: Session | ((s:Session)=>Session)) => {
    const previous = current.current;
    if (!previous) return false;
    const value = typeof next === 'function' ? next(previous) : next;
    // A UI update must never erase an alarm acknowledgement persisted meanwhile.
    const timers = value.timers.map(t => {
      const old = previous.timers.find(x => x.id === t.id);
      return old?.notifiedAt ? {...t, notifiedAt: old.notifiedAt} : t;
    });
    return replace({...value, timers});
  }, [replace]);
  useEffect(() => {
    const unlock = () => {try {primeAudio();} catch {/* Sound may be unavailable. */}};
    window.addEventListener('pointerdown', unlock, {once:true});
    const timer = window.setInterval(() => {
      const s = current.current;
      if (!s) return;
      const now = Date.now();
      const due = s.timers.filter(t => !t.acknowledged && !t.notifiedAt && t.endsAt <= now);
      if (!due.length) return;
      const ids = new Set(due.map(t => t.id));
      if (replace({...s, timers:s.timers.map(t => ids.has(t.id) ? {...t,notifiedAt:now} : t)})) {
        try {alarm();} catch {setError('タイマーは完了しましたが、音を再生できませんでした。');}
      }
    }, 500);
    return () => {window.clearInterval(timer);window.removeEventListener('pointerdown', unlock);};
  }, [replace]);
  return {session, update, replace, error};
}
