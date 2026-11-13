// Runs the Tread playground off the main thread, so the page stays
// responsive while a program runs and a long run can be stopped by
// terminating this worker. tread.wasm is the real Tread SDK compiled to
// WebAssembly (cmd/tread-playground, scripts/build-playground.sh).
//
// Messages in:  { id, source, check }
// Messages out: { type: "ready", version }
//               { type: "result", id, ms, stdout, stderr, exit, tokens, ast, bytecode }
//               { type: "error", message }
/* global Go */
importScripts("wasm_exec.js");

const ready = (async () => {
  const go = new Go();
  let result;
  try {
    result = await WebAssembly.instantiateStreaming(fetch("tread.wasm"), go.importObject);
  } catch {
    // Some servers send the wrong Content-Type; fall back to bytes.
    const bytes = await (await fetch("tread.wasm")).arrayBuffer();
    result = await WebAssembly.instantiate(bytes, go.importObject);
  }
  go.run(result.instance); // registers treadPlay and treadVersion, then waits
  self.postMessage({ type: "ready", version: self.treadVersion });
})().catch((err) => {
  self.postMessage({ type: "error", message: String(err && err.message ? err.message : err) });
  throw err;
});

self.onmessage = async (event) => {
  await ready;
  const { id, source, check } = event.data;
  const start = performance.now();
  const r = self.treadPlay(String(source), Boolean(check));
  self.postMessage({
    type: "result",
    id,
    ms: performance.now() - start,
    stdout: r.stdout,
    stderr: r.stderr,
    exit: r.exit,
    tokens: r.tokens,
    ast: r.ast,
    bytecode: r.bytecode,
  });
};
