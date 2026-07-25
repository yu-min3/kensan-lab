import { Link } from "react-router-dom";
import { Badge } from "./ui/badge";

// タスクに付く project バッジ = その project 詳細（/projects?name=）へのリンク。
// 「このタスク何だっけ」からプロジェクトの状況（概要・現在地・ログ）へ 1 クリックで辿れる。
// クリックは行のドラッグ・編集ダイアログに伝播させない（stopPropagation）。
export function ProjectBadge({ project }: { project: string }) {
  return (
    <Link
      to={`/projects?name=${encodeURIComponent(project)}`}
      className="shrink-0 rounded-sm hover:opacity-80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring transition-opacity"
      title={`プロジェクト「${project}」を開く`}
      onClick={(e) => e.stopPropagation()}
    >
      <Badge variant="outline">{project}</Badge>
    </Link>
  );
}
