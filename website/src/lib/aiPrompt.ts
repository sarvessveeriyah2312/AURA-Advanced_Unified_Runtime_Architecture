// Prompts that a visitor can paste into an AI coding assistant (Claude
// Code, Cursor, Copilot Chat, ...) to run a terminal command for them.
// Each prompt names the goal, the exact command and its shell, asks the
// assistant to explain it before running it, and says how to check the
// result. Keep them plain text: they are pasted into chat boxes.
import { REPO_URL, readmeSection } from "../data/site";

export interface CommandPrompt {
  goal: string; // what the visitor wants, e.g. "install the Tread SDK on macOS"
  command: string; // the exact command, as shown on the page
  shell: string; // e.g. "a macOS terminal (zsh or bash)"
  about?: string; // one sentence on what the command does
  check: string; // how to confirm success, e.g. "run `tread doctor`"
}

export function commandPrompt({ goal, command, shell, about, check }: CommandPrompt): string {
  return [
    `I want to ${goal}.`,
    "",
    `Please run this command in ${shell}:`,
    "",
    command,
    "",
    `Before you run it, briefly explain what it does.${about ? ` ${about}` : ""} Tread's official source is ${REPO_URL}.`,
    "",
    `Afterwards, ${check}, and tell me the result. If a step fails, show me the error and suggest a fix. The installation guide is ${readmeSection("installation")}.`,
  ].join("\n");
}
