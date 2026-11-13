import { Check, CircleAlert, Link2, LoaderCircle, Play, RotateCcw, ScanSearch, Square } from "lucide-react";
import { useCallback, useEffect, useId, useState, useSyncExternalStore, type KeyboardEvent } from "react";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { CodeEditor } from "../components/ui/CodeEditor";
import { TerminalText } from "../components/ui/CodeBlock";
import { ExamplePicker, type ExampleGroup } from "../components/ui/ExamplePicker";
import { Container } from "../components/ui/SectionHeading";
import { EXAMPLES, HERO_EXAMPLE, type Example } from "../data/examples";
import { VERSION, readmeSection } from "../data/site";
import { useCopy } from "../hooks/useCopy";
import { cn } from "../lib/cn";
import { encodeProgram, sharedProgram, subscribeHash, usePlayground, type PlayResult } from "../lib/playground";

// The examples shown on the home page, verified against the real binary,
// grouped for the example picker.
const PROGRAMS: Example[] = [HERO_EXAMPLE, ...EXAMPLES];
const byId = (...ids: string[]) => ids.map((id) => PROGRAMS.find((p) => p.id === id)).filter((p): p is Example => p !== undefined);
const GROUPS: ExampleGroup[] = [
  { title: "Basics", examples: byId("hero", "variables", "functions", "interpolation") },
  { title: "Control flow", examples: byId("conditionals", "loops", "recursion") },
  { title: "Data", examples: byId("collections", "enums", "generics") },
  { title: "Errors", examples: byId("typing", "traces") },
].map((g) => ({
  ...g,
  // "New in 0.5.0: arrays and ..." reads as "Arrays and ..." in the picker.
  examples: g.examples.map((e) => {
    const s = e.summary.replace(/^New in [\d.]+: /, "");
    return { ...e, summary: s.charAt(0).toUpperCase() + s.slice(1) };
  }),
}));
// Any example missing from a group would be unreachable from the picker.
const grouped = new Set(GROUPS.flatMap((g) => g.examples.map((e) => e.id)));
const UNGROUPED = PROGRAMS.filter((p) => !grouped.has(p.id));
if (UNGROUPED.length > 0) GROUPS.push({ title: "More", examples: UNGROUPED });
const DEFAULT = PROGRAMS.find((p) => p.id === "loops") ?? HERO_EXAMPLE;

type View = "output" | "tokens" | "ast" | "bytecode";

const VIEWS: { id: View; label: string; empty: string }[] = [
  { id: "output", label: "Output", empty: "Press Run to compile and run the program." },
  { id: "tokens", label: "Tokens", empty: "Run or check the program to see the lexer's tokens." },
  { id: "ast", label: "AST", empty: "The syntax tree appears when the program parses without errors." },
  { id: "bytecode", label: "Bytecode", empty: "Bytecode appears when the program compiles without errors." },
];

const STAGES = [
  { n: "1", name: "Lexer", view: "Tokens", text: "Turns the source text into tokens: keywords, names, numbers, strings and operators, each with its line and column." },
  { n: "2", name: "Parser", view: "AST", text: "Builds a syntax tree from the tokens, following the grammar and the newline rules." },
  { n: "3", name: "Type checker", view: "Output", text: "Resolves every name and checks every type. Errors here stop the program before it runs." },
  { n: "4", name: "Compiler", view: "Bytecode", text: "Translates the checked tree into instructions for a stack machine, with typed opcodes." },
  { n: "5", name: "Virtual machine", view: "Output", text: "Runs the bytecode. Runtime errors show the line, the expression and a stack trace." },
];

export function PlaygroundPage() {
  const { status, version, error, run, stop } = usePlayground();
  // A shared link carries its program in the fragment (#code=...). Until
  // the visitor edits or picks an example, the shared program is shown.
  const shared = useSyncExternalStore(subscribeHash, sharedProgram, () => null);
  const [draft, setDraft] = useState<string | null>(null);
  const [picked, setPicked] = useState<string | null>(null);
  const code = draft ?? shared ?? DEFAULT.code + "\n";
  const example = picked ?? (shared !== null && draft === null ? "" : DEFAULT.id);
  const setCode = (next: string) => {
    setDraft(next);
    if (picked === null && shared !== null) setPicked("");
  };
  const [result, setResult] = useState<PlayResult | null>(null);
  const [view, setView] = useState<View>("output");
  const share = useCopy();
  const tabsId = useId();

  // While the compiler is still loading, Run and Check are queued: the
  // worker starts them as soon as it is ready.
  const busy = status === "running" || status === "failed";
  const go = useCallback(
    async (check: boolean) => {
      if (status === "running" || status === "failed") return;
      const r = await run(code, check);
      if (r) setResult(r);
    },
    [code, run, status],
  );

  // Ctrl/⌘ + Enter runs the program from anywhere on the page, not only
  // from inside the editor (which handles it itself).
  useEffect(() => {
    const onKey = (e: globalThis.KeyboardEvent) => {
      if (e.defaultPrevented || e.key !== "Enter" || !(e.metaKey || e.ctrlKey)) return;
      e.preventDefault();
      void go(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [go]);

  const choose = (id: string) => {
    const p = PROGRAMS.find((x) => x.id === id);
    if (!p) return;
    setPicked(id);
    setDraft(p.code + "\n");
    setResult(null);
    setView("output");
  };

  const shareLink = () => {
    const url = `${window.location.origin}${window.location.pathname}#code=${encodeProgram(code)}`;
    window.history.replaceState(null, "", url);
    void share.copy(url);
  };

  const onTabKey = (e: KeyboardEvent<HTMLButtonElement>) => {
    if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
    e.preventDefault();
    const i = VIEWS.findIndex((v) => v.id === view);
    const next = VIEWS[(i + (e.key === "ArrowRight" ? 1 : VIEWS.length - 1)) % VIEWS.length]!.id;
    setView(next);
    document.getElementById(`${tabsId}-${next}`)?.focus();
  };

  const shown = view === "output" ? (result ? result.stdout + result.stderr : "") : result ? result[view] : "";
  const placeholder =
    view === "output" && result && result.check && result.exit === 0
      ? "No errors. (Check does not run the program, so there is no output.)"
      : view === "output" && result && shown === ""
        ? "The program printed nothing."
        : VIEWS.find((v) => v.id === view)!.empty;

  return (
    <>
      <section aria-labelledby="playground-title" className="border-b border-line">
        <Container className="pt-14 pb-10 sm:pt-20">
          <p className="font-mono text-xs font-medium tracking-[0.18em] text-accent uppercase">Playground · v{VERSION}</p>
          <h1 id="playground-title" className="mt-3 text-4xl font-semibold tracking-tight sm:text-5xl">
            Try Tread in your browser
          </h1>
          <p className="mt-4 max-w-2xl text-base leading-relaxed text-pretty text-muted sm:text-lg">
            This is the real Tread compiler and virtual machine, compiled to WebAssembly. Write a program, run it, and look
            inside: the tokens, the syntax tree and the bytecode. Nothing is installed, and your code never leaves your
            browser.
          </p>
        </Container>
      </section>

      <Container className="py-10">
        <div className="flex flex-wrap items-center gap-2">
          <ExamplePicker groups={GROUPS} value={example} onChange={choose} />
          <div className="ml-auto flex flex-wrap items-center gap-2">
            <Button variant="ghost" size="md" onClick={shareLink} aria-label={share.copied ? "Link copied" : "Copy a link to this program"}>
              {share.copied ? <Check className="size-4" aria-hidden /> : <Link2 className="size-4" aria-hidden />}
              {share.copied ? "Link copied" : "Share"}
            </Button>
            <Button variant="secondary" size="md" onClick={() => void go(true)} disabled={busy}>
              <ScanSearch className="size-4" aria-hidden />
              Check
            </Button>
            {status === "running" ? (
              <Button size="md" onClick={stop}>
                <Square className="size-4" aria-hidden />
                Stop
              </Button>
            ) : (
              <Button size="md" onClick={() => void go(false)} disabled={busy}>
                {status === "loading" ? <LoaderCircle className="size-4 animate-spin" aria-hidden /> : <Play className="size-4" aria-hidden />}
                Run
              </Button>
            )}
          </div>
        </div>

        <div className="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-2">
          <div className="min-w-0 overflow-hidden rounded-2xl border border-code-line bg-code-bg">
            <div className="flex h-11 items-center justify-between gap-3 border-b border-code-line bg-code-bar px-4">
              <span className="font-mono text-xs text-code-dim">main.tr</span>
              <span id="editor-keys" className="hidden font-mono text-[11px] text-code-dim sm:inline">
                Ctrl/⌘ + Enter runs · Esc, then Tab, leaves the editor
              </span>
            </div>
            <CodeEditor value={code} onChange={setCode} onRun={() => void go(false)} label="Tread program, main.tr" className="h-[26rem] lg:h-[32rem]" />
          </div>

          <div className="min-w-0 overflow-hidden rounded-2xl border border-code-line bg-code-bg">
            <div className="flex h-11 items-center justify-between gap-2 border-b border-code-line bg-code-bar pr-2 pl-2">
              <div role="tablist" aria-label="Program output and compiler stages" className="flex min-w-0 items-center gap-0.5">
                {VIEWS.map((v) => {
                  const active = v.id === view;
                  return (
                    <button
                      key={v.id}
                      id={`${tabsId}-${v.id}`}
                      type="button"
                      role="tab"
                      aria-selected={active}
                      aria-controls={`${tabsId}-panel`}
                      tabIndex={active ? 0 : -1}
                      onClick={() => setView(v.id)}
                      onKeyDown={onTabKey}
                      className={cn(
                        "relative h-8 shrink-0 rounded-lg px-2 font-mono text-xs transition-colors sm:px-3",
                        active ? "bg-white/[0.06] text-code-fg" : "text-code-dim hover:text-code-fg",
                      )}
                    >
                      {v.label}
                      {active && (
                        <span className="absolute -bottom-[7px] left-1/2 h-0.5 w-7 -translate-x-1/2 rounded-full bg-[var(--tok-string)]" aria-hidden />
                      )}
                    </button>
                  );
                })}
              </div>
              <Status status={status} result={result} version={version} />
            </div>
            <div id={`${tabsId}-panel`} role="tabpanel" aria-labelledby={`${tabsId}-${view}`} aria-live="polite" className="h-[26rem] overflow-auto lg:h-[32rem]">
              {status === "failed" ? (
                <p className="flex items-start gap-2 p-4 text-sm text-[var(--tok-error)]">
                  <CircleAlert className="mt-0.5 size-4 shrink-0" aria-hidden />
                  The playground could not start: {error}. It needs a browser with WebAssembly and Web Workers.
                </p>
              ) : shown ? (
                view === "output" ? (
                  <TerminalText output={shown.replace(/\n$/, "")} className="min-h-full" />
                ) : (
                  <pre className="scrollbar-thin min-h-full overflow-x-auto p-4 font-mono text-[12.5px] leading-6 text-code-fg" tabIndex={0}>
                    {shown}
                  </pre>
                )
              ) : (
                <p className="p-4 font-mono text-[13px] text-code-dim">{status === "loading" && !result ? "Loading the Tread compiler…" : placeholder}</p>
              )}
            </div>
          </div>
        </div>

        <p className="mt-3 flex items-center gap-2 text-xs text-subtle">
          <RotateCcw className="size-3.5" aria-hidden />
          A run stops after 50 million instructions (error TR-R004) or 1 MiB of output, so an endless loop cannot freeze the
          page. Press Stop to end a run early.
        </p>

        <section aria-labelledby="how-title" className="mt-14">
          <h2 id="how-title" className="text-2xl font-semibold tracking-tight">
            What happens when you press Run
          </h2>
          <p className="mt-2 max-w-3xl text-sm leading-relaxed text-muted">
            Tread is built to be read one stage at a time. Each stage is its own Go package; the tabs above show what three of
            them produce. The same code runs in the <code className="font-mono text-[13px]">tread</code> command, so the
            output here is exactly what <code className="font-mono text-[13px]">tread run main.tr</code> prints.
          </p>
          <ol className="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-5">
            {STAGES.map((s) => (
              <li key={s.n}>
                <Card className="h-full">
                  <p className="font-mono text-xs text-accent">
                    {s.n} · {s.view}
                  </p>
                  <h3 className="mt-2 font-semibold">{s.name}</h3>
                  <p className="mt-1.5 text-sm leading-relaxed text-muted">{s.text}</p>
                </Card>
              </li>
            ))}
          </ol>
          <p className="mt-6 text-sm text-muted">
            Want it on your machine?{" "}
            <a href="/download/" className="font-medium text-accent hover:underline">
              Download Tread
            </a>{" "}
            or read the{" "}
            <a href={readmeSection("language-tutorial")} className="font-medium text-accent hover:underline">
              language tutorial
            </a>
            .
          </p>
        </section>
      </Container>
    </>
  );
}

function Status({ status, result, version }: { status: string; result: PlayResult | null; version: string | null }) {
  let text = "";
  let tone = "text-code-dim";
  if (status === "loading") text = "loading…";
  else if (status === "running") text = "running…";
  else if (result) {
    text = `exit ${result.exit} · ${result.ms < 10 ? result.ms.toFixed(1) : Math.round(result.ms)} ms`;
    tone = result.exit === 0 ? "text-[var(--tok-string)]" : "text-[var(--tok-error)]";
  } else if (version) text = `tread ${version}`;
  return <span className={cn("shrink-0 pr-2 font-mono text-[11px] whitespace-nowrap", tone)}>{text}</span>;
}
