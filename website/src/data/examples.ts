// Tread programs shown on the site. Every program and its output was run
// with the `tread` 0.5.0 binary; `output` is copied verbatim.
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
  label: "greet.tr",
  file: "greet.tr",
  command: "run",
  summary: "A function, a typed parameter, and string interpolation.",
  code: `fn greet(name: String) -> String {
    return "Hello, \${name}!"
}

fn main() -> Unit {
    let message = greet("Tread")
    print(message)
}`,
  output: `Hello, Tread!`,
  exitCode: 0,
};

export const EXAMPLES: Example[] = [
  {
    id: "variables",
    label: "Variables",
    file: "variables.tr",
    command: "run",
    summary: "Bindings are immutable by default. Use let mut for values that change.",
    code: `fn main() -> Unit {
    let language = "Tread"        // immutable
    let mut visits = 0           // mutable
    visits = visits + 1
    visits = visits + 1
    print(language)
    print(visits)
}`,
    output: `Tread
2`,
    exitCode: 0,
  },
  {
    id: "functions",
    label: "Functions",
    file: "functions.tr",
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
    file: "mixed.tr",
    command: "check",
    summary: "Types are checked before anything runs. Int and Float never mix implicitly.",
    code: `fn main() -> Unit {
    let count: Int = 3
    let price = 2.5              // inferred as Float
    print(count * price)
}`,
    output: `error[TR-T007]: operator '*' cannot be applied to Int and Float
  --> mixed.tr:4:11
   |
 4 |     print(count * price)
   |           ^^^^^^^^^^^^^`,
    exitCode: 1,
  },
  {
    id: "conditionals",
    label: "Conditionals",
    file: "grade.tr",
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
    file: "order.tr",
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
    file: "factorial.tr",
    command: "run",
    summary: "Functions may call themselves. Int arithmetic is overflow-checked.",
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
    id: "loops",
    label: "Loops",
    file: "loops.tr",
    command: "run",
    summary: "New in 0.4.0: while, for over a half-open Int range, break and continue.",
    code: `fn main() -> Unit {
    let mut total = 0
    for i in 1..101 {
        total = total + i
    }
    print("sum 1..100 = \${total}")

    let mut n = 27
    let mut steps = 0
    while n != 1 {
        if n % 2 == 0 {
            n = n / 2
        } else {
            n = 3 * n + 1
        }
        steps = steps + 1
    }
    print("27 reaches 1 in \${steps} steps")

    for i in 0..10 {
        if i % 2 == 0 {
            continue
        }
        if i > 7 {
            break
        }
        print("odd \${i}")
    }
}`,
    output: `sum 1..100 = 5050
27 reaches 1 in 111 steps
odd 1
odd 3
odd 5
odd 7`,
    exitCode: 0,
  },
  {
    id: "collections",
    label: "Arrays & maps",
    file: "groceries.tr",
    command: "run",
    summary: "New in 0.5.0: arrays and maps with value semantics. Copying a collection never shares changes.",
    code: `fn main() -> Unit {
    let mut cart = ["apples", "bread"]
    cart.push("milk")

    let mut prices = ["apples": 3, "bread": 2, "milk": 1]
    prices["bread"] = 4

    let mut total = 0
    for item in cart {
        total = total + prices[item]
    }
    print(cart)
    print("total: \${total}")

    let receipt = cart
    cart[0] = "pears"
    print("receipt keeps \${receipt[0]}")
}`,
    output: `["apples", "bread", "milk"]
total: 8
receipt keeps apples`,
    exitCode: 0,
  },
  {
    id: "enums",
    label: "Enums & match",
    file: "shapes.tr",
    command: "run",
    summary: "New in 0.5.0: enums carry values, and match must cover every case.",
    code: `enum Shape {
    Circle(Float)
    Rect(Float, Float)
    Empty
}

fn area(s: Shape) -> Float {
    match s {
        Shape.Circle(r) => { return 3.14 * r * r }
        Shape.Rect(w, h) => { return w * h }
        Shape.Empty => { return 0.0 }
    }
}

fn main() -> Unit {
    for s in [Shape.Circle(1.0), Shape.Rect(2.0, 3.0), Shape.Empty] {
        print("\${s}: \${area(s)}")
    }
}`,
    output: `Shape.Circle(1.0): 3.14
Shape.Rect(2.0, 3.0): 6.0
Shape.Empty: 0.0`,
    exitCode: 0,
  },
  {
    id: "generics",
    label: "Generics",
    file: "generics.tr",
    command: "run",
    summary: "New in 0.5.0: generic functions and the built-in Option<T>.",
    code: `fn first<T>(items: Array<T>) -> Option<T> {
    if items.len() == 0 {
        return Option.None
    }
    return Option.Some(items[0])
}

fn main() -> Unit {
    print(first([3, 1, 2]))
    print(first(["x", "y"]))

    let ages = ["sam": 31]
    match ages.get("bob") {
        Option.Some(age) => { print("bob is \${age}") }
        Option.None => { print("no age for bob") }
    }
}`,
    output: `Option.Some(3)
Option.Some("x")
no age for bob`,
    exitCode: 0,
  },
  {
    id: "traces",
    label: "Stack traces",
    file: "trace.tr",
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
    output: `error[TR-R001]: division by zero (in function 'divide')
   --> trace.tr:10:12
    |
 10 |     return a / b
    |            ^^^^^

stack trace:
  0: divide   trace.tr:10:12  return a / b
  1: average  trace.tr:6:12   return divide(total, count)
  2: main     trace.tr:2:11   print(average(10, 0))`,
    exitCode: 1,
  },
];

// Real output of the CLI commands (tread 0.5.0).
export const CLI_HELP = `Usage: tread <command> [arguments]

Commands:
  new <name>                       Create a new project in the directory <name>
  check <file.tr>                  Check a program for errors without running it
  build <file.tr> [-o <file.tbc>]  Compile a program to a bytecode file
  run <file.tr|file.tbc>           Run a program from source or bytecode
  doctor                           Diagnose the Tread installation
  version                          Print the Tread version
  sdk --version                    Print the Tread SDK version
  runtime --version                Print the Tread Runtime version
  help                             Print this help

Flags:
  -h, --help     Print this help
  --version      Print the Tread version`;
