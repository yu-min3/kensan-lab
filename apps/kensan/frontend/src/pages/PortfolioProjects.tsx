import type { PortfolioProject, PortfolioTech, PortfolioView } from "../lib/api";

// 地図の下に並べる 2 枚。プロジェクトの列（README の headline / stack と、指標・マイルストーンから自動）と、
// 技術レーダー（portfolio.md の ## 技術）。技術名が一致すると、カードのチップとレーダーの点が同時に光る。

const RING_CLASS: Record<string, string> = { 採用: "pf-ring-adopt", 試す: "pf-ring-trial", 調べる: "pf-ring-assess", やめる: "pf-ring-hold" };

type Pick = { tech: string | null; onPick: (name: string) => void };

export function ProjectsPanel({ v, tech, onPick }: { v: PortfolioView } & Pick) {
  const alloc = new Map(v.compass.allocs.map((a) => [a.project, a]));
  const byName = new Map(v.radar.items.map((t) => [t.name, t]));
  return (
    <section aria-labelledby="proj-h" className="flex flex-col gap-3">
      <div className="flex justify-between items-baseline gap-3 flex-wrap">
        <h2 id="proj-h" className="h-serif text-lg font-semibold">プロジェクト · 何を目指して、何で行くか</h2>
        <span className="text-xs text-muted-foreground">README の headline と stack。技術を押すとレーダーで光る</span>
      </div>
      {v.projects.length === 0 ? (
        <p className="text-sm text-muted-foreground">README の frontmatter に <code>headline:</code> を書くと、ここにプロジェクトが並びます。</p>
      ) : (
        <div className="flex flex-col gap-3">
          {v.projects.map((p) => (
            <ProjectCard key={p.name} p={p} alloc={alloc.get(p.name)} byName={byName} tech={tech} onPick={onPick} />
          ))}
        </div>
      )}
    </section>
  );
}

function ProjectCard({ p, alloc, byName, tech, onPick }: {
  p: PortfolioProject; alloc?: PortfolioView["compass"]["allocs"][number]; byName: Map<string, PortfolioTech>;
} & Pick) {
  return (
    <article id={`project-${p.name}`} className="scroll-mt-6 bg-card border border-border rounded-xl p-4 sm:p-5 grid grid-cols-1 lg:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)] gap-x-7 gap-y-4 min-w-0">
      <div className="flex flex-col gap-3 min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-mono text-xs font-semibold">{p.name}</span>
          {p.aims.map((a) => (
            <span key={a} className="pf-gap-bg pf-gap rounded-full px-2 py-px text-[11px] font-bold whitespace-nowrap">{a.replace("×", " × ")}</span>
          ))}
          {alloc && alloc.target > 0 && (
            <span className={`ml-auto font-mono tnum text-[11px] ${alloc.actual < alloc.target * 0.6 ? "pf-gap font-semibold" : "text-muted-foreground"}`}>
              活動 {alloc.actual}% / 狙い {alloc.target}%
            </span>
          )}
        </div>
        <p className="h-serif text-[1.15rem] sm:text-[1.25rem] font-bold leading-snug break-words">
          {p.headline || <span className="text-muted-foreground font-normal text-sm">README に headline: を書くと、目指すことがここに出ます</span>}
        </p>
        {p.meter && <Meter m={p.meter} />}
        {p.next && (
          <p className="border-l-[3px] border-brand pl-3 py-0.5 text-sm">
            <span className="block font-mono text-[10px] tracking-widest text-brand">次の一手</span>
            {p.next}
          </p>
        )}
        <div className="flex flex-col gap-1.5">
          <span className="font-mono text-[10px] tracking-widest text-muted-foreground">使う技術</span>
          {p.stack.length === 0 ? (
            <span className="text-xs text-muted-foreground">README に stack: を書くと出ます</span>
          ) : (
            <div className="flex flex-wrap gap-1.5">
              {p.stack.map((s) => {
                const t = byName.get(s);
                return (
                  <button
                    key={s}
                    type="button"
                    aria-pressed={tech === s}
                    onClick={() => onPick(s)}
                    title={t ? `${t.ring}: ${t.why}` : "レーダーに未登録"}
                    className={`pf-chip ${t ? RING_CLASS[t.ring] ?? "" : "pf-chip-unknown"} ${tech === s ? "pf-chip-on" : ""}`}
                  >
                    {s}
                  </button>
                );
              })}
            </div>
          )}
        </div>
      </div>
      <div className="flex flex-col gap-2 min-w-0">
        <span className="font-mono text-[10px] tracking-widest text-muted-foreground">積み上げ</span>
        {p.steps.length === 0 ? (
          <p className="text-xs text-muted-foreground">README の ## マイルストーン を完了にすると、ここに積み上がります。</p>
        ) : (
          <ol className="flex flex-col">
            {p.steps.map((s) => (
              <li key={s.title} className="grid grid-cols-[4.6rem_0.7rem_minmax(0,1fr)] gap-2 py-1 text-[13px] items-start">
                <span className="font-mono tnum text-[11px] text-muted-foreground text-right pt-0.5">{s.date || "済"}</span>
                <i className="mt-1.5 w-2.5 h-2.5 rounded-full pf-dot-done" />
                <span className="break-words">{s.title}</span>
              </li>
            ))}
            {p.next && (
              <li className="grid grid-cols-[4.6rem_0.7rem_minmax(0,1fr)] gap-2 py-1 text-[13px] items-start">
                <span className="font-mono text-[11px] text-muted-foreground text-right pt-0.5">次</span>
                <i className="mt-1.5 w-2.5 h-2.5 rounded-full border-2 border-brand" />
                <span className="text-brand break-words">{p.next}</span>
              </li>
            )}
          </ol>
        )}
      </div>
    </article>
  );
}

function Meter({ m }: { m: NonNullable<PortfolioProject["meter"]> }) {
  const pct = m.target > 0 ? Math.min(100, (m.current / m.target) * 100) : 0;
  const unit = m.kind === "milestones" ? `/ ${m.target}` : m.unit;
  const label = m.kind === "milestones" ? "マイルストーン" : `${m.label}（目標 ${m.target}）`;
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex justify-between items-baseline gap-2">
        <span className="text-xs text-muted-foreground">{label}</span>
        <span className="h-serif tnum text-2xl font-bold leading-none">
          {m.current}
          {unit && <small className="text-xs font-normal text-muted-foreground ml-1">{unit}</small>}
        </span>
      </div>
      <div className="relative h-2.5 rounded-full bg-muted" role="img" aria-label={`${label} ${m.current} / ${m.target}`}>
        <i className="absolute inset-y-0 left-0 rounded-full pf-meter" style={{ width: `${pct}%` }} />
        {m.checkpoints.filter((c) => c.value < m.target).map((c) => (
          <u key={c.value} className="absolute -inset-y-0.5 w-0.5 bg-border" style={{ left: `${(c.value / m.target) * 100}%` }} title={c.label} />
        ))}
      </div>
      {m.checkpoints.length > 0 && (
        <div className="relative h-3.5 text-[10px] text-muted-foreground tnum" aria-hidden="true">
          {m.checkpoints.map((c) => (
            <span key={c.value} className="absolute -translate-x-1/2 whitespace-nowrap" style={{ left: `${Math.min(96, (c.value / m.target) * 100)}%` }}>{c.value}</span>
          ))}
        </div>
      )}
    </div>
  );
}

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
