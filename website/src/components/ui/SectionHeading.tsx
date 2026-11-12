import type { ReactNode } from "react";
import { cn } from "../../lib/cn";
import { Reveal } from "./Reveal";

export function SectionHeading({
  eyebrow,
  title,
  description,
  id,
  align = "left",
}: {
  eyebrow: string;
  title: ReactNode;
  description?: ReactNode;
  id?: string;
  align?: "left" | "center";
}) {
  return (
    <Reveal className={cn("max-w-2xl", align === "center" && "mx-auto text-center")}>
      <p className="font-mono text-xs font-medium tracking-[0.18em] text-accent uppercase">{eyebrow}</p>
      <h2 id={id} className="mt-3 text-3xl font-semibold tracking-tight text-balance sm:text-4xl">
        {title}
      </h2>
      {description && <p className="mt-4 text-base leading-relaxed text-pretty text-muted sm:text-lg">{description}</p>}
    </Reveal>
  );
}

export function Container({ children, className }: { children: ReactNode; className?: string }) {
  // Full width: content spans the viewport, with side padding that grows
  // on larger screens. Text blocks keep their own reading-width limits.
  return <div className={cn("w-full px-4 sm:px-6 lg:px-10 xl:px-16 2xl:px-24", className)}>{children}</div>;
}

export function Section({
  id,
  labelledBy,
  children,
  className,
}: {
  id: string;
  labelledBy: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section id={id} aria-labelledby={labelledBy} className={cn("py-20 sm:py-28", className)}>
      <Container>{children}</Container>
    </section>
  );
}
