import { useEffect, useRef, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, todayISO } from "../lib/api";
import { Card, CardHead, CardBody } from "./ui/card";
import { ErrorState, Skeleton } from "./ui/states";
import { SaveStatus, type SaveState } from "./ui/save-status";

// ダッシュボードの落書きボード。メモ（構造化カード）とは別に、1 枚の白紙に
// 乱雑に書き殴る場所。実体は whiteboard.md。frontmatter は隠し、本文だけ編集する。
// 保存は 800ms デバウンスの自動保存。

const PATH = "whiteboard.md";

function seed(today: string): string {
  return `---\ntype: memo\ntags: [whiteboard]\ncreated: ${today}\nupdated: ${today}\n---\n\n`;
}

// frontmatter（--- ... ---）と本文を分ける。本文だけを textarea に出す。
function splitFm(content: string): { fm: string; body: string } {
  const m = content.match(/^---\n[\s\S]*?\n---\n?/);
  if (m) return { fm: m[0].replace(/\n*$/, "\n"), body: content.slice(m[0].length).replace(/^\n+/, "") };
  return { fm: "", body: content };
}

export function Whiteboard() {
  const qc = useQueryClient();
  const wb = useQuery({
    queryKey: ["file", PATH],
    queryFn: () => api.file(PATH),
    retry: (count, err) => !(err instanceof ApiError && err.status === 404) && count < 2,
  });

  const notFound = wb.error instanceof ApiError && wb.error.status === 404;

  // 初回 404 のときは骨組みごと作成する
  const create = useMutation({
    mutationFn: () => api.createFile(PATH, seed(todayISO())),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["file", PATH] }),
  });
  useEffect(() => {
    if (notFound && !create.isPending && !create.isSuccess) create.mutate();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [notFound]);

  // 編集中のドラフトと、保存に使う baseMtime（保存成功ごとに最新へ進める）
  const [body, setBody] = useState<string | null>(null);
  const baseMtime = useRef<string>("");
  const fm = useRef<string>("");
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // サーバ値が来たら（かつ未編集なら）ドラフトを同期する
  useEffect(() => {
    if (!wb.data) return;
    const parts = splitFm(wb.data.content);
    fm.current = parts.fm;
    if (body === null) {
      baseMtime.current = wb.data.doc.mtime;
      setBody(parts.body);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [wb.data]);

  const save = useMutation({
    mutationFn: (next: string) => api.putFile(PATH, fm.current + next, baseMtime.current),
    onSuccess: (res) => {
      baseMtime.current = res.doc.mtime;
    },
  });

  function onChange(next: string) {
    setBody(next);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => save.mutate(next), 800);
  }

  useEffect(() => () => void (timer.current && clearTimeout(timer.current)), []);

  const conflict = save.error instanceof ApiError && save.error.status === 409;
  const saveState: SaveState = conflict
    ? "conflict"
    : save.isError
      ? "error"
      : save.isPending
        ? "saving"
        : save.isSuccess
          ? "saved"
          : "idle";

  return (
    <Card>
      <CardHead
        title="ホワイトボード"
        sub="whiteboard.md"
        badge={body !== null ? <SaveStatus state={saveState} idleText="自動保存" /> : undefined}
      />
      <CardBody>
        {wb.isPending || (notFound && !wb.data) ? (
          <Skeleton className="h-64 w-full" />
        ) : wb.isError && !notFound ? (
          <ErrorState error={wb.error} onRetry={() => wb.refetch()} />
        ) : conflict ? (
          <ErrorState
            error={new ApiError(409, "他のクライアント（Claude Code / VSCode）が先に保存しました。再読込してください。")}
            onRetry={() => {
              save.reset();
              setBody(null);
              wb.refetch();
            }}
          />
        ) : (
          <textarea
            id="whiteboard-body"
            name="whiteboard-body"
            className="w-full min-h-[16rem] resize-y bg-transparent text-sm leading-relaxed focus:outline-none font-sans"
            placeholder="思考の置き場。なんでも書き殴る…"
            value={body ?? ""}
            onChange={(e) => onChange(e.target.value)}
            aria-label="ホワイトボード"
          />
        )}
      </CardBody>
    </Card>
  );
}
