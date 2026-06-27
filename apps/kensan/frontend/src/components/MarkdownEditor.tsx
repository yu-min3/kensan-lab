import { useMemo } from "react";
import CodeMirror from "@uiw/react-codemirror";
import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { EditorView } from "@codemirror/view";
import { tags as t } from "@lezer/highlight";

// 世界一気持ちいい md 編集を目指す薄いラッパー。CodeMirror 6 のソース編集で、
// 見出し・太字・コード・リンクがその場で効く（Obsidian のソースモード的な感覚）。
// 生 md をそのまま扱うので git diff も構造化パースも壊れない（WYSIWYG にしない理由）。

// デザイントークンに馴染ませる editor テーマ
const editorTheme = EditorView.theme({
  "&": { backgroundColor: "transparent", color: "hsl(var(--foreground))" },
  "&.cm-editor.cm-focused": { outline: "none" },
  ".cm-content": { fontFamily: "inherit", padding: "0", caretColor: "hsl(var(--foreground))" },
  ".cm-line": { lineHeight: "1.7", padding: "0" },
  ".cm-cursor, .cm-dropCursor": { borderLeftColor: "hsl(var(--foreground))" },
  "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection": {
    backgroundColor: "hsl(var(--accent))",
  },
  ".cm-gutters": { display: "none" },
  ".cm-activeLine": { backgroundColor: "transparent" },
  ".cm-placeholder": { color: "hsl(var(--muted-foreground))" },
  ".cm-scroller": { fontFamily: "inherit" },
});

// 見出し/装飾を「書いてる最中に効かせる」ハイライト
const mdHighlight = HighlightStyle.define([
  { tag: t.heading1, fontSize: "1.4em", fontWeight: "700", lineHeight: "2" },
  { tag: t.heading2, fontSize: "1.22em", fontWeight: "700", lineHeight: "1.9" },
  { tag: t.heading3, fontSize: "1.08em", fontWeight: "700" },
  { tag: [t.heading4, t.heading5, t.heading6], fontWeight: "700" },
  { tag: t.strong, fontWeight: "700" },
  { tag: t.emphasis, fontStyle: "italic" },
  { tag: t.strikethrough, textDecoration: "line-through" },
  { tag: [t.monospace], fontFamily: "ui-monospace, SFMono-Regular, monospace", color: "hsl(var(--brand))" },
  { tag: [t.link, t.url], color: "hsl(var(--brand))" },
  { tag: [t.list], color: "hsl(var(--muted-foreground))" },
  { tag: [t.quote], color: "hsl(var(--muted-foreground))", fontStyle: "italic" },
  { tag: [t.contentSeparator], color: "hsl(var(--muted-foreground))" },
]);

export function MarkdownEditor({
  value,
  onChange,
  placeholder,
  minHeight = "60vh",
  autoFocus,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  minHeight?: string;
  autoFocus?: boolean;
}) {
  const extensions = useMemo(
    () => [
      markdown({ base: markdownLanguage }),
      syntaxHighlighting(mdHighlight),
      EditorView.lineWrapping,
    ],
    [],
  );
  return (
    <CodeMirror
      value={value}
      onChange={onChange}
      placeholder={placeholder}
      autoFocus={autoFocus}
      theme={editorTheme}
      extensions={extensions}
      basicSetup={{
        lineNumbers: false,
        foldGutter: false,
        highlightActiveLine: false,
        highlightActiveLineGutter: false,
        bracketMatching: false,
        closeBrackets: false,
        autocompletion: false,
        searchKeymap: false,
      }}
      style={{ minHeight }}
      className="text-sm"
    />
  );
}
