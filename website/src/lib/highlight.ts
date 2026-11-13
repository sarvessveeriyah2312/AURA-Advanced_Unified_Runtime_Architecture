// A small, dependency-free highlighter for Tread 0.x source. It knows the
// language's real keywords, built-in types and functions, comments,
// numbers, and strings with ${...} interpolation (highlighted recursively).

export type TokenKind =
  | "keyword"
  | "type"
  | "builtin"
  | "function"
  | "string"
  | "interp"
  | "number"
  | "comment"
  | "punct"
  | "plain";

export interface Token {
  kind: TokenKind;
  text: string;
}

const KEYWORDS = new Set([
  "fn",
  "let",
  "mut",
  "if",
  "else",
  "return",
  "module",
  "true",
  "false",
  "while",
  "for",
  "in",
  "break",
  "continue",
  "enum",
  "match",
]);
const TYPES = new Set(["Int", "Float", "Bool", "String", "Unit"]);
const BUILTINS = new Set(["print", "toString"]);
const PUNCT = new Set(["{", "}", "(", ")", ",", ":"]);

const isIdentStart = (c: string) => /[A-Za-z_]/.test(c);
const isIdent = (c: string) => /[A-Za-z0-9_]/.test(c);
const isDigit = (c: string) => /[0-9]/.test(c);

export function highlight(src: string): Token[] {
  const out: Token[] = [];
  const push = (kind: TokenKind, text: string) => {
    if (!text) return;
    const last = out[out.length - 1];
    if (last && last.kind === kind) last.text += text;
    else out.push({ kind, text });
  };

  let i = 0;
  while (i < src.length) {
    const c = src[i] ?? "";
    const next = src[i + 1] ?? "";

    if (c === "/" && next === "/") {
      const end = src.indexOf("\n", i);
      const stop = end === -1 ? src.length : end;
      push("comment", src.slice(i, stop));
      i = stop;
    } else if (c === "/" && next === "*") {
      const end = src.indexOf("*/", i + 2);
      const stop = end === -1 ? src.length : end + 2;
      push("comment", src.slice(i, stop));
      i = stop;
    } else if (c === '"') {
      i = scanString(src, i, push);
    } else if (isDigit(c)) {
      let j = i;
      // A number stops before "..", so 0..10 is a range (ADR-0012).
      while (
        j < src.length &&
        /[0-9.]/.test(src[j] ?? "") &&
        !(src[j] === "." && src[j + 1] === ".")
      )
        j++;
      push("number", src.slice(i, j));
      i = j;
    } else if (isIdentStart(c)) {
      let j = i;
      while (j < src.length && isIdent(src[j] ?? "")) j++;
      const word = src.slice(i, j);
      let k = j;
      while (src[k] === " ") k++;
      if (KEYWORDS.has(word)) push("keyword", word);
      else if (TYPES.has(word)) push("type", word);
      else if (BUILTINS.has(word)) push("builtin", word);
      else if (src[k] === "(") push("function", word);
      else push("plain", word);
      i = j;
    } else if (PUNCT.has(c)) {
      push("punct", c);
      i++;
    } else {
      push("plain", c);
      i++;
    }
  }
  return out;
}

// scanString highlights a string literal starting at src[start] === '"'
// and returns the index just past it. Interpolated expressions are
// highlighted as code; strings are single-line in Tread.
function scanString(
  src: string,
  start: number,
  push: (k: TokenKind, t: string) => void,
): number {
  let i = start + 1;
  let chunk = '"';
  while (i < src.length) {
    const c = src[i] ?? "";
    if (c === "\n") break;
    if (c === "\\") {
      chunk += src.slice(i, i + 2);
      i += 2;
      continue;
    }
    if (c === "$" && src[i + 1] === "{") {
      push("string", chunk);
      chunk = "";
      let depth = 1;
      let j = i + 2;
      while (j < src.length && depth > 0 && src[j] !== "\n") {
        if (src[j] === "{") depth++;
        else if (src[j] === "}") depth--;
        if (depth > 0) j++;
      }
      push("interp", "${");
      for (const t of highlight(src.slice(i + 2, j))) push(t.kind, t.text);
      if (src[j] === "}") {
        push("interp", "}");
        j++;
      }
      i = j;
      continue;
    }
    chunk += c;
    i++;
    if (c === '"') {
      push("string", chunk);
      return i;
    }
  }
  push("string", chunk);
  return i;
}
