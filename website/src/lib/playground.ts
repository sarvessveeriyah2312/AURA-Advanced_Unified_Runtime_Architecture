import { useCallback, useEffect, useRef, useState } from "react";

// Client for public/playground/worker.js, which runs the real Tread SDK
// (compiled to WebAssembly) in a Web Worker.

export interface PlayResult {
  stdout: string;
  stderr: string;
  exit: number;
  ms: number;
  // Compiler stages; "" when unavailable.
  tokens: string;
  ast: string;
  bytecode: string;
  check: boolean;
}

export type PlayStatus = "loading" | "ready" | "running" | "failed";

const WORKER_URL = "/playground/worker.js";

export function usePlayground() {
  const [status, setStatus] = useState<PlayStatus>("loading");
  const [version, setVersion] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const worker = useRef<Worker | null>(null);
  const pending = useRef<{ id: number; check: boolean; resolve: (r: PlayResult | null) => void } | null>(null);
  const nextId = useRef(1);

  // spawn starts a fresh worker; state changes come from its messages.
  const spawn = useCallback(() => {
    worker.current?.terminate();
    const w = new Worker(WORKER_URL);
    w.onmessage = (e: MessageEvent) => {
      const msg = e.data as { type: string; id?: number; version?: string; message?: string } & Omit<PlayResult, "check">;
      if (msg.type === "ready") {
        setVersion(msg.version ?? null);
        setStatus("ready");
      } else if (msg.type === "error") {
        setError(msg.message ?? "unknown error");
        setStatus("failed");
      } else if (msg.type === "result" && pending.current && msg.id === pending.current.id) {
        const p = pending.current;
        pending.current = null;
        setStatus("ready");
        p.resolve({ ...msg, check: p.check });
      }
    };
    w.onerror = () => {
      setError("the playground could not start in this browser");
      setStatus("failed");
    };
    worker.current = w;
  }, []);

  useEffect(() => {
    spawn();
    return () => worker.current?.terminate();
  }, [spawn]);

  // run resolves with the result, or null if the run was stopped.
  const run = useCallback((source: string, check: boolean): Promise<PlayResult | null> => {
    const w = worker.current;
    if (!w) return Promise.resolve(null);
    pending.current?.resolve(null);
    const id = nextId.current++;
    setStatus("running");
    return new Promise((resolve) => {
      pending.current = { id, check, resolve };
      w.postMessage({ id, source, check });
    });
  }, []);

  // stop ends a running program by restarting the worker.
  const stop = useCallback(() => {
    pending.current?.resolve(null);
    pending.current = null;
    setStatus("loading");
    setError(null);
    spawn();
  }, [spawn]);

  return { status, version, error, run, stop };
}

// Programs are shared in the URL fragment (#code=...), base64url-encoded
// UTF-8, so a shared link needs no server and never reaches one.
export function encodeProgram(code: string): string {
  const bytes = new TextEncoder().encode(code);
  let bin = "";
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

export function decodeProgram(encoded: string): string | null {
  try {
    const bin = atob(encoded.replace(/-/g, "+").replace(/_/g, "/"));
    const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0));
    return new TextDecoder("utf-8", { fatal: true }).decode(bytes);
  } catch {
    return null;
  }
}

// The program in a shared link's fragment, or null. Read with
// useSyncExternalStore: null while prerendering and hydrating, then the
// real value, so the prerendered markup always matches.
export function subscribeHash(onChange: () => void): () => void {
  window.addEventListener("hashchange", onChange);
  return () => window.removeEventListener("hashchange", onChange);
}

export function sharedProgram(): string | null {
  const m = /^#code=([A-Za-z0-9_-]+)$/.exec(window.location.hash);
  return m?.[1] ? decodeProgram(m[1]) : null;
}
