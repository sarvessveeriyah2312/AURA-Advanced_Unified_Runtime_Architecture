import { RELEASED, VERSION } from "./site";

// Questions developers ask about Tread. Each answer stands on its own
// (it is also emitted as FAQPage structured data), and every claim is
// checked against the repository. Keep answers short and factual.
export interface Faq {
  question: string;
  answer: string;
}

export const FAQS: Faq[] = [
  {
    question: "What is Tread?",
    answer:
      "Tread is a small, statically typed programming language built to teach how programming languages work. Its toolchain, written in Go, is a lexer, parser, type checker, bytecode compiler, and stack-based virtual machine, each a separate package small enough to read in one sitting. You write programs in .tr files and run them with the tread command.",
  },
  {
    question: "Who is Tread for?",
    answer:
      "Tread is for developers and students who want to understand a language from the inside: how source text becomes tokens, a syntax tree, checked types, bytecode, and finally a running program. It is also a small, predictable language for experimenting with statically typed code.",
  },
  {
    question: "Is Tread ready for production use?",
    answer:
      "No. Tread 0.x is experimental, and the language may change between minor versions. It has no collections, user-defined types, imports, or file I/O yet. It is thoroughly tested, with CI running the full test suite on Linux, macOS, and Windows, but it is meant for learning and experimenting.",
  },
  {
    question: "Do I need Go to use Tread?",
    answer: RELEASED
      ? `Only to build Tread from source, which needs Go 1.25 or later. The precompiled Tread ${VERSION} archives for macOS, Linux, and Windows do not need Go: each contains a single, self-contained tread executable.`
      : `Today, yes: Tread ${VERSION} is not published yet, so you build it from source with Go 1.25 or later. Once ${VERSION} is released, its precompiled archives for macOS, Linux, and Windows will not need Go: each contains a single, self-contained tread executable.`,
  },
  {
    question: "Which platforms does Tread support?",
    answer:
      "macOS 12 or later on Apple silicon and Intel, Linux with kernel 3.2 or later on x86-64 and ARM64, and Windows 10 or Windows Server 2016 or later on x86-64. Each release archive is tested by running it on its own platform in CI.",
  },
  {
    question: "Is Tread free and open source?",
    answer:
      "Yes. Tread is free and open source under the MIT License. Its source code, issue tracker, and releases are on GitHub, and it has no dependencies beyond the Go toolchain used to build it.",
  },
  {
    question: "Does Tread compile to native code?",
    answer:
      "No. The tread run command compiles a program to Tread bytecode in memory and executes it on the Tread virtual machine every time. Tread does not produce native executables or saved bytecode files.",
  },
  {
    question: "What does a Tread error message look like?",
    answer:
      "Every error has a stable code such as TR-T001, the file, line, and column, and the source line with the problem underlined. Several compile-time errors are reported in one run. Since 0.2.0, a runtime error inside a function also prints a stack trace of the calls that led to it.",
  },
];
