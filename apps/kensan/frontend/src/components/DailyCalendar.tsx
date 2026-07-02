import { useState, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronLeft, ChevronRight } from "lucide-react";
import clsx from "clsx";
import { api } from "../lib/api";
import { Card, CardHead, CardBody } from "./ui/card";
import { Badge } from "./ui/badge";

const WEEKDAYS = ["日", "月", "火", "水", "木", "金", "土"];

// daily/YYYY/MM/DD.md → "YYYY-MM-DD"
function pathToDate(path: string): string | null {
  const m = path.match(/daily\/(\d{4})\/(\d{2})\/(\d{2})\.md$/);
  return m ? `${m[1]}-${m[2]}-${m[3]}` : null;
}

function ymd(year: number, month: number, day: number): string {
  return `${year}-${String(month + 1).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
}

// 月初の曜日ぶん空白を詰めてから日を並べ、末尾を 7 の倍数に揃える
function monthGrid(year: number, month: number): (number | null)[] {
  const startDow = new Date(year, month, 1).getDay();
  const total = new Date(year, month + 1, 0).getDate();
  const cells: (number | null)[] = Array(startDow).fill(null);
  for (let d = 1; d <= total; d++) cells.push(d);
  while (cells.length % 7 !== 0) cells.push(null);
  return cells;
}

// 過去の日記をブラウズするカレンダー（旧 kensan の月グリッドを踏襲）。
// 記録のある日はマーカー付きでクリック可能、選択中の日と今日を強調する。
export function DailyCalendar({
  selected,
  onSelect,
}: {
  selected: string; // "YYYY-MM-DD"
  onSelect: (date: string) => void;
}) {
  const today = todayParts();
  const [sy, sm] = selected.split("-").map(Number);
  const [view, setView] = useState({ year: sy, month: sm - 1 });

  const list = useQuery({
    queryKey: ["dailyList"],
    queryFn: () => api.dailyList(400),
    staleTime: 60_000,
  });

  // 記録のある日付セット & タグ参照（最近一覧用に Doc も保持）
  const entries = useMemo(() => {
    const dates = new Set<string>();
    const docs = (list.data?.files ?? [])
      .map((d) => ({ date: pathToDate(d.path), doc: d }))
      .filter((e): e is { date: string; doc: (typeof e)["doc"] } => e.date !== null);
    for (const e of docs) dates.add(e.date);
    return { dates, docs };
  }, [list.data]);

  const cells = monthGrid(view.year, view.month);
  const monthLabel = `${view.year}年${view.month + 1}月`;
  const monthCount = useMemo(() => {
    const prefix = `${view.year}-${String(view.month + 1).padStart(2, "0")}`;
    let n = 0;
    entries.dates.forEach((d) => d.startsWith(prefix) && n++);
    return n;
  }, [entries.dates, view]);

  const shiftMonth = (delta: number) =>
    setView(({ year, month }) => {
      const m = month + delta;
      return { year: year + Math.floor(m / 12), month: ((m % 12) + 12) % 12 };
    });
  const goToday = () => setView({ year: today.year, month: today.month });

  return (
    <div className="ds-stack">
      <Card>
        <CardHead
          title="カレンダー"
          sub={`${monthLabel} · ${monthCount}件`}
          actions={
            <div className="ds-inline">
              <Button onClick={() => shiftMonth(-1)} aria-label="前の月">
                <ChevronLeft size={15} />
              </Button>
              <button
                onClick={goToday}
                className="text-xs text-muted-foreground hover:text-foreground px-1"
              >
                今日
              </button>
              <Button onClick={() => shiftMonth(1)} aria-label="次の月">
                <ChevronRight size={15} />
              </Button>
            </div>
          }
        />
        <CardBody>
          <div className="grid grid-cols-7 gap-1 mb-1">
            {WEEKDAYS.map((w, i) => (
              <div
                key={w}
                className={clsx(
                  "text-center text-[10px] font-semibold py-0.5",
                  i === 0 ? "text-destructive" : i === 6 ? "text-brand" : "text-muted-foreground",
                )}
              >
                {w}
              </div>
            ))}
          </div>
          <div className="grid grid-cols-7 gap-1">
            {cells.map((day, i) => {
              if (day === null) return <div key={`e${i}`} className="aspect-square" />;
              const date = ymd(view.year, view.month, day);
              const has = entries.dates.has(date);
              const isToday = date === today.iso;
              const isSel = date === selected;
              const isFuture = date > today.iso;
              const dow = i % 7;
              // 未来は不可。記録のない過去日もクリックで開ける（→「日記を作成」へ）。
              return (
                <button
                  key={date}
                  disabled={isFuture}
                  onClick={() => onSelect(date)}
                  title={has ? date : isFuture ? undefined : `${date}（日記を作成）`}
                  className={clsx(
                    "aspect-square rounded-md flex items-center justify-center text-xs tnum relative transition-colors duration-fast",
                    isSel
                      ? "bg-brand text-brand-foreground font-bold"
                      : has
                        ? "bg-brand-muted text-accent-foreground font-semibold hover:opacity-80"
                        : isFuture
                          ? "text-muted-foreground opacity-40 cursor-default"
                          : "text-muted-foreground hover:bg-accent",
                  )}
                  style={
                    !isSel && !has && dow === 0
                      ? { color: "hsl(var(--destructive) / 0.6)" }
                      : undefined
                  }
                >
                  {day}
                  {isToday && !isSel && (
                    <span className="absolute inset-0 rounded-md ring-1 ring-brand pointer-events-none" />
                  )}
                  {has && !isSel && (
                    <span className="absolute bottom-1 size-1 rounded-full bg-brand" />
                  )}
                </button>
              );
            })}
          </div>
        </CardBody>
      </Card>

      <Card>
        <CardHead title="最近の日記" />
        <CardBody className="ds-stack !gap-0.5">
          {entries.docs.slice(0, 8).map(({ date, doc }) => (
            <button
              key={doc.path}
              onClick={() => onSelect(date)}
              className={clsx(
                "flex items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-accent/60",
                date === selected && "bg-accent",
              )}
            >
              <span className="text-xs font-medium tnum shrink-0 w-[4.5rem]">{date}</span>
              <span className="flex gap-1 flex-wrap min-w-0">
                {(doc.meta.tags ?? []).slice(0, 2).map((t) => (
                  <Badge key={t} variant="brand" className="!text-[10px] !px-1.5 !py-0">
                    {t}
                  </Badge>
                ))}
              </span>
            </button>
          ))}
          {entries.docs.length === 0 && !list.isPending && (
            <p className="text-xs text-muted-foreground px-2 py-1">まだ日記がありません。</p>
          )}
        </CardBody>
      </Card>
    </div>
  );
}

// カレンダーのナビ用の小さな四角ボタン（ui/button の primary 系とは別の控えめな見た目）
function Button({
  children,
  ...rest
}: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      {...rest}
      className="inline-flex items-center justify-center size-6 rounded-md text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
    >
      {children}
    </button>
  );
}

function todayParts() {
  const d = new Date();
  return {
    year: d.getFullYear(),
    month: d.getMonth(),
    iso: ymd(d.getFullYear(), d.getMonth(), d.getDate()),
  };
}
