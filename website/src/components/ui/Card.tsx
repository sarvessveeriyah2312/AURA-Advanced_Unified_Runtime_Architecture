import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "../../lib/cn";

export function Card({
  className,
  children,
  interactive = false,
  ...rest
}: HTMLAttributes<HTMLDivElement> & { children: ReactNode; interactive?: boolean }) {
  return (
    <div
      className={cn(
        "rounded-2xl border border-line bg-surface p-6 shadow-card",
        interactive && "transition-colors duration-200 hover:border-line-strong",
        className,
      )}
      {...rest}
    >
      {children}
    </div>
  );
}

export function IconBadge({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span
      className={cn(
        "inline-flex size-10 shrink-0 items-center justify-center rounded-xl border border-line bg-accent-soft text-accent",
        className,
      )}
      aria-hidden="true"
    >
      {children}
    </span>
  );
}

export function Pill({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border border-line bg-surface px-3 py-1 text-xs font-medium text-muted",
        className,
      )}
    >
      {children}
    </span>
  );
}
