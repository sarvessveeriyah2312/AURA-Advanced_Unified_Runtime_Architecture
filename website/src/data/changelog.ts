// The changelog page is generated from the repository's CHANGELOG.md at
// build time, so it can never drift from it. This parser handles exactly
// the Markdown subset that file uses: release headings ("## [x.y.z] -
// date"), section headings ("### ..."), paragraphs, and two-level "-"
// lists whose text may wrap onto indented continuation lines.
import source from "../../../CHANGELOG.md?raw";

export interface ListItem {
  text: string;
  children: ListItem[];
}

export type Block = { type: "p"; text: string } | { type: "ul"; items: ListItem[] };

export interface Section {
  title: string;
  blocks: Block[];
}

export interface Release {
  version: string;
  date: string | null; // ISO date, or null while unreleased
  id: string; // anchor id, e.g. "v0-3-0"
  intro: Block[];
  sections: Section[];
}

export interface Changelog {
  intro: Block[];
  releases: Release[];
}

const LIST_ITEM = /^( *)- {3}(.*)$/;

function parseBlocks(lines: string[]): Block[] {
  const blocks: Block[] = [];
  let paragraph: string[] = [];
  let list: ListItem[] | null = null;
  // stack[level] is the item currently open at that nesting level.
  let stack: ListItem[] = [];

  const flushParagraph = () => {
    if (paragraph.length) blocks.push({ type: "p", text: paragraph.join(" ") });
    paragraph = [];
  };
  const flushList = () => {
    if (list) blocks.push({ type: "ul", items: list });
    list = null;
    stack = [];
  };

  for (const raw of lines) {
    const line = raw.replace(/\s+$/, "");
    if (!line.trim()) {
      flushParagraph();
      continue;
    }
    const item = LIST_ITEM.exec(line);
    if (item) {
      flushParagraph();
      const level = Math.floor((item[1] ?? "").length / 4);
      const node: ListItem = { text: (item[2] ?? "").trim(), children: [] };
      if (!list) list = [];
      if (level === 0 || !stack[level - 1]) {
        list.push(node);
        stack = [node];
      } else {
        stack[level - 1]!.children.push(node);
        stack = stack.slice(0, level);
        stack[level] = node;
      }
      continue;
    }
    if (list && /^ {4}/.test(line)) {
      // Continuation of the deepest open list item.
      const open = stack[stack.length - 1];
      if (open) {
        open.text += " " + line.trim();
        continue;
      }
    }
    flushList();
    paragraph.push(line.trim());
  }
  flushParagraph();
  flushList();
  return blocks;
}

export function parseChangelog(markdown: string): Changelog {
  const lines = markdown.replace(/\r\n?/g, "\n").split("\n");
  const releases: Release[] = [];
  const intro: string[] = [];
  let current: { release: Release; intro: string[]; section: { title: string; lines: string[] } | null; sections: { title: string; lines: string[] }[] } | null = null;

  const finish = () => {
    if (!current) return;
    current.release.intro = parseBlocks(current.intro);
    current.release.sections = current.sections.map((s) => ({ title: s.title, blocks: parseBlocks(s.lines) }));
    releases.push(current.release);
  };

  for (const line of lines) {
    const release = /^## \[([^\]]+)\]\s*-\s*(.+)$/.exec(line);
    if (release) {
      finish();
      const version = release[1] ?? "";
      const when = (release[2] ?? "").trim();
      current = {
        release: {
          version,
          date: /^\d{4}-\d{2}-\d{2}$/.test(when) ? when : null,
          id: "v" + version.replace(/[^0-9A-Za-z]+/g, "-"),
          intro: [],
          sections: [],
        },
        intro: [],
        section: null,
        sections: [],
      };
      continue;
    }
    if (line.startsWith("# ")) continue; // the document title
    const section = /^### (.+)$/.exec(line);
    if (section && current) {
      current.section = { title: (section[1] ?? "").trim(), lines: [] };
      current.sections.push(current.section);
      continue;
    }
    if (!current) intro.push(line);
    else if (current.section) current.section.lines.push(line);
    else current.intro.push(line);
  }
  finish();
  return { intro: parseBlocks(intro), releases };
}

export const CHANGELOG = parseChangelog(source);
