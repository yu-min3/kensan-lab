import type { useAutosaveFile } from "../hooks/useAutosaveFile";
import { Button } from "./ui/button";

export function FileSaveRecovery({ file }: { file: ReturnType<typeof useAutosaveFile> }) {
  return <>
    {file.saveError && (
      <div role="alert" className="ds-stack rounded-md border border-destructive p-3">
        <p className="text-sm">{file.conflict ? "ほかの場所でも変更されています。入力を下に残しています。内容を確認してから続けてください。" : "保存できませんでした。入力はこのタブに残っています。"}</p>
        <textarea aria-label="保存できていない入力" readOnly value={file.draft} rows={5} className="w-full rounded border border-border bg-card p-2 text-sm" />
        <div className="ds-inline flex-wrap">
          {!file.conflict && <Button variant="outline" size="sm" onClick={() => file.flush()}>保存を再試行</Button>}
          <Button variant="outline" size="sm" onClick={file.retry}>入力を控えて最新の内容を読み込む</Button>
        </div>
      </div>
    )}
    {file.recovery && <details open>
      <summary className="cursor-pointer text-sm">読み込み前の入力（コピーして戻せます）</summary>
      <textarea aria-label="読み込み前の入力" readOnly value={file.recovery} rows={5} className="mt-2 w-full rounded border border-border bg-card p-2 text-sm" />
    </details>}
  </>;
}
