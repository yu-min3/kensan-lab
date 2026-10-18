import type { PortfolioView } from "../lib/api";

// 技術レーダー（portfolio.md の ## 技術）。プロジェクトの操作はホームの一覧へ集約する。

const RING_CLASS: Record<string, string> = { 採用: "pf-ring-adopt", 試す: "pf-ring-trial", 調べる: "pf-ring-assess", やめる: "pf-ring-hold" };

type Pick = { tech: string | null; onPick: (name: string) => void };

// ---- 技術レーダー

const C = 260;
const R = [124, 174, 212, 244];

export function RadarPanel({ v, tech, onPick }: { v: PortfolioView } & Pick) {
  const { rings, items } = v.radar;
  const quads = v.radar.quadrants.length > 0 ? v.radar.quadrants : [...new Set(items.map((t) => t.quadrant))];
  const n = Math.max(quads.length, 1);
  const span = 360 / n;
  const sel = items.find((t) => t.name === tech) ?? null;
  const users = sel ? v.projects.filter((p) => p.stack.includes(sel.name)).map((p) => p.name) : [];
  const placed = quads.flatMap((q, qi) =>
    rings.flatMap((r, ri) => {
      const list = items.filter((t) => t.quadrant === q && t.ring === r);
      const inner = ri ? R[ri - 1] : 30;
      const outer = R[ri];
      const a0 = 180 + qi * span;
      const pad = Math.min(24, span / 4);
      return list.map((t, k) => {
        const frac = (k + 1) / (list.length + 1);
        const ang = ((a0 + pad + (span - pad * 2) * frac) * Math.PI) / 180;
        const lv = list.length > 3 ? [0.3, 0.62, 0.86][k % 3] : list.length > 1 ? [0.4, 0.72][k % 2] : 0.5;
        const rad = inner + (outer - inner) * lv;
        return { t, x: C + rad * Math.cos(ang), y: C + rad * Math.sin(ang) };
      });
    }),
  );
  // 番号は一覧（区画順・ファイル順）と揃える
  const num = new Map(quads.flatMap((q) => items.filter((t) => t.quadrant === q)).map((t, i) => [t.name, i + 1]));
  return (
    <section aria-labelledby="radar-h" className="bg-card border border-border rounded-xl p-4 sm:p-5 flex flex-col gap-3 min-w-0">
      <div className="flex justify-between items-baseline gap-3 flex-wrap">
        <h2 id="radar-h" className="h-serif text-lg font-semibold">技術レーダー · 何を採り、何をやめたか</h2>
        <span className="text-xs text-muted-foreground">portfolio.md の ## 技術</span>
      </div>
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          portfolio.md に <code>## 技術</code> を作り、<code>### 区画</code> の下へ <code>- 採用 名前 — 何のために</code> と書くと点が並びます。
        </p>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)] gap-5 items-start">
          <div className="min-w-0">
            <svg viewBox="0 0 520 520" role="img" aria-label={`${rings.join("・")}の輪と、${quads.join("・")}の区画に技術を置いたレーダー`} className={`w-full h-auto ${sel ? "pf-radar-dim" : ""}`}>
              {[3, 2, 1, 0].map((i) => (
                <circle key={i} cx={C} cy={C} r={R[i]} className={`pf-radar-ring-${i}`} />
              ))}
              {quads.map((_, qi) => {
                const a = ((180 + qi * span) * Math.PI) / 180;
                return <line key={qi} x1={C} y1={C} x2={C + R[3] * Math.cos(a)} y2={C + R[3] * Math.sin(a)} className="pf-radar-axis" />;
              })}
              {rings.map((r, i) => (
                <text key={r} x={C + 5} y={C - ((i ? R[i - 1] : 0) + R[i]) / 2 + 4} fontSize={10.5} className="fill-muted-foreground pf-radar-label">{r}</text>
              ))}
              {quads.map((q, qi) => {
                // 区画名は外側の隅（斜め方向）か、真上・真下・真横の縁に置く
                const a = ((180 + (qi + 0.5) * span) * Math.PI) / 180;
                const cx = Math.cos(a), cy = Math.sin(a);
                const x = cx > 0.3 ? 512 : cx < -0.3 ? 8 : C;
                const y = cy > 0.3 ? 512 : cy < -0.3 ? 18 : C;
                return (
                  <text key={q} x={x} y={y} fontSize={12.5} fontWeight={700} textAnchor={cx > 0.3 ? "end" : cx < -0.3 ? "start" : "middle"} className="fill-foreground">{q}</text>
                );
              })}
              {placed.map(({ t, x, y }) => (
                <g key={t.name} className={`pf-radar-dot ${tech === t.name ? "is-hit" : ""}`} onClick={() => onPick(t.name)} role="button" aria-label={`${t.name}（${t.ring}）`}>
                  <circle cx={x} cy={y} r={9} className={RING_CLASS[t.ring]} />
                  <text x={x} y={y + 3.5} textAnchor="middle" fontSize={9.5} fontWeight={700} className="fill-card font-mono">{num.get(t.name)}</text>
                </g>
              ))}
            </svg>
            <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs">
              {rings.map((r) => (
                <span key={r} className="inline-flex items-center gap-1.5"><i className={`w-2.5 h-2.5 rounded-full ${RING_CLASS[r]}`} />{r}</span>
              ))}
            </div>
          </div>
          <div className="flex flex-col gap-3 min-w-0">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-5 gap-y-3">
              {quads.map((q) => (
                <div key={q} className="min-w-0">
                  <h3 className="text-xs font-bold tracking-wide mb-1">{q}</h3>
                  <ol className="flex flex-col">
                    {items.filter((t) => t.quadrant === q).map((t) => (
                      <li key={t.name}>
                        <button
                          type="button"
                          aria-pressed={tech === t.name}
                          onClick={() => onPick(t.name)}
                          className={`w-full grid grid-cols-[1.4rem_minmax(0,1fr)] gap-1.5 text-left text-[13px] rounded px-1 py-0.5 ${tech === t.name ? "bg-brand/15" : "hover:bg-muted"}`}
                        >
                          <span className="font-mono text-[11px] text-muted-foreground text-right pt-px">{num.get(t.name)}</span>
                          <span className="min-w-0 break-words">{t.name}<span className={`ml-1.5 text-[10px] font-bold ${RING_CLASS[t.ring]}-text`}>{t.ring}</span></span>
                        </button>
                      </li>
                    ))}
                  </ol>
                </div>
              ))}
            </div>
            <div className="rounded-lg bg-muted/60 px-3 py-2.5 text-sm min-h-[4.5rem]" aria-live="polite">
              {tech && !sel ? (
                <><b className="h-serif">{tech}</b><br /><span className="text-muted-foreground">レーダーに未登録。portfolio.md の ## 技術 に 1 行足すと出ます。</span></>
              ) : sel ? (
                <>
                  <b className="h-serif">{sel.name}</b>
                  <span className={`ml-2 text-[11px] font-bold ${RING_CLASS[sel.ring]}-text`}>{sel.ring}</span>
                  {sel.why && <><br />{sel.why}</>}
                  <br />
                  <span className="text-muted-foreground">{users.length ? `使っているプロジェクト: ${users.join("、")}` : "今のプロジェクトでは使っていない"}</span>
                </>
              ) : (
                <span className="text-muted-foreground">点か一覧を選ぶと、何のために使うかが出ます。</span>
              )}
            </div>
          </div>
        </div>
      )}
    </section>
  );
}
