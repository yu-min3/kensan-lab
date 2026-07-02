import { Component, useState, type ReactNode } from "react";
import { PageHeader } from "../components/PageHeader";
import { Card, CardHead, CardBody } from "../components/ui/card";
import { MarkdownEditor } from "../components/MarkdownEditor";
import { MilkdownEditor } from "../components/editors/MilkdownEditor";
import { TipTapEditor } from "../components/editors/TipTapEditor";

const SAMPLE = `# 見出し1
## 見出し2

**太字** と *斜体* と \`code\` と ~~打消~~

- リスト
- リスト
  - ネスト

- [ ] チェックボックス（未完）
- [x] チェックボックス（完了）

> 引用ブロック

[リンク](https://example.com) ・ [[wikilink]] ・ @due(2026-07-01)

\`\`\`go
fmt.Println("code block")
\`\`\`

| 列A | 列B |
|----|----|
| 1  | 2  |
`;

// 各エディタを隔離（v3 互換などで落ちても他が生きるように）
class Boundary extends Component<{ children: ReactNode }, { err: Error | null }> {
  state = { err: null as Error | null };
  static getDerivedStateFromError(err: Error) {
    return { err };
  }
  render() {
    if (this.state.err) {
      return (
        <div className="text-sm text-destructive p-3">
          このエディタは初期化に失敗しました（依存の互換性など）。
          <pre className="text-xs mt-1 whitespace-pre-wrap text-muted-foreground">{String(this.state.err.message)}</pre>
        </div>
      );
    }
    return this.props.children;
  }
}

function Stars({ n }: { n: number }) {
  return (
    <span className="text-brand tnum" title={`${n}/5`}>
      {"★".repeat(n)}
      <span className="text-muted-foreground/40">{"★".repeat(5 - n)}</span>
    </span>
  );
}

const CRITERIA: { k: string; milkdown: number; tiptap: number; cm: number; note: string }[] = [
  { k: "## が即レンダリング", milkdown: 5, tiptap: 5, cm: 2, note: "WYSIWYG ⇔ ソース(styled)" },
  { k: "md fidelity（round-trip）", milkdown: 4, tiptap: 2, cm: 5, note: "CM=生md / TipTap=正規化強め" },
  { k: "打鍵感・feel", milkdown: 5, tiptap: 4, cm: 3, note: "Milkdown は slash 等 Notion 風" },
  { k: "独自記法 [[ ]] / @due / [ ]", milkdown: 2, tiptap: 2, cm: 5, note: "WYSIWYG は標準外を素通り/要拡張" },
  { k: "実装・保守の軽さ", milkdown: 3, tiptap: 3, cm: 5, note: "CM は既設・軽い拡張" },
  { k: "バンドルの軽さ", milkdown: 2, tiptap: 2, cm: 4, note: "WYSIWYG は重い" },
  { k: "git diff の静かさ", milkdown: 4, tiptap: 2, cm: 5, note: "CM はノイズ0" },
];

const PROCON = [
  {
    name: "Milkdown (Crepe)",
    pro: ["## が即見出しにレンダリング", "Notion/Obsidian 的な打鍵感・slash コマンド", "remark ベースで round-trip 比較的素直", "表・チェックボックスもインライン"],
    con: ["[[wikilink]]/@due は標準では素通り（要プラグイン）", "バンドル大・テーマ CSS を持ち込む", "やや新しめで学習コスト"],
  },
  {
    name: "TipTap",
    pro: ["legacy kensan と同じ WYSIWYG 体験", "ツールバー/バブルメニュー等エコシステム豊富", "定番で情報が多い"],
    con: ["md 入出力は addon 経由・正規化が強く git diff が荒れやすい", "v3 と tiptap-markdown の互換に注意", "独自記法は要拡張"],
  },
  {
    name: "CodeMirror (現状)",
    pro: ["生 md を一切書き換えない（fidelity 完璧・diff 0）", "[[ ]]/@due/[ ] がそのまま見える", "既に導入済み・軽量", "ライブプレビュー化の余地あり"],
    con: ["## は文字のまま（styled なだけ、真のレンダリングではない）", "WYSIWYG ほどの“見たまま感”は出ない"],
  },
];

export function EditorLabPage() {
  const [out, setOut] = useState<{ milkdown: string; tiptap: string; cm: string }>({
    milkdown: SAMPLE,
    tiptap: SAMPLE,
    cm: SAMPLE,
  });

  return (
    <>
      {/* TipTap 用の最小見た目（Tailwind preflight が見出しを潰すため） */}
      <style>{`
        .tiptap-host :is(h1){font-size:1.4em;font-weight:700;margin:.4em 0}
        .tiptap-host :is(h2){font-size:1.2em;font-weight:700;margin:.4em 0}
        .tiptap-host :is(h3){font-weight:700}
        .tiptap-host ul{list-style:disc;padding-left:1.4em}
        .tiptap-host ol{list-style:decimal;padding-left:1.4em}
        .tiptap-host blockquote{border-left:3px solid hsl(var(--border));padding-left:.8em;color:hsl(var(--muted-foreground))}
        .tiptap-host code{font-family:ui-monospace,monospace;background:hsl(var(--muted));padding:0 .25em;border-radius:3px}
        .tiptap-host pre{background:hsl(var(--muted));padding:.6em;border-radius:6px;overflow:auto}
        .tiptap-host table{border-collapse:collapse}.tiptap-host td,.tiptap-host th{border:1px solid hsl(var(--border));padding:.2em .5em}
        .tiptap-host ul[data-type=taskList]{list-style:none;padding-left:.2em}
        .tiptap-host ul[data-type=taskList] li{display:flex;gap:.5em;align-items:flex-start}
        .tiptap-host ul[data-type=taskList] li>label{margin-top:.15em}
        .tiptap-host .ProseMirror{outline:none;min-height:18rem}
        .milkdown-host .milkdown{min-height:18rem}
      `}</style>

      <PageHeader
        eyebrow="エディタ比較 · お試し"
        title="md エディタ候補"
        sub="同じ md を 3 つのエディタで触り比べ。下に「保存される md（出力）」を出すので fidelity も見える。"
      />

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 items-start">
        <EditorCard title="Milkdown (Crepe)" sub="WYSIWYG · 推奨" output={out.milkdown}>
          <Boundary>
            <MilkdownEditor defaultValue={SAMPLE} onChange={(md) => setOut((o) => ({ ...o, milkdown: md }))} />
          </Boundary>
        </EditorCard>

        <EditorCard title="TipTap" sub="WYSIWYG · legacy が採用" output={out.tiptap}>
          <Boundary>
            <TipTapEditor defaultValue={SAMPLE} onChange={(md) => setOut((o) => ({ ...o, tiptap: md }))} />
          </Boundary>
        </EditorCard>

        <EditorCard title="CodeMirror" sub="ソース(styled) · 現状" output={out.cm}>
          <Boundary>
            <MarkdownEditor value={out.cm} onChange={(md) => setOut((o) => ({ ...o, cm: md }))} minHeight="18rem" />
          </Boundary>
        </EditorCard>
      </div>

      {/* 評価表 */}
      <Card className="mt-6">
        <CardHead title="評価表" sub="★5 = 良い（実装/バンドルは軽いほど高評価）" />
        <CardBody>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-xs text-muted-foreground border-b border-border">
                  <th className="py-2 pr-3 font-medium">観点</th>
                  <th className="py-2 px-3 font-medium">Milkdown</th>
                  <th className="py-2 px-3 font-medium">TipTap</th>
                  <th className="py-2 px-3 font-medium">CodeMirror</th>
                  <th className="py-2 pl-3 font-medium">メモ</th>
                </tr>
              </thead>
              <tbody>
                {CRITERIA.map((c) => (
                  <tr key={c.k} className="border-b border-border last:border-b-0">
                    <td className="py-2 pr-3">{c.k}</td>
                    <td className="py-2 px-3"><Stars n={c.milkdown} /></td>
                    <td className="py-2 px-3"><Stars n={c.tiptap} /></td>
                    <td className="py-2 px-3"><Stars n={c.cm} /></td>
                    <td className="py-2 pl-3 text-xs text-muted-foreground">{c.note}</td>
                  </tr>
                ))}
                <tr className="font-semibold">
                  <td className="py-2 pr-3">合計</td>
                  <td className="py-2 px-3 tnum">{CRITERIA.reduce((s, c) => s + c.milkdown, 0)}</td>
                  <td className="py-2 px-3 tnum">{CRITERIA.reduce((s, c) => s + c.tiptap, 0)}</td>
                  <td className="py-2 px-3 tnum">{CRITERIA.reduce((s, c) => s + c.cm, 0)}</td>
                  <td className="py-2 pl-3 text-xs text-muted-foreground">重み次第で順位は変わる</td>
                </tr>
              </tbody>
            </table>
          </div>
        </CardBody>
      </Card>

      {/* pro/con */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 mt-6">
        {PROCON.map((p) => (
          <Card key={p.name}>
            <CardHead title={p.name} />
            <CardBody className="ds-section">
              <div>
                <div className="text-xs text-success font-medium mb-1">Pros</div>
                <ul className="ds-stack !gap-1 text-sm">
                  {p.pro.map((x, i) => (
                    <li key={i} className="flex gap-1.5"><span className="text-success">+</span>{x}</li>
                  ))}
                </ul>
              </div>
              <div>
                <div className="text-xs text-destructive font-medium mb-1">Cons</div>
                <ul className="ds-stack !gap-1 text-sm">
                  {p.con.map((x, i) => (
                    <li key={i} className="flex gap-1.5"><span className="text-destructive">−</span>{x}</li>
                  ))}
                </ul>
              </div>
            </CardBody>
          </Card>
        ))}
      </div>

      <p className="text-sm text-muted-foreground mt-6">
        所感: <strong>“見たまま感”最優先なら Milkdown</strong>、<strong>md 純度・独自記法・git を最優先なら CodeMirror</strong>。
        TipTap は legacy 慣れの価値はあるが、md fidelity は Milkdown に劣る。日記/フリースペースは構造パース対象外なので
        WYSIWYG でも実害は小さいが、<code>[[ ]]</code>・<code>@due</code> をよく使うなら CM が無難。触ってみて決めてください。
      </p>
    </>
  );
}

function EditorCard({ title, sub, output, children }: { title: string; sub: string; output: string; children: ReactNode }) {
  const [showOut, setShowOut] = useState(false);
  return (
    <Card>
      <CardHead title={title} sub={sub} />
      <CardBody className="ds-stack">
        <div className="rounded-md border border-border bg-card p-2 min-h-[18rem]">{children}</div>
        <button className="text-xs text-brand hover:underline self-start" onClick={() => setShowOut((v) => !v)}>
          {showOut ? "出力 md を隠す" : "保存される md（出力）を見る"}
        </button>
        {showOut && (
          <pre className="text-xs whitespace-pre-wrap bg-muted rounded-md p-2 max-h-60 overflow-auto font-mono">{output}</pre>
        )}
      </CardBody>
    </Card>
  );
}
