import { Check, ChevronDown, FileCode } from "lucide-react";
import { useEffect, useId, useRef, useState, type KeyboardEvent } from "react";
import type { Example } from "../../data/examples";
import { cn } from "../../lib/cn";

export interface ExampleGroup {
  title: string;
  examples: Example[];
}

// ExamplePicker chooses the playground's program: a button that shows the
// current example and opens a grouped list with a one-line description of
// each. It follows the WAI-ARIA listbox pattern: the arrow keys, Home and
// End move, Enter or Space chooses, Escape closes and returns focus.
export function ExamplePicker({
  groups,
  value,
  onChange,
  customLabel = "Your program",
}: {
  groups: ExampleGroup[];
  value: string; // an example id, or "" for the visitor's own program
  onChange: (id: string) => void;
  customLabel?: string;
}) {
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const id = useId();
  const root = useRef<HTMLDivElement>(null);
  const button = useRef<HTMLButtonElement>(null);
  const list = useRef<HTMLUListElement>(null);

  const all = groups.flatMap((g) => g.examples);
  const current = all.find((e) => e.id === value);
  const name = (e: Example) => (e.id === "hero" ? "Hello" : e.label);

  // Close on a click outside the picker.
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (root.current && !root.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  // Keep the active option visible while moving with the keyboard.
  useEffect(() => {
    if (open) document.getElementById(`${id}-opt-${active}`)?.scrollIntoView({ block: "nearest" });
  }, [open, active, id]);

  const show = () => {
    const i = all.findIndex((e) => e.id === value);
    setActive(i < 0 ? 0 : i);
    setOpen(true);
    requestAnimationFrame(() => list.current?.focus());
  };

  const close = () => {
    setOpen(false);
    button.current?.focus();
  };

  const choose = (i: number) => {
    const e = all[i];
    if (e) onChange(e.id);
    close();
  };

  const onButtonKey = (e: KeyboardEvent<HTMLButtonElement>) => {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      show();
    }
  };

  const onListKey = (e: KeyboardEvent<HTMLUListElement>) => {
    const last = all.length - 1;
    switch (e.key) {
      case "ArrowDown":
        setActive((i) => Math.min(last, i + 1));
        break;
      case "ArrowUp":
        setActive((i) => Math.max(0, i - 1));
        break;
      case "Home":
        setActive(0);
        break;
      case "End":
        setActive(last);
        break;
      case "Enter":
      case " ":
        choose(active);
        break;
      case "Escape":
        close();
        break;
      case "Tab":
        setOpen(false);
        return; // let focus move on
      default:
        return;
    }
    e.preventDefault();
  };

  let index = -1;
  return (
    <div ref={root} className="relative">
      <button
        ref={button}
        type="button"
        onClick={() => (open ? setOpen(false) : show())}
        onKeyDown={onButtonKey}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={`${id}-list`}
        className="inline-flex h-10 min-w-[13rem] items-center gap-2.5 rounded-lg border border-line-strong bg-surface pr-2.5 pl-3 text-sm text-fg transition-colors hover:border-accent/60 hover:bg-surface-2"
      >
        <FileCode className="size-4 shrink-0 text-accent" aria-hidden />
        <span className="flex min-w-0 flex-1 items-baseline gap-2 text-left">
          <span className="text-xs text-subtle">Example</span>
          <span className="truncate font-medium">{current ? name(current) : customLabel}</span>
        </span>
        <ChevronDown className={cn("size-4 shrink-0 text-muted transition-transform", open && "rotate-180")} aria-hidden />
      </button>

      {open && (
        <ul
          ref={list}
          id={`${id}-list`}
          role="listbox"
          tabIndex={-1}
          aria-label="Example programs"
          aria-activedescendant={`${id}-opt-${active}`}
          onKeyDown={onListKey}
          className="scrollbar-thin absolute top-full left-0 z-30 mt-2 max-h-[min(28rem,70vh)] w-[min(24rem,calc(100vw-2rem))] overflow-y-auto rounded-xl border border-line bg-surface p-1.5 shadow-card outline-none"
        >
          {groups.map((g) => (
            <li key={g.title} role="presentation">
              <p className="px-2.5 pt-2.5 pb-1 font-mono text-[10.5px] font-medium tracking-[0.16em] text-subtle uppercase" aria-hidden>
                {g.title}
              </p>
              <ul role="group" aria-label={g.title}>
                {g.examples.map((e) => {
                  index++;
                  const i = index;
                  const selected = e.id === value;
                  return (
                    <li
                      key={e.id}
                      id={`${id}-opt-${i}`}
                      role="option"
                      aria-selected={selected}
                      onMouseEnter={() => setActive(i)}
                      onMouseDown={(ev) => ev.preventDefault()} // keep focus in the list
                      onClick={() => choose(i)}
                      className={cn(
                        "flex cursor-pointer items-start gap-2.5 rounded-lg px-2.5 py-2",
                        i === active ? "bg-accent-soft" : "bg-transparent",
                      )}
                    >
                      <Check className={cn("mt-0.5 size-4 shrink-0 text-accent", !selected && "invisible")} aria-hidden />
                      <span className="min-w-0">
                        <span className="flex items-baseline gap-2">
                          <span className="text-sm font-medium text-fg">{name(e)}</span>
                          <span className="truncate font-mono text-[11px] text-subtle">{e.file}</span>
                        </span>
                        <span className="mt-0.5 block text-xs leading-relaxed text-muted">{e.summary}</span>
                      </span>
                    </li>
                  );
                })}
              </ul>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
