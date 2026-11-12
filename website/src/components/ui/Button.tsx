import type { AnchorHTMLAttributes, ButtonHTMLAttributes, ReactNode } from "react";
import { cn } from "../../lib/cn";

type Variant = "primary" | "secondary" | "ghost";
type Size = "sm" | "md" | "lg";

const base =
  "inline-flex items-center justify-center gap-2 rounded-lg font-medium whitespace-nowrap transition-colors duration-200 select-none disabled:opacity-50";

const variants: Record<Variant, string> = {
  primary:
    "bg-accent-2 text-white shadow-[0_8px_24px_-12px_var(--accent-2)] hover:bg-accent dark:bg-accent dark:text-[#0a0716] dark:hover:bg-[#b2a6ff]",
  secondary:
    "border border-line-strong bg-surface text-fg hover:border-accent/60 hover:bg-surface-2",
  ghost: "text-muted hover:text-fg hover:bg-surface-2",
};

const sizes: Record<Size, string> = {
  sm: "h-9 px-3 text-sm",
  md: "h-10 px-4 text-sm",
  lg: "h-12 px-5 text-[15px]",
};

interface Common {
  variant?: Variant;
  size?: Size;
  children: ReactNode;
  className?: string;
}

// ButtonLink renders a real link styled as a button. External links open
// in a new tab with safe rel attributes.
export function ButtonLink({
  variant = "primary",
  size = "md",
  className,
  children,
  href,
  ...rest
}: Common & AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) {
  const external = href.startsWith("http");
  return (
    <a
      href={href}
      className={cn(base, variants[variant], sizes[size], className)}
      {...(external ? { target: "_blank", rel: "noopener noreferrer" } : {})}
      {...rest}
    >
      {children}
    </a>
  );
}

export function Button({
  variant = "primary",
  size = "md",
  className,
  children,
  type = "button",
  ...rest
}: Common & ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button type={type} className={cn(base, variants[variant], sizes[size], className)} {...rest}>
      {children}
    </button>
  );
}
