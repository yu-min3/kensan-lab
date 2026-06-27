import { PageHeader } from "../components/PageHeader";
import { MemoCards } from "../components/MemoCards";

// メモページ。旧 kensan の「ポコポコ追加できてカードで一覧」体験を memos.md の上に再現。
// 追加・編集・削除は楽観ロック付きで保存し、競合は隠さず伝える（MemoCards）。
export function MemoPage() {
  return (
    <>
      <PageHeader
        eyebrow="記録 · memos.md"
        title="メモ"
        sub="思いついたらここに落とす。Pinned は上に固定、Scratch はカードで一覧。整理は週次レビューで。"
      />
      <MemoCards />
    </>
  );
}
