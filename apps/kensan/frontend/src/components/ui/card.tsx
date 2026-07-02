import type { ReactNode } from "react";
import clsx from "clsx";

// components.md 05. Card — border のみ・影なし。head / body / foot の3パート
export function Card({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <section className={clsx("bg-card border border-border rounded-lg", className)}>
      {children}
    </section>
  );
}

export function CardHead({
  title,
  sub,
  badge,
  actions,
}: {
  title: string;
  sub?: string;
  badge?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <header className="ds-card flex items-start justify-between gap-3 border-b border-border !pb-3">
      <div className="min-w-0">
        <h3 className="h-serif text-base font-semibold flex items-center gap-2">
          {title}
          {badge}
        </h3>
        {sub && <p className="text-xs text-muted-foreground mt-0.5">{sub}</p>}
      </div>
      {actions && <div className="ds-inline shrink-0">{actions}</div>}
    </header>
  );
}

export function CardBody({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={clsx("ds-card", className)}>{children}</div>;
}

export function CardFoot({ children }: { children: ReactNode }) {
  return <footer className="ds-card border-t border-border !pt-3">{children}</footer>;
}
