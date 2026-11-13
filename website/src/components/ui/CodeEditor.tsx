import { useMemo, useRef, type KeyboardEvent } from "react";
import { highlight } from "../../lib/highlight";
import { cn } from "../../lib/cn";

// CodeEditor is a plain textarea over a highlighted copy of the same text.
// The textarea's own text is transparent, so the visitor sees the colours
// but edits, selects and copies real text with the browser's native
// behaviour. Both layers share font metrics; the outer box scrolls them
// together.
//
// Tab inserts four spaces. Press Escape first to move focus with Tab
// instead. Ctrl+Enter (Cmd+Enter on macOS) calls onRun.
const TYPE = "font-mono text-[13px] leading-6 tracking-normal [tab-size:4]";

export function CodeEditor({
  value,
  onChange,
  onRun,
  label,
  className,
}: {
  value: string;
  onChange: (value: string) => void;
  onRun?: () => void;
  label: string;
  className?: string;
}) {
  const tabMovesFocus = useRef(false);
  const tokens = useMemo(() => highlight(value), [value]);
  const lineCount = value.split("\n").length;

  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
      e.preventDefault();
      onRun?.();
      return;
    }
    if (e.key === "Escape") {
      tabMovesFocus.current = true;
      return;
    }
    if (e.key === "Tab" && !e.shiftKey && !tabMovesFocus.current) {
      e.preventDefault();
      const el = e.currentTarget;
      const { selectionStart: s, selectionEnd: end } = el;
      const next = value.slice(0, s) + "    " + value.slice(end);
      onChange(next);
      requestAnimationFrame(() => el.setSelectionRange(s + 4, s + 4));
      return;
    }
    if (e.key !== "Tab" && e.key !== "Shift") tabMovesFocus.current = false;
  };

  return (
    <div
      className={cn(
        "scrollbar-thin overflow-auto bg-code-bg focus-within:outline-2 focus-within:-outline-offset-2 focus-within:outline-accent/50",
        className,
      )}
    >
      <div className="flex min-h-full min-w-max">
        <div aria-hidden className={cn(TYPE, "sticky left-0 z-10 w-11 shrink-0 bg-code-bg py-4 pr-3 text-right text-code-dim/60 select-none")}>
          {Array.from({ length: lineCount }, (_, i) => (
            <div key={i}>{i + 1}</div>
          ))}
        </div>
        <div className="relative min-w-0 flex-1">
          <pre aria-hidden className={cn(TYPE, "pointer-events-none m-0 py-4 pr-8 pl-1 whitespace-pre text-code-fg")}>
            <code>
              {tokens.map((t, i) => (
                <span key={i} className={t.kind === "plain" ? undefined : `tok-${t.kind}`}>
                  {t.text}
                </span>
              ))}
              {/* Keeps a trailing empty line as tall as the textarea's. */}
              {"\n "}
            </code>
          </pre>
          <textarea
            value={value}
            onChange={(e) => onChange(e.target.value)}
            onKeyDown={onKeyDown}
            aria-label={label}
            aria-describedby="editor-keys"
            spellCheck={false}
            autoCapitalize="off"
            autoComplete="off"
            autoCorrect="off"
            wrap="off"
            className={cn(
              TYPE,
              "absolute inset-0 h-full w-full resize-none overflow-hidden bg-transparent py-4 pr-8 pl-1 whitespace-pre text-transparent caret-code-fg outline-none selection:bg-accent/35",
            )}
          />
        </div>
      </div>
    </div>
  );
}
