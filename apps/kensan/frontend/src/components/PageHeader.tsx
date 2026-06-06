import type { ReactNode } from "react";

// patterns.md 01. Page Header — eyebrow / title / subtext / actions の必須4層
export function PageHeader({
  eyebrow,
  title,
  sub,
  actions,
}: {
  eyebrow: string;
  title: string;
  sub: string;
  actions?: ReactNode;
}) {
  return (
    <header className="flex items-end justify-between gap-4 pb-4 mb-6 border-b border-border">
      <div className="min-w-0">
        <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
          {eyebrow}
        </div>
        <h2 className="h-serif text-2xl font-bold mt-1">{title}</h2>
        <p className="text-[13px] text-muted-foreground mt-1 max-w-2xl">{sub}</p>
      </div>
      {actions && <div className="ds-inline shrink-0">{actions}</div>}
    </header>
  );
}
