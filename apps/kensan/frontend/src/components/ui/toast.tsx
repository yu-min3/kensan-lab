import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from "react";
import { X } from "lucide-react";
import { Button } from "./button";

// patterns.md 05. Toast / Notification — 一時的（自動消滅）。
// 位置は bottom-right（モバイルは bottom-center）、stack は最大 3 件、タイトル必須。
// 取り消し可能な操作（削除など）は action に「元に戻す」を渡す（04. Inline + Undo）。

export interface ToastInput {
  title: string;
  desc?: string;
  /** 「元に戻す」等。押すとトーストは即閉じる */
  action?: { label: string; onClick: () => void };
  /** 既定 6s。undo 付き削除は 8s（patterns.md: Undo は 6–8 秒表示） */
  durationMs?: number;
}

interface ToastItem extends ToastInput {
  id: number;
}

const ToastContext = createContext<(t: ToastInput) => void>(() => {});

export function useToast() {
  return useContext(ToastContext);
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);
  const seq = useRef(0);

  const dismiss = useCallback((id: number) => {
    setItems((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const toast = useCallback(
    (input: ToastInput) => {
      const id = ++seq.current;
      // stack 最大 3 件。古いものから消す
      setItems((prev) => [...prev.slice(-2), { ...input, id }]);
      window.setTimeout(() => dismiss(id), input.durationMs ?? 6000);
    },
    [dismiss],
  );

  return (
    <ToastContext.Provider value={toast}>
      {children}
      <div
        className="fixed bottom-6 inset-x-4 sm:inset-x-auto sm:right-6 sm:w-80 z-[60] flex flex-col gap-2 pointer-events-none"
        role="status"
        aria-live="polite"
      >
        {items.map((t) => (
          <div
            key={t.id}
            className="pointer-events-auto flex items-start gap-2 rounded-md border border-border bg-card shadow-lg px-3 py-2.5"
          >
            <div className="flex-1 min-w-0">
              <p className="text-sm font-semibold">{t.title}</p>
              {t.desc && <p className="text-xs text-muted-foreground mt-0.5">{t.desc}</p>}
            </div>
            {t.action && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  t.action?.onClick();
                  dismiss(t.id);
                }}
              >
                {t.action.label}
              </Button>
            )}
            <button
              className="text-muted-foreground hover:text-foreground p-1 -m-1 shrink-0"
              aria-label="通知を閉じる"
              onClick={() => dismiss(t.id)}
            >
              <X size={14} />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}
