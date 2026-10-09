import { useSearchParams } from "react-router-dom";
import { FileText } from "lucide-react";
import { Card, CardBody } from "../components/ui/card";
import { Empty } from "../components/ui/states";
import { markportURL } from "../lib/markport";

// ノート一覧・Markdown Viewer は markport へ統合した（2026-10-09）。旧 URL・ブックマークの受け皿。
// markport は Mac のローカルでしか動かないので、自動遷移せずリンクだけ出す。
export function MarkportRedirect() {
  const [params] = useSearchParams();
  const path = params.get("path") ?? "";
  return (
    <Card>
      <CardBody>
        <Empty
          icon={<FileText />}
          title="ノートは markport で開きます"
          desc={path ? `「${path}」は Mac 上の markport で読めます。` : "ノートの閲覧・検索は Mac 上の markport に移りました。"}
          actions={
            <a
              href={markportURL(path)}
              className="ds-control inline-flex items-center rounded-lg bg-brand text-brand-foreground text-sm px-3.5 font-medium hover:opacity-90"
            >
              markport で開く
            </a>
          }
        />
      </CardBody>
    </Card>
  );
}
