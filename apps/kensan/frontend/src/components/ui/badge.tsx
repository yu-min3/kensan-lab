import type { ReactNode } from "react";
import clsx from "clsx";

// components.md 02. Badge
type Variant = "brand" | "success" | "warning" | "destructive" | "muted" | "outline";

const variantClass: Record<Variant, string> = {
  brand: "bg-brand-muted text-accent-foreground",
  success: "bg-success/15 text-success",
  warning: "bg-warning/15 text-warning",
  destructive: "bg-destructive/12 text-destructive",
  muted: "bg-muted text-muted-foreground",
  outline: "border border-border-strong text-muted-foreground",
};

const dotClass: Record<Variant, string> = {
  brand: "bg-brand",
  success: "bg-success",
  warning: "bg-warning",
  destructive: "bg-destructive",
  muted: "bg-muted-foreground",
  outline: "bg-muted-foreground",
};

export function Badge({
  variant = "muted",
  dot,
  children,
  className,
}: {
  variant?: Variant;
  dot?: boolean;
  children: ReactNode;
  className?: string;
}) {
  return (
    <span
      className={clsx(
        "inline-flex items-center gap-1.5 rounded-sm px-2 py-0.5 text-xs font-semibold",
        variantClass[variant],
        className,
      )}
    >
      {dot && <span className={clsx("size-1.5 rounded-full", dotClass[variant])} />}
      {children}
    </span>
  );
}
