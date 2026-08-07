#!/usr/bin/env node
// Fake `haft serve` for bridge protocol tests: speaks NDJSON JSON-RPC on
// stdio. Configure advertised tools via FAKE_TOOLS (comma-separated).
// Exits with code 7 on a duplicate `initialize` to expose double-start races.
import { createInterface } from "node:readline";

const tools = (process.env.FAKE_TOOLS ?? "haft_query")
  .split(",")
  .map((name) => name.trim())
  .filter((name) => name.length > 0)
  .map((name) => ({ name, description: "fake", inputSchema: { type: "object" } }));

let initialized = false;

const reply = (id, result) => {
  process.stdout.write(`${JSON.stringify({ jsonrpc: "2.0", id, result })}\n`);
};

const handle = (request) => {
  if (request.method === "initialize") {
    if (initialized) {
      process.exit(7);
    }

    initialized = true;
    reply(request.id, { protocolVersion: "2024-11-05", capabilities: {}, serverInfo: { name: "fake-haft" } });
    return;
  }

  if (request.method === "tools/list") {
    reply(request.id, { tools });
    return;
  }

  if (request.method === "tools/call") {
    const name = request.params?.name;
    const args = request.params?.arguments ?? {};
    if (args.boom === true) {
      reply(request.id, { content: [{ type: "text", text: "kernel says no" }], isError: true });
      return;
    }

    reply(request.id, { content: [{ type: "text", text: `ok:${name}:${JSON.stringify(args)}` }] });
  }
};

createInterface({ input: process.stdin })
  .on("line", (line) => {
    const trimmed = line.trim();
    if (trimmed.length === 0) {
      return;
    }

    handle(JSON.parse(trimmed));
  });
