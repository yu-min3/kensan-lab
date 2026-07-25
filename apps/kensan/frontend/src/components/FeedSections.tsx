import { CircleCheckBig, Inbox, Newspaper, Radar } from "lucide-react";
import { MarkdownView } from "./MarkdownView";
import { Badge } from "./ui/badge";
import { Card, CardBody } from "./ui/card";

export interface FeedSection {
  title: string;
  content: string;
  count: number;
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

export function FeedSections({ content }: { content: string }) {
  const sections = splitFeedSections(content);
  const byTitle = new Map(sections.map((section) => [section.title, section]));
  const action = byTitle.get("要対応");
  const confirmation = byTitle.get("確認");
  const news = byTitle.get("今日のニュース");
  const watch = byTitle.get("リリース・定点観測");
  const known = new Set(["要対応", "確認", "今日のニュース", "リリース・定点観測"]);
  const others = sections.filter((section) => !known.has(section.title));

  return (
    <div className="space-y-5">
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
          <Badge variant={action?.count ? "warning" : "muted"}>
            要対応 {action?.count ?? 0}
          </Badge>
        </div>
        <div className="grid min-w-0 grid-cols-1 gap-3 xl:grid-cols-2">
          <Card className="min-w-0 overflow-hidden border-warning/40">
            <div className="flex items-center justify-between border-b border-warning/30 bg-warning/5 px-4 py-3">
              <h4 className="font-semibold">要対応</h4>
              <Badge variant={action?.count ? "warning" : "muted"} dot>
                {action?.count ?? 0}件
              </Badge>
            </div>
            <CardBody>
              <SectionBody section={action} empty="今日の要対応はありません。" />
            </CardBody>
          </Card>
          <Card className="min-w-0 overflow-hidden">
            <div className="flex items-center justify-between border-b border-border bg-muted/30 px-4 py-3">
              <div className="flex items-center gap-2">
                <CircleCheckBig className="size-4 text-muted-foreground" />
                <h4 className="font-semibold">確認</h4>
              </div>
              <Badge variant="muted">{confirmation?.count ?? 0}件</Badge>
            </div>
            <CardBody>
              <SectionBody section={confirmation} empty="今日の確認事項はありません。" />
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
