import { Check, Copy } from "lucide-react";
import { useMemo, type ReactNode } from "react";
import { useCopy } from "../../hooks/useCopy";
import { highlight } from "../../lib/highlight";
import { cn } from "../../lib/cn";

export function CopyButton({ text, label, className }: { text: string; label: string; className?: string }) {
  const { copied, copy } = useCopy();
  return (
    <button
      type="button"
      onClick={() => void copy(text)}
      className={cn(
        "inline-flex h-8 items-center gap-1.5 rounded-md border border-code-line px-2.5 text-xs font-medium text-code-dim transition-colors hover:border-[#2c3462] hover:text-code-fg",
        className,
      )}
      aria-label={copied ? "Copied" : label}
    >
      {copied ? <Check className="size-3.5 text-[var(--tok-string)]" aria-hidden /> : <Copy className="size-3.5" aria-hidden />}
      <span aria-hidden>{copied ? "Copied" : "Copy"}</span>
      <span className="sr-only" role="status" aria-live="polite">
        {copied ? "Copied to clipboard" : ""}
      </span>
    </button>
  );
}

// WindowBar is the top bar of an editor or terminal panel.
export function WindowBar({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="flex h-11 items-center justify-between gap-3 border-b border-code-line bg-code-bar px-4">
      <div className="flex min-w-0 items-center gap-3">
        <span className="flex gap-1.5" aria-hidden>
          <span className="size-2.5 rounded-full bg-[#ff5f57]/80" />
          <span className="size-2.5 rounded-full bg-[#febc2e]/80" />
          <span className="size-2.5 rounded-full bg-[#28c840]/80" />
        </span>
        <span className="truncate font-mono text-xs text-code-dim">{title}</span>
      </div>
      {children}
    </div>
  );
}

// TreadCode renders highlighted Tread source with line numbers.
export function TreadCode({ code, className }: { code: string; className?: string }) {
  const lines = useMemo(() => {
    const tokens = highlight(code);
    // Split tokens into lines so each line can carry a number.
    const result: { kind: string; text: string }[][] = [[]];
    for (const t of tokens) {
      const parts = t.text.split("\n");
      parts.forEach((part, i) => {
        if (i > 0) result.push([]);
        if (part) result[result.length - 1]!.push({ kind: t.kind, text: part });
      });
    }
    return result;
  }, [code]);

  return (
    <pre
      className={cn("scrollbar-thin overflow-x-auto py-4 font-mono text-[13px] leading-6 text-code-fg", className)}
      tabIndex={0}
      aria-label="Tread source code"
    >
      <code className="grid min-w-max">
        {lines.map((line, n) => (
          <span key={n} className="flex">
            <span className="w-10 shrink-0 pr-4 text-right text-code-dim/60 select-none" aria-hidden>
              {n + 1}
            </span>
            <span className="pr-6">
              {line.length === 0
                ? " "
                : line.map((t, i) => (
                    <span key={i} className={t.kind === "plain" ? undefined : `tok-${t.kind}`}>
                      {t.text}
                    </span>
                  ))}
            </span>
          </span>
        ))}
      </code>
    </pre>
  );
}

// TerminalText renders command output. Lines starting with "error" are
// tinted so diagnostics read as diagnostics.
export function TerminalText({ command, output, className }: { command?: string; output: string; className?: string }) {
  return (
    <pre
      className={cn("scrollbar-thin overflow-x-auto px-4 py-4 font-mono text-[13px] leading-6 text-code-fg", className)}
      tabIndex={0}
    >
      <code className="block min-w-max">
        {command && (
          <span className="block">
            <span className="text-[var(--tok-string)] select-none" aria-hidden>
              ${" "}
            </span>
            {command}
          </span>
        )}
        {output.split("\n").map((line, i) => (
          <span
            key={i}
            className={cn(
              "block",
              /^error\[/.test(line) && "text-[var(--tok-error)]",
              /^stack trace:/.test(line) && "text-[var(--tok-interp)]",
            )}
          >
            {line || " "}
          </span>
        ))}
      </code>
    </pre>
  );
}
