import type { ReactNode } from "react";

// Renders the inline Markdown used in CHANGELOG.md (`code`, **bold**,
// *emphasis*, [links](url)) as React elements. No HTML is injected, so
// changelog text can never inject markup.
const TOKEN = /(`[^`]+`|\*\*[^*]+\*\*|\[[^\]]+\]\([^)\s]+\)|\*[^*\s][^*]*\*)/g;

export function InlineMarkdown({ text }: { text: string }) {
  return <>{render(text, "")}</>;
}

function render(text: string, keyPrefix: string): ReactNode[] {
  const out: ReactNode[] = [];
  let last = 0;
  let n = 0;
  for (const match of text.matchAll(TOKEN)) {
    const token = match[0];
    const at = match.index ?? 0;
    if (at > last) out.push(text.slice(last, at));
    const key = `${keyPrefix}${n++}`;
    if (token.startsWith("`")) {
      out.push(
        <code key={key} className="rounded bg-surface-2 px-1.5 py-0.5 font-mono text-[0.86em] text-fg">
          {token.slice(1, -1)}
        </code>,
      );
    } else if (token.startsWith("**")) {
      out.push(
        <strong key={key} className="font-semibold text-fg">
          {render(token.slice(2, -2), key + ".")}
        </strong>,
      );
    } else if (token.startsWith("[")) {
      const link = /^\[([^\]]+)\]\(([^)\s]+)\)$/.exec(token);
      const href = link?.[2] ?? "";
      const external = /^https?:\/\//.test(href);
      out.push(
        <a
          key={key}
          href={href}
          className="text-accent underline-offset-2 hover:underline"
          {...(external ? { target: "_blank", rel: "noopener noreferrer" } : {})}
        >
          {render(link?.[1] ?? "", key + ".")}
          {external && <span className="sr-only"> (opens in a new tab)</span>}
        </a>,
      );
    } else {
      out.push(<em key={key}>{render(token.slice(1, -1), key + ".")}</em>);
    }
    last = at + token.length;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}
