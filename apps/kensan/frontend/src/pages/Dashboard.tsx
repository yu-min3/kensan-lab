import { todayISO } from "../lib/api";
import { PageHeader } from "../components/PageHeader";
import { GoalsPanel } from "../components/GoalsPanel";
import { DiaryCta } from "../components/DiaryCta";
import { TaskBoard } from "../components/TaskBoard";
import { MemoSummary } from "../components/MemoSummary";
import { Whiteboard } from "../components/Whiteboard";

const WEEKDAYS = ["日", "月", "火", "水", "木", "金", "土"];

// ダッシュボード = 1 画面のハブ（レイアウト: パターン B「右レール思考置き場」）。
// メイン列（左 2/3）: North Star → 今日やる/ストック（縦積み）。長いストックはここに集約。
// 右レール（1/3・sticky）: 日記起票・メモ・ホワイトボード。スクロールしても思考の道具が常に視界に残る。
export function Dashboard() {
  const today = new Date();
  return (
    <div className="ds-section">
      <PageHeader
        eyebrow={`今日 · ${todayISO()}`}
        title={`${today.getMonth() + 1}月${today.getDate()}日（${WEEKDAYS[today.getDay()]}）`}
        sub="北極星・今日やること・思考の置き場をここで完結させる。"
      />

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 items-start">
        {/* メイン列: North Star + 今日やる ⇄ ストック（縦積み）+ 完了済み */}
        <div className="lg:col-span-2 ds-section">
          <GoalsPanel />
          <TaskBoard lanes="stack" />
        </div>

        {/* 右レール（sticky）: 日記起票・メモ・ホワイトボード */}
        <aside className="lg:sticky lg:top-6 self-start ds-section">
          <DiaryCta />
          <section className="ds-section">
            <h2 className="h-serif text-base font-semibold px-0.5">メモ</h2>
            <MemoSummary />
          </section>
          <Whiteboard />
        </aside>
      </div>
    </div>
  );
}
