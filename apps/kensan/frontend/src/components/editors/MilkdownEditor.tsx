import { useEffect, useRef } from "react";
import { Crepe } from "@milkdown/crepe";
import "@milkdown/crepe/theme/common/style.css";
import "@milkdown/crepe/theme/frame.css";

// Milkdown(Crepe): remark ベースの WYSIWYG。## を打つとその場で見出しにレンダリング。
// 非制御（defaultValue で初期化、編集結果は onChange で md として受け取る）。
export function MilkdownEditor({ defaultValue, onChange }: { defaultValue: string; onChange?: (md: string) => void }) {
  const ref = useRef<HTMLDivElement>(null);
  const cbRef = useRef(onChange);
  cbRef.current = onChange;

  useEffect(() => {
    if (!ref.current) return;
    const crepe = new Crepe({ root: ref.current, defaultValue });
    crepe.on((listener) => {
      listener.markdownUpdated((_ctx, markdown) => cbRef.current?.(markdown));
    });
    crepe.create();
    return () => {
      crepe.destroy();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return <div ref={ref} className="milkdown-host text-sm" />;
}
