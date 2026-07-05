import { useEffect, useRef, type CSSProperties } from "react";
import { Crepe } from "@milkdown/crepe";
import "@milkdown/crepe/theme/common/style.css";
import "@milkdown/crepe/theme/frame.css";

// Milkdown(Crepe): remark ベースの WYSIWYG。## を打つとその場で見出しにレンダリング。
// kensan の標準エディタ（2026-07 に CodeMirror / TipTap から一本化。docs/refactoring_master_plan.md P2）。
// 非制御（defaultValue で初期化、編集結果は onChange で md として受け取る）。
// 内容を外から差し替えるときは key を変えて remount する（useAutosaveFile の editorKey）。
export function MilkdownEditor({
  defaultValue,
  onChange,
  placeholder,
  minHeight,
}: {
  defaultValue: string;
  onChange?: (md: string) => void;
  placeholder?: string;
  minHeight?: string;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const cbRef = useRef(onChange);
  cbRef.current = onChange;

  useEffect(() => {
    if (!ref.current) return;
    const crepe = new Crepe({
      root: ref.current,
      defaultValue,
      featureConfigs: {
        [Crepe.Feature.Placeholder]: { text: placeholder ?? "", mode: "block" },
      },
    });
    // Crepe は create 時に正規化済みの初回 markdownUpdated を発火する。これを onChange に
    // 流すと「開いただけ」で dirty 判定 → 自動保存が走り本文がリフォーマットされ得る
    // （git で版管理する文書では特に困る）。初回 1 回だけスキップする。
    let initialEmit = true;
    crepe.on((listener) => {
      listener.markdownUpdated((_ctx, markdown) => {
        if (initialEmit) {
          initialEmit = false;
          return;
        }
        cbRef.current?.(markdown);
      });
    });
    crepe.create();
    return () => {
      crepe.destroy();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div
      ref={ref}
      className="milkdown-host text-sm"
      style={minHeight ? ({ "--editor-min-h": minHeight } as CSSProperties) : undefined}
    />
  );
}
