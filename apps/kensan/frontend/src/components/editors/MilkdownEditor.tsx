import { useEffect, useRef, type CSSProperties } from "react";
import { Crepe } from "@milkdown/crepe";
import { serializerCtx } from "@milkdown/kit/core";
import { Plugin } from "@milkdown/kit/prose/state";
import { $prose } from "@milkdown/kit/utils";
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
    // The standard markdownUpdated event is debounced by 200ms. Observe document
    // transactions immediately so leaving the editor can flush the last keystroke.
    // Initialization is excluded, so opening a file never reformats/saves it.
    let ready = false;
    crepe.editor.use($prose(ctx => new Plugin({
      view: () => ({
        update: (view, previous) => {
          if (ready && !previous.doc.eq(view.state.doc)) {
            cbRef.current?.(ctx.get(serializerCtx)(view.state.doc));
          }
        },
      }),
    })));
    void crepe.create().then(() => { ready = true; });
    return () => {
      ready = false;
      void crepe.destroy();
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
