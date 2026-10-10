import { useState } from "react";
import { Link } from "react-router-dom";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Check, X } from "lucide-react";
import { api, type PortfolioCell, type PortfolioProposal, type PortfolioView } from "../lib/api";
import { useWorkspaceRefresh } from "../hooks/useWorkspaceRefresh";
import { Button } from "../components/ui/button";
import { ErrorState, SkeletonRows } from "../components/ui/states";
import { useToast } from "../components/ui/toast";

// ダッシュボード: North Star に対して、どこに証拠があり（地図）、時間がどこへ向いていて（羅針盤）、
// 今年なにを積んだか（年報）を、ファイルから毎回組み立てて見せる。AI は使わない。
// 夜に拾われた証拠の候補（reflect/inbox）は、ここで採用か却下を押すだけ。

const STATUS_LABEL = { public: "外から見える", internal: "社内だけ", empty: "空き" } as const;
const projectURL = (name: string) => `/projects?name=${encodeURIComponent(name)}`;

export function PortfolioPage() {
  const data = useQuery({ queryKey: ["portfolio"], queryFn: api.portfolio, refetchInterval: 120_000 });
  const [tab, setTab] = useState<"map" | "annual">("map");
  if (data.isPending) return <SkeletonRows rows={8} />;
  if (data.isError) return <ErrorState error={data.error} onRetry={() => data.refetch()} />;
  const v = data.data;
  return (
    <div className="pf ds-section">
      <Hero v={v} />
      {v.missing && (
        <p className="text-sm text-muted-foreground">
          workspace 直下に <code>portfolio.md</code> がまだありません。地図は空のまま表示しています。
        </p>
      )}
      {v.proposals.length > 0 && <Proposals proposals={v.proposals} v={v} />}
      <div role="tablist" aria-label="表示" className="flex gap-1 border-b border-border">
        {([["map", "地図と羅針盤"], ["annual", `${v.annual.year} 年報`]] as const).map(([id, label]) => (
          <button
            key={id}
            role="tab"
            aria-selected={tab === id}
            onClick={() => setTab(id)}
            className={`px-3.5 py-2 -mb-px text-sm border-b-2 ${tab === id ? "border-brand font-semibold" : "border-transparent text-muted-foreground"}`}
          >
            {label}
          </button>
        ))}
      </div>
      {tab === "map" ? (
        <div className="grid grid-cols-1 xl:grid-cols-[minmax(0,1.55fr)_minmax(0,1fr)] gap-5 items-start">
          <MapPanel v={v} />
          <CompassPanel v={v} />
        </div>
      ) : (
        <AnnualView v={v} />
      )}
    </div>
  );
}

function Hero({ v }: { v: PortfolioView }) {
  const stale = reflectStale(v);
  return (
    <section aria-labelledby="ns" className="pf-sky rounded-2xl px-6 py-6 sm:px-7 grid grid-cols-1 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)] gap-6 items-end">
      <div className="min-w-0">
        <div className="pf-muted font-mono text-[11px] tracking-[0.24em]">NORTH STAR</div>
        <h1 id="ns" className="h-serif font-semibold text-[1.35rem] sm:text-[1.7rem] leading-relaxed mt-1.5 break-words">
          {v.northStar || "goals.md に North Star を書くと、ここに出ます"}
        </h1>
      </div>
      <dl className="grid grid-cols-3 gap-3">
        <Stat value={`${v.publicCells}`} sub=" / 9" label="外から見えるマス" />
        <Stat value={`${v.annual.total}`} label="今年積んだ節目" note={v.annual.month ? `今月 +${v.annual.month}` : undefined} />
        <Stat value={`${v.gapShare}`} sub="%" label="空きを埋める時間（4 週）" warn={v.gapShare === 0 && v.compass.total > 0} />
      </dl>
      {stale && <p className="lg:col-span-2 text-xs pf-muted">{stale}</p>}
    </section>
  );
}

function Stat({ value, sub, label, note, warn }: { value: string; sub?: string; label: string; note?: string; warn?: boolean }) {
  return (
    <div className="border-t border-white/20 pt-2 flex flex-col gap-0.5 min-w-0">
      <dd className={`font-mono tnum text-2xl font-semibold leading-tight ${warn ? "text-orange-300" : "text-white"}`}>
        {value}
        {sub && <small className="text-sm font-normal pf-muted">{sub}</small>}
      </dd>
      <dt className="text-[11px] leading-snug pf-muted">{label}</dt>
      {note && <span className="pf-glow font-mono text-[11px]">{note}</span>}
    </div>
  );
}

function reflectStale(v: PortfolioView): string | null {
  const r = v.reflect;
  if (!r) return "夜の証拠拾いはまだ動いていません。提案は /reflection を回した日に届きます。";
  if (!r.lastSuccessAt) return `夜の証拠拾いは一度も成功していません（${r.reason || r.status || "理由不明"}）。`;
  const days = (Date.parse(`${v.date}T12:00:00+09:00`) - Date.parse(r.lastSuccessAt)) / 86_400_000;
  return days > 2 ? `夜の証拠拾いが ${Math.floor(days)} 日成功していません（最後 ${r.lastSuccessAt.slice(0, 10)}・${r.reason || r.status}）。` : null;
}

function rungName(v: PortfolioView, id: string) {
  return v.rungs.find((r) => r.id === id)?.name ?? id;
}

function MapPanel({ v }: { v: PortfolioView }) {
  const firstOpen = v.cells.find((c) => c.status !== "public" && c.aims.length > 0) ?? v.cells[0];
  const [sel, setSel] = useState(`${firstOpen.rung}/${firstOpen.domain}`);
  const cur = v.cells.find((c) => `${c.rung}/${c.domain}` === sel) ?? firstOpen;
  return (
    <section aria-labelledby="map-h" className="bg-card border border-border rounded-xl p-4 sm:p-5 flex flex-col gap-3 min-w-0">
      <div className="flex justify-between items-baseline gap-3 flex-wrap">
        <h2 id="map-h" className="h-serif text-lg font-semibold">地図 · どこが埋まっているか</h2>
        <span className="text-xs text-muted-foreground">マスを押すと中の証拠が並ぶ</span>
      </div>
      <div className="grid grid-cols-[4.2rem_repeat(3,minmax(0,1fr))] sm:grid-cols-[6.2rem_repeat(3,minmax(0,1fr))] gap-1 sm:gap-1.5">
        <span />
        {v.domains.map((d) => (
          <span key={d} className="text-xs text-muted-foreground text-center">{d}</span>
        ))}
        {v.rungs.map((r) => (
          <Row key={r.id} v={v} rung={r} sel={sel} onSel={setSel} />
        ))}
      </div>
      <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
        <span><i className="inline-block w-3 h-2.5 rounded-sm mr-1.5 align-middle pf-cell-public" />外から見える</span>
        <span><i className="inline-block w-3 h-2.5 rounded-sm mr-1.5 align-middle pf-cell-internal" />力はあるが社内だけ</span>
        <span><i className="inline-block w-3 h-2.5 rounded-sm mr-1.5 align-middle pf-cell-empty" />空き</span>
        <span className="pf-gap">← 埋めに行っている取り組み</span>
      </div>
      <CellDetail v={v} c={cur} />
    </section>
  );
}

function Row({ v, rung, sel, onSel }: { v: PortfolioView; rung: PortfolioView["rungs"][number]; sel: string; onSel: (k: string) => void }) {
  return (
    <>
      <span className="text-[11px] sm:text-sm font-semibold flex flex-col justify-center leading-tight">
        {rung.name}
        <small className="hidden sm:block text-[11px] font-normal text-muted-foreground">{rung.question}</small>
      </span>
      {v.domains.map((d) => {
        const c = v.cells.find((x) => x.rung === rung.id && x.domain === d)!;
        const k = `${c.rung}/${c.domain}`;
        const top = c.evidence.find((e) => e.public) ?? c.evidence[0];
        return (
          <button
            key={k}
            type="button"
            aria-pressed={sel === k}
            aria-label={`${rung.name} × ${d}: ${STATUS_LABEL[c.status]}`}
            onClick={() => onSel(k)}
            className={`pf-cell-${c.status} text-left rounded-lg p-1.5 sm:p-2.5 min-h-[76px] sm:min-h-[92px] flex flex-col gap-0.5 min-w-0 ${sel === k ? "ring-2 ring-brand" : ""}`}
          >
            <span className={`pf-t-${c.status} text-[10px] sm:text-[11px] font-bold`}>{STATUS_LABEL[c.status]}</span>
            <span className="text-[11px] sm:text-[13px] leading-snug line-clamp-2 break-words">
              {top ? top.title : c.note || ""}
            </span>
            <span className="mt-auto font-mono text-[10px] sm:text-[11px]">
              {c.aims.length > 0 ? (
                <span className="pf-gap">← {c.aims.map((a) => a.project).join("・")}</span>
              ) : c.evidence.length > 0 ? (
                <span className="text-muted-foreground">{c.evidence.length} 件</span>
              ) : null}
            </span>
          </button>
        );
      })}
    </>
  );
}

function CellDetail({ v, c }: { v: PortfolioView; c: PortfolioCell }) {
  return (
    <div className="border-t border-border pt-3 flex flex-col gap-2.5" aria-live="polite">
      <h3 className="h-serif font-semibold">
        {rungName(v, c.rung)} × {c.domain}
        <span className={`ml-2 text-xs font-sans pf-t-${c.status}`}>{STATUS_LABEL[c.status]}</span>
      </h3>
      {c.evidence.length === 0 && <p className="text-sm text-muted-foreground">{c.note || "まだ証拠がありません。"}</p>}
      {c.evidence.map((e) => (
        <div key={e.raw} className="grid grid-cols-[4.4rem_minmax(0,1fr)] gap-x-3 gap-y-0.5 rounded-lg bg-muted/60 px-3 py-2.5">
          <span className="font-mono text-xs text-muted-foreground row-span-3">{e.when}</span>
          <b className="text-sm">{e.title}</b>
          {(e.story || e.impact) && (
            <span className="text-[13px]">
              {e.story}
              {e.impact && <span className="text-muted-foreground"> → {e.impact}</span>}
            </span>
          )}
          <span className={`text-[11px] font-bold ${e.public ? "pf-t-public" : "pf-t-internal"}`}>
            {e.public ? "外から見える" : "社内だけ"}
            {e.project && <span className="font-normal text-muted-foreground"> · {e.project}</span>}
          </span>
        </div>
      ))}
      {c.aims.map((a) => (
        <div key={a.project} className="pf-gap-bg rounded-lg px-3 py-2.5 flex flex-wrap items-center justify-between gap-2 text-sm">
          <span>
            <b className="pf-gap">{a.project}</b> が埋めに行く{a.next ? ` · 次の節目「${a.next}」` : ""}
          </span>
          <Link to={projectURL(a.project)} className="text-brand text-sm">プロジェクトを開く →</Link>
        </div>
      ))}
    </div>
  );
}

function CompassPanel({ v }: { v: PortfolioView }) {
  const c = v.compass;
  const behind = [...c.allocs].filter((a) => a.target > 0).sort((a, b) => (b.target - b.actual) - (a.target - a.actual))[0];
  const ahead = [...c.allocs].sort((a, b) => (b.actual - b.target) - (a.actual - a.target))[0];
  const nextFor = (p: string) => v.cells.flatMap((x) => x.aims).find((a) => a.project === p)?.next;
  return (
    <section aria-labelledby="cmp-h" className="bg-card border border-border rounded-xl p-4 sm:p-5 flex flex-col gap-3 min-w-0">
      <div className="flex justify-between items-baseline gap-3 flex-wrap">
        <h2 id="cmp-h" className="h-serif text-lg font-semibold">羅針盤 · 時間がどこへ向いているか</h2>
        <span className="text-xs text-muted-foreground">{c.from.slice(5)}〜{c.to.slice(5)}</span>
      </div>
      {c.allocs.length === 0 ? (
        <p className="text-sm text-muted-foreground">goals.md に <code>## 配分</code> を書くと、狙いと実際を並べられます。</p>
      ) : (
        <div className="flex flex-col gap-3">
          {c.allocs.map((a) => (
            <div key={a.project} className="grid grid-cols-[5.5rem_minmax(0,1fr)_2.8rem] gap-2.5 items-center text-sm">
              <Link to={projectURL(a.project)} className="truncate hover:underline">{a.project}</Link>
              <div className="flex flex-col gap-1.5">
                <div className="relative h-4 rounded bg-muted" role="img" aria-label={`${a.project} 実際 ${a.actual}% 狙い ${a.target}%`}>
                  <i className="absolute inset-y-0 left-0 rounded bg-brand" style={{ width: `${a.actual}%` }} />
                  {a.target > 0 && <u className="absolute -inset-y-1 w-0.5 rounded pf-gap-line" style={{ left: `${a.target}%` }} />}
                </div>
                <div className="flex items-end gap-0.5 h-5" aria-hidden="true">
                  {a.weeks.map((w, i) => (
                    <span key={i} className={`w-2.5 rounded-t-sm ${i === a.weeks.length - 1 ? "bg-brand" : "bg-border"}`} style={{ height: `${Math.max(2, Math.min(20, w * 3))}px` }} />
                  ))}
                </div>
              </div>
              <span className={`font-mono tnum text-right ${a.actual === 0 && a.target > 0 ? "pf-gap font-semibold" : ""}`}>{a.actual}%</span>
            </div>
          ))}
        </div>
      )}
      {behind && behind.target - behind.actual >= 10 && (
        <div className="border-l-[3px] border-[hsl(var(--pf-gap))] pl-3 py-1 text-sm flex flex-col gap-1">
          <span>
            {ahead && ahead.actual > ahead.target && <>時間の <b className="pf-gap">{ahead.actual}%</b> が {ahead.project} へ。</>}
            狙い {behind.target}% の <b className="pf-gap">{behind.project} は {behind.actual}%</b>。
          </span>
          {nextFor(behind.project) && <span>今週のリバランス案: {behind.project} に 1 コマ — <b className="pf-gap">{nextFor(behind.project)}</b></span>}
        </div>
      )}
      <p className="text-[11px] text-muted-foreground">
        青 = 直近 4 週の完了（@done）とログの数の割合（量の目安で、価値ではない）。橙の目盛り = goals.md の ## 配分。仕事の時間は含まない。
      </p>
    </section>
  );
}

function AnnualView({ v }: { v: PortfolioView }) {
  const blank = v.cells.filter((c) => c.status === "empty" && c.aims.length === 0).length;
  return (
    <section aria-label={`${v.annual.year} 年報`} className="flex flex-col gap-5">
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <Big n={v.annual.total} t="今年積んだ節目" />
        <Big n={v.annual.month} t="今月" />
        <Big n={v.publicCells} t="外から見えるマス（9 のうち）" />
        <Big n={blank} t="誰も向かっていない空きマス" />
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4">
        {v.annual.chapters.map((ch) => (
          <article key={ch.quarter} className="bg-card border border-border rounded-xl p-4 flex flex-col gap-2 min-w-0">
            <span className="font-mono text-[11px] text-muted-foreground">{ch.quarter}</span>
            <h3 className="h-serif font-semibold">{ch.name || "（章の名前は portfolio.md の ## 年報の章）"}</h3>
            <ul className="flex flex-col gap-1 text-[13px]">
              {ch.items.map((it) => (
                <li key={it.date + it.title} className="flex gap-2">
                  <span className="font-mono text-[11px] text-muted-foreground whitespace-nowrap pt-0.5">{it.date.slice(5)}</span>
                  <span className="min-w-0">{it.title}{it.project && <span className="text-muted-foreground"> · {it.project}</span>}</span>
                </li>
              ))}
            </ul>
          </article>
        ))}
        <article className="pf-gap-bg border border-dashed border-[hsl(var(--pf-gap))] rounded-xl p-4 flex flex-col gap-2">
          <span className="font-mono text-[11px] text-muted-foreground">空いている章</span>
          <h3 className="h-serif font-semibold pf-gap">まだ書けていない話</h3>
          <ul className="flex flex-col gap-1 text-[13px]">
            {v.cells.filter((c) => c.status !== "public").flatMap((c) => c.aims.map((a) => (
              <li key={c.rung + c.domain + a.project}>{rungName(v, c.rung)} × {c.domain} — {a.project}{a.next ? `「${a.next}」` : ""}</li>
            )))}
          </ul>
        </article>
      </div>
    </section>
  );
}

function Big({ n, t }: { n: number; t: string }) {
  return (
    <div className="border-t-[3px] border-foreground pt-2">
      <div className="h-serif text-4xl font-bold leading-none tnum">{n}</div>
      <div className="text-xs text-muted-foreground mt-1">{t}</div>
    </div>
  );
}

function Proposals({ proposals, v }: { proposals: PortfolioProposal[]; v: PortfolioView }) {
  const refresh = useWorkspaceRefresh();
  const toast = useToast();
  const decide = useMutation({
    mutationFn: ({ p, accept }: { p: PortfolioProposal; accept: boolean }) => api.decideProposal(p, accept),
    onSuccess: (_, { accept }) => toast({ title: accept ? "地図に載せました" : "見送りました" }),
    onSettled: refresh,
  });
  return (
    <section aria-labelledby="prop-h" className="bg-card border border-border rounded-xl p-4 sm:p-5 flex flex-col gap-3">
      <div className="flex justify-between items-baseline gap-3 flex-wrap">
        <h2 id="prop-h" className="h-serif text-lg font-semibold">証拠の候補 · {proposals.length} 件</h2>
        <span className="text-xs text-muted-foreground">日記とログから拾った候補。採用すると地図に載る（外から見えるかは後で付ける）</span>
      </div>
      {decide.isError && <p role="alert" className="text-xs text-destructive">{decide.error.message}</p>}
      <ul className="flex flex-col gap-2">
        {proposals.map((p) => (
          <li key={p.file + p.line} className="rounded-lg bg-muted/60 px-3 py-2.5 flex flex-col sm:flex-row sm:items-center gap-2 justify-between">
            <div className="min-w-0 flex flex-col gap-0.5">
              <span className="text-[11px] text-muted-foreground">{rungName(v, p.rung)} × {p.domain} · {p.evidence.when}{p.source ? ` · ${p.source}` : ""}</span>
              <b className="text-sm">{p.evidence.title}</b>
              {(p.evidence.story || p.evidence.impact) && (
                <span className="text-[13px]">{p.evidence.story}{p.evidence.impact && <span className="text-muted-foreground"> → {p.evidence.impact}</span>}</span>
              )}
            </div>
            <div className="flex gap-2 shrink-0">
              <Button size="sm" variant="primary" loading={decide.isPending} onClick={() => decide.mutate({ p, accept: true })}>
                <Check size={14} />採用
              </Button>
              <Button size="sm" variant="ghost" onClick={() => decide.mutate({ p, accept: false })}>
                <X size={14} />見送る
              </Button>
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}
