import { type ButtonHTMLAttributes, forwardRef } from "react";
import { Loader2 } from "lucide-react";
import clsx from "clsx";

// components.md 01. Button — variant / size はここに列挙されたものだけ
type Variant = "primary" | "secondary" | "outline" | "ghost" | "destructive" | "link";
type Size = "sm" | "md" | "lg";

const variantClass: Record<Variant, string> = {
  // foreground も必ず semantic トークンで（dark mode では brand/destructive の
  // 上に乗る文字色が暗色に反転する設計 — text-white 固定は NG）
  primary: "bg-brand text-brand-foreground hover:opacity-90",
  secondary: "bg-muted text-foreground hover:bg-accent",
  outline: "border border-border-strong bg-transparent hover:bg-accent",
  ghost: "bg-transparent hover:bg-accent",
  destructive: "bg-destructive text-destructive-foreground hover:opacity-90",
  link: "bg-transparent text-brand underline-offset-4 hover:underline p-0 h-auto",
};

const sizeClass: Record<Size, string> = {
  sm: "text-xs px-2.5 gap-1.5",
  md: "text-sm px-3.5 gap-2",
  lg: "text-base px-5 gap-2",
};

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
  iconOnly?: boolean;
  loading?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = "secondary", size = "md", iconOnly, loading, className, children, disabled, ...rest }, ref) => (
    <button
      ref={ref}
      disabled={disabled || loading}
      className={clsx(
        "ds-control inline-flex items-center justify-center rounded-lg font-medium transition-colors duration-fast",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50 disabled:pointer-events-none",
        variantClass[variant],
        variant !== "link" && sizeClass[size],
        iconOnly && "aspect-square px-0",
        className,
      )}
      {...rest}
    >
      {loading && <Loader2 size={size === "sm" ? 14 : size === "lg" ? 18 : 16} className="animate-spin" />}
      {children}
    </button>
  ),
);
Button.displayName = "Button";
