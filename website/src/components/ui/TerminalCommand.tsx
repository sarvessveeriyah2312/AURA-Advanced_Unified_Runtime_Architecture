import { Check, Copy, SquareTerminal } from "lucide-react";
import { useId, useState, type KeyboardEvent } from "react";
import { useCopy } from "../../hooks/useCopy";
import { cn } from "../../lib/cn";

type Tab = "command" | "prompt";

const TABS: { id: Tab; label: string }[] = [
  { id: "command", label: "Command" },
  { id: "prompt", label: "Prompt for AI" },
];

// TerminalCommand shows a terminal command with a "Prompt for AI" tab: a
// ready-to-paste prompt (src/lib/aiPrompt.ts) asking an AI assistant to
// run the command. The copy button copies whichever tab is shown.
export function TerminalCommand({
  command,
  prompt,
  label,
  title = "Terminal",
  sign = "$",
  className,
}: {
  command: string; // one command per line
  prompt: string;
  label: string; // what the command does, for screen readers: "the macOS SDK install command"
  title?: string;
  sign?: string; // the shell prompt shown before each line
  className?: string;
}) {
  const [tab, setTab] = useState<Tab>("command");
  const { copied, copy } = useCopy();
  const id = useId();
  const text = tab === "command" ? command : prompt;

  // Arrow keys move between the two tabs (WAI-ARIA tabs pattern).
  const onKeyDown = (e: KeyboardEvent<HTMLButtonElement>) => {
    if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
    e.preventDefault();
    const next: Tab = tab === "command" ? "prompt" : "command";
    setTab(next);
    document.getElementById(`${id}-tab-${next}`)?.focus();
  };

  return (
    <div className={cn("overflow-hidden rounded-2xl border border-code-line bg-code-bg", className)}>
      <div className="flex h-12 items-center justify-between gap-3 border-b border-code-line bg-code-bar pr-2 pl-4">
        <span className="flex min-w-0 items-center gap-2.5">
          <SquareTerminal className="size-4 shrink-0 text-[var(--tok-string)]" aria-hidden />
          <span className="hidden truncate text-[13px] text-code-dim sm:inline">{title}</span>
        </span>
        <div role="tablist" aria-label={`Show ${label} or an AI prompt`} className="flex items-center gap-1">
          {TABS.map((t) => {
            const active = t.id === tab;
            return (
              <button
                key={t.id}
                id={`${id}-tab-${t.id}`}
                type="button"
                role="tab"
                aria-selected={active}
                aria-controls={`${id}-panel`}
                tabIndex={active ? 0 : -1}
                onClick={() => setTab(t.id)}
                onKeyDown={onKeyDown}
                className={cn(
                  "relative h-8 rounded-lg px-3 font-mono text-xs whitespace-nowrap transition-colors",
                  active ? "bg-white/[0.06] text-code-fg" : "text-code-dim hover:text-code-fg",
                )}
              >
                {t.label}
                {active && (
                  <span
                    className="absolute -bottom-[9px] left-1/2 h-0.5 w-7 -translate-x-1/2 rounded-full bg-[var(--tok-string)]"
                    aria-hidden
                  />
                )}
              </button>
            );
          })}
        </div>
      </div>

      <div
        id={`${id}-panel`}
        role="tabpanel"
        aria-labelledby={`${id}-tab-${tab}`}
        className="flex items-start gap-3 py-3 pr-3 pl-4"
      >
        {tab === "command" ? (
          <pre className="scrollbar-thin min-w-0 flex-1 overflow-x-auto py-1.5 font-mono text-[13px] leading-6 text-code-fg" tabIndex={0}>
            <code className="block min-w-max">
              {command.split("\n").map((line, i) => (
                <span key={i} className="block">
                  <span className="text-[var(--tok-string)] select-none" aria-hidden>
                    {sign}{" "}
                  </span>
                  {line}
                </span>
              ))}
            </code>
          </pre>
        ) : (
          <pre
            className="scrollbar-thin max-h-60 min-w-0 flex-1 overflow-y-auto py-1.5 font-sans text-[13px] leading-relaxed whitespace-pre-wrap text-code-fg"
            tabIndex={0}
          >
            {prompt}
          </pre>
        )}
        <button
          type="button"
          onClick={() => void copy(text)}
          className="inline-flex size-10 shrink-0 items-center justify-center rounded-xl border border-code-line bg-white/[0.03] text-code-dim transition-colors hover:border-[#2c3462] hover:text-code-fg"
          aria-label={copied ? "Copied" : tab === "command" ? `Copy ${label}` : `Copy an AI prompt for ${label}`}
          title={tab === "command" ? "Copy command" : "Copy prompt"}
        >
          {copied ? <Check className="size-4 text-[var(--tok-string)]" aria-hidden /> : <Copy className="size-4" aria-hidden />}
          <span className="sr-only" role="status" aria-live="polite">
            {copied ? "Copied to clipboard" : ""}
          </span>
        </button>
      </div>
    </div>
  );
}
