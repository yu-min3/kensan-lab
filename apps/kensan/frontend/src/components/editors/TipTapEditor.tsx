import { useEditor, EditorContent } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import TaskList from "@tiptap/extension-task-list";
import TaskItem from "@tiptap/extension-task-item";
import { Markdown } from "tiptap-markdown";

// TipTap(ProseMirror): legacy kensan と同じ WYSIWYG。このプロジェクト向けに最小構成
// （見出し・太字/斜体・リスト・チェックボックス・コード・引用）。表やスラッシュは入れない。
// md 入出力は tiptap-markdown（@tiptap/core ^3 対応）。[[wikilink]]/@due は素のテキストとして保持される。
export function TipTapEditor({ defaultValue, onChange }: { defaultValue: string; onChange?: (md: string) => void }) {
  const editor = useEditor({
    extensions: [
      StarterKit,
      TaskList,
      TaskItem.configure({ nested: true }),
      Markdown.configure({ html: false, transformPastedText: true }),
    ],
    content: defaultValue,
    onUpdate: ({ editor }) => {
      const md = (editor.storage as unknown as Record<string, { getMarkdown?: () => string }>)?.markdown?.getMarkdown?.();
      if (typeof md === "string") onChange?.(md);
    },
  });
  return <EditorContent editor={editor} className="tiptap-host text-sm" />;
}
