// AURA programs shown on the site. Every program and its output was run
// with the released `aura` 0.2.0 binary; `output` is copied verbatim.
// Keep this file free of imports so the verification script can read it.

export interface Example {
  id: string;
  label: string;
  file: string;
  command: "run" | "check";
  summary: string;
  code: string;
  output: string;
  exitCode: number;
}

export const HERO_EXAMPLE: Example = {
  id: "hero",
  label: "greet.aura",
  file: "greet.aura",
  command: "run",
  summary: "A function, a typed parameter, and string interpolation.",
  code: `fn greet(name: String) -> String {
    return "Hello, \${name}!"
}

fn main() -> Unit {
    let message = greet("AURA")
    print(message)
}`,
  output: `Hello, AURA!`,
  exitCode: 0,
};

export const EXAMPLES: Example[] = [
  {
    id: "variables",
    label: "Variables",
    file: "variables.aura",
    command: "run",
    summary: "Bindings are immutable by default. Use let mut for values that change.",
    code: `fn main() -> Unit {
    let language = "AURA"        // immutable
    let mut visits = 0           // mutable
    visits = visits + 1
    visits = visits + 1
    print(language)
    print(visits)
}`,
    output: `AURA
2`,
    exitCode: 0,
  },
  {
    id: "functions",
    label: "Functions",
    file: "functions.aura",
    command: "run",
    summary: "Typed parameters and explicit return types. Functions can be declared in any order.",
    code: `fn main() -> Unit {
    print(square(9))
    print(greet("Ada"))
}

fn square(n: Int) -> Int {
    return n * n
}

fn greet(name: String) -> String {
    return "Hello, \${name}!"
}`,
    output: `81
Hello, Ada!`,
    exitCode: 0,
  },
  {
    id: "typing",
    label: "Static typing",
    file: "mixed.aura",
    command: "check",
    summary: "Types are checked before anything runs. Int and Float never mix implicitly.",
    code: `fn main() -> Unit {
    let count: Int = 3
    let price = 2.5              // inferred as Float
    print(count * price)
}`,
    output: `error[AURA-T007]: operator '*' cannot be applied to Int and Float
  --> mixed.aura:4:11
   |
 4 |     print(count * price)
   |           ^^^^^^^^^^^^^`,
    exitCode: 1,
  },
  {
    id: "conditionals",
    label: "Conditionals",
    file: "grade.aura",
    command: "run",
    summary: "if, else if, and else. Every condition must be a Bool.",
    code: `fn main() -> Unit {
    grade(91)
    grade(72)
    grade(40)
}

fn grade(score: Int) -> Unit {
    if score >= 80 {
        print("\${score}: A")
    } else if score >= 60 {
        print("\${score}: B")
    } else {
        print("\${score}: try again")
    }
}`,
    output: `91: A
72: B
40: try again`,
    exitCode: 0,
  },
  {
    id: "interpolation",
    label: "Interpolation",
    file: "order.aura",
    command: "run",
    summary: "${...} embeds any Int, Float, Bool, or String expression in a string.",
    code: `fn main() -> Unit {
    let item = "coffee"
    let price = 4
    let qty = 3
    print("\${qty} x \${item} = \${price * qty}")
    print("in stock: \${qty > 0}")
}`,
    output: `3 x coffee = 12
in stock: true`,
    exitCode: 0,
  },
  {
    id: "recursion",
    label: "Recursion",
    file: "factorial.aura",
    command: "run",
    summary: "Repetition uses recursion in AURA today. Int arithmetic is overflow-checked.",
    code: `fn main() -> Unit {
    print(factorial(10))
    print(factorial(20))
}

fn factorial(n: Int) -> Int {
    if n <= 1 {
        return 1
    }
    return n * factorial(n - 1)
}`,
    output: `3628800
2432902008176640000`,
    exitCode: 0,
  },
  {
    id: "traces",
    label: "Stack traces",
    file: "trace.aura",
    command: "run",
    summary: "New in 0.2.0: a runtime error inside a function shows every call that led to it.",
    code: `fn main() -> Unit {
    print(average(10, 0))
}

fn average(total: Int, count: Int) -> Int {
    return divide(total, count)
}

fn divide(a: Int, b: Int) -> Int {
    return a / b
}`,
    output: `error[AURA-R001]: division by zero (in function 'divide')
   --> trace.aura:10:12
    |
 10 |     return a / b
    |            ^^^^^

stack trace:
  0: divide   trace.aura:10:12  return a / b
  1: average  trace.aura:6:12   return divide(total, count)
  2: main     trace.aura:2:11   print(average(10, 0))`,
    exitCode: 1,
  },
];

// Real output of the CLI commands (aura 0.2.0).
export const CLI_HELP = `Usage: aura <command> [arguments]

Commands:
  run <file.aura>     Check, compile and run a program
  check <file.aura>   Check a program for errors without running it
  version             Print the AURA version
  help                Print this help

Flags:
  -h, --help     Print this help
  --version      Print the AURA version`;
