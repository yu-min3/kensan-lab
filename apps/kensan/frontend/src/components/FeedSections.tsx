import { useState } from "react";
import { Check, ChevronDown, ChevronUp, CircleCheckBig, Gauge, Inbox, Newspaper, Radar, RotateCcw } from "lucide-react";
import { MarkdownView } from "./MarkdownView";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
import { Card, CardBody } from "./ui/card";

// レポート冒頭の要約。読む順が意味を持つ唯一のzoneなので、既知zoneとして
// 先頭に固定する。未知sectionと同じ`others`扱いだと末尾へ流れる。
const SUMMARY_TITLE = "30秒サマリー";

export interface FeedSection {
  title: string;
  content: string;
  count: number;
}

export interface FeedItem {
  title: string;
  content: string;
  key: string;
  version: string;
}

export function splitFeedSections(markdown: string): FeedSection[] {
  const matches = [...markdown.matchAll(/^##[ \t]+(.+?)\s*$/gm)];
  if (matches.length === 0) {
    return [{ title: "レポート", content: markdown.trim(), count: countItems(markdown) }];
  }
  return matches.map((match, index) => {
    const start = (match.index ?? 0) + match[0].length;
    const end = matches[index + 1]?.index ?? markdown.length;
    const content = markdown.slice(start, end).trim();
    return { title: match[1].trim(), content, count: countItems(content) };
  });
}

function countItems(markdown: string): number {
  return [...markdown.matchAll(/^###[ \t]+/gm)].length;
}

export function splitFeedItems(markdown: string): FeedItem[] {
  const matches = [...markdown.matchAll(/^###[ \t]+(.+?)\s*$/gm)];
  return matches.map((match, index) => {
    const start = (match.index ?? 0) + match[0].length;
    const end = matches[index + 1]?.index ?? markdown.length;
    const content = markdown.slice(start, end).trim();
    const source = content.match(/(?:出典|URL):[ \t]*(https?:\/\/[^\s)）]+)/i)?.[1];
    const updatedAt = content.match(/(?:更新日時|受信日時):[ \t]*(\S+)/)?.[1];
    const title = match[1].trim();
    return {
      title,
      content,
      key: source ?? `title:${title.replace(/[（(].*?[）)]/g, "").trim().toLowerCase()}`,
      version: updatedAt ?? `content:${stableHash(normalizeForFingerprint(content))}`,
    };
  });
}

function normalizeForFingerprint(content: string): string {
  return content
    .replace(/前回(?:・前々回)?(?:のレポート)?[^。\n]*[。\n]?/g, "")
    .replace(/締切まで(?:残り)?\d+日/g, "締切まで")
    .replace(/\s+/g, " ")
    .trim();
}

function stableHash(value: string): string {
  let hash = 2166136261;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return (hash >>> 0).toString(16).padStart(8, "0");
}

function EmptySection({ children }: { children: string }) {
  return (
    <div className="rounded-md border border-dashed border-border px-4 py-5 text-sm text-muted-foreground">
      {children}
    </div>
  );
}

function SectionBody({ section, empty }: { section?: FeedSection; empty: string }) {
  if (!section || !section.content) return <EmptySection>{empty}</EmptySection>;
  return <MarkdownView content={section.content} />;
}

function InboxItems({
  section,
  empty,
  acknowledgedVersions,
  pendingKey,
  onAcknowledgement,
}: {
  section?: FeedSection;
  empty: string;
  acknowledgedVersions: Map<string, string>;
  pendingKey?: string;
  onAcknowledgement: (item: FeedItem, acknowledged: boolean) => void;
}) {
  const [showAcknowledged, setShowAcknowledged] = useState(false);
  if (!section || !section.content) return <EmptySection>{empty}</EmptySection>;
  const items = splitFeedItems(section.content);
  if (items.length === 0) return <MarkdownView content={section.content} />;
  const visible = items.filter((item) => acknowledgedVersions.get(item.key) !== item.version);
  const acknowledged = items.filter((item) => acknowledgedVersions.get(item.key) === item.version);

  return (
    <div className="space-y-3">
      {visible.length === 0 && <EmptySection>{empty}</EmptySection>}
      {visible.map((item) => (
        <article key={item.key} className="rounded-lg border border-border bg-card p-3">
          <h5 className="font-semibold leading-snug">{item.title}</h5>
          <div className="mt-2">
            <MarkdownView content={item.content} />
          </div>
          <div className="mt-3 flex justify-end border-t border-border/60 pt-2">
            <Button
              variant="ghost"
              size="sm"
              loading={pendingKey === item.key}
              onClick={() => onAcknowledgement(item, true)}
            >
              <Check className="size-3.5" />
              確認済みにする
            </Button>
          </div>
        </article>
      ))}
      {acknowledged.length > 0 && (
        <div className="border-t border-border/60 pt-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setShowAcknowledged((value) => !value)}
          >
            {showAcknowledged ? <ChevronUp className="size-3.5" /> : <ChevronDown className="size-3.5" />}
            確認済み {acknowledged.length}件
          </Button>
          {showAcknowledged && (
            <div className="mt-2 space-y-2">
              {acknowledged.map((item) => (
                <article key={item.key} className="rounded-lg border border-border/60 bg-muted/20 p-3 opacity-75">
                  <h5 className="font-medium leading-snug line-through">{item.title}</h5>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="mt-2"
                    loading={pendingKey === item.key}
                    onClick={() => onAcknowledgement(item, false)}
                  >
                    <RotateCcw className="size-3.5" />
                    戻す
                  </Button>
                </article>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export function FeedSections({
  content,
  acknowledgedVersions = new Map<string, string>(),
  pendingKey,
  onAcknowledgement = () => undefined,
}: {
  content: string;
  acknowledgedVersions?: Map<string, string>;
  pendingKey?: string;
  onAcknowledgement?: (item: FeedItem, acknowledged: boolean) => void;
}) {
  const sections = splitFeedSections(content);
  const byTitle = new Map(sections.map((section) => [section.title, section]));
  const summary = byTitle.get(SUMMARY_TITLE);
  const action = byTitle.get("要対応");
  const confirmation = byTitle.get("確認");
  const news = byTitle.get("今日のニュース");
  const watch = byTitle.get("リリース・定点観測");
  const known = new Set([SUMMARY_TITLE, "要対応", "確認", "今日のニュース", "リリース・定点観測"]);
  const others = sections.filter((section) => !known.has(section.title));
  const visibleCount = (section?: FeedSection) =>
    section ? splitFeedItems(section.content).filter((item) => acknowledgedVersions.get(item.key) !== item.version).length : 0;
  const actionCount = visibleCount(action);
  const confirmationCount = visibleCount(confirmation);

  return (
    <div className="space-y-5">
      {summary?.content ? (
        <Card className="min-w-0 overflow-hidden border-brand/30">
          <div className="flex flex-wrap items-center justify-between gap-2 border-b border-brand/20 bg-brand-muted/20 px-5 py-4">
            <div className="flex items-center gap-2">
              <Gauge className="size-5 text-brand" />
              <div>
                <h3 className="h-serif text-lg font-bold">{SUMMARY_TITLE}</h3>
                <p className="text-xs text-muted-foreground">今日の要点と判断</p>
              </div>
            </div>
          </div>
          <CardBody>
            <MarkdownView content={summary.content} />
          </CardBody>
        </Card>
      ) : null}

      <section aria-labelledby="feed-inbox-heading">
        <div className="mb-2 flex flex-wrap items-end justify-between gap-2">
          <div>
            <div className="flex items-center gap-2">
              <Inbox className="size-4 text-warning" />
              <h3 id="feed-inbox-heading" className="h-serif text-lg font-bold">
                Inbox
              </h3>
            </div>
            <p className="mt-0.5 text-xs text-muted-foreground">Gmailなど、先に目を通す個人的な通知</p>
          </div>
          <Badge variant={actionCount ? "warning" : "muted"}>
            要対応 {actionCount}
          </Badge>
        </div>
        <div className="grid min-w-0 grid-cols-1 gap-3 xl:grid-cols-2">
          <Card className="min-w-0 overflow-hidden border-warning/40">
            <div className="flex items-center justify-between border-b border-warning/30 bg-warning/5 px-4 py-3">
              <h4 className="font-semibold">要対応</h4>
              <Badge variant={actionCount ? "warning" : "muted"} dot>
                {actionCount}件
              </Badge>
            </div>
            <CardBody>
              <InboxItems
                section={action}
                empty="未確認の要対応はありません。"
                acknowledgedVersions={acknowledgedVersions}
                pendingKey={pendingKey}
                onAcknowledgement={onAcknowledgement}
              />
            </CardBody>
          </Card>
          <Card className="min-w-0 overflow-hidden">
            <div className="flex items-center justify-between border-b border-border bg-muted/30 px-4 py-3">
              <div className="flex items-center gap-2">
                <CircleCheckBig className="size-4 text-muted-foreground" />
                <h4 className="font-semibold">確認</h4>
              </div>
              <Badge variant="muted">{confirmationCount}件</Badge>
            </div>
            <CardBody>
              <InboxItems
                section={confirmation}
                empty="未確認の確認事項はありません。"
                acknowledgedVersions={acknowledgedVersions}
                pendingKey={pendingKey}
                onAcknowledgement={onAcknowledgement}
              />
            </CardBody>
          </Card>
        </div>
      </section>

      <Card className="min-w-0 overflow-hidden border-brand/30">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-brand/20 bg-brand-muted/20 px-5 py-4">
          <div className="flex items-center gap-2">
            <Newspaper className="size-5 text-brand" />
            <div>
              <h3 className="h-serif text-lg font-bold">今日のニュース</h3>
              <p className="text-xs text-muted-foreground">最近の活動と関心から選んだトピック</p>
            </div>
          </div>
          <Badge variant="muted">{news?.count ?? 0}件</Badge>
        </div>
        <CardBody>
          <SectionBody section={news} empty="今日のニュースはありません。" />
        </CardBody>
      </Card>

      <Card className="min-w-0 overflow-hidden">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border bg-muted/20 px-5 py-4">
          <div className="flex items-center gap-2">
            <Radar className="size-5 text-muted-foreground" />
            <div>
              <h3 className="h-serif text-lg font-bold">リリース・定点観測</h3>
              <p className="text-xs text-muted-foreground">追っているプロジェクトの変化</p>
            </div>
          </div>
          <Badge variant="muted">{watch?.count ?? 0}件</Badge>
        </div>
        <CardBody>
          <SectionBody section={watch} empty="新しい更新はありません。" />
        </CardBody>
      </Card>

      {others.map((section) => (
        <Card key={section.title} className="min-w-0 overflow-hidden">
          <div className="border-b border-border px-5 py-4">
            <h3 className="h-serif text-lg font-bold">{section.title}</h3>
          </div>
          <CardBody>
            <SectionBody section={section} empty="項目はありません。" />
          </CardBody>
        </Card>
      ))}
    </div>
  );
}
