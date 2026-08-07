// Manual smoke check: drives the owned NDJSON bridge against a real
// `haft serve` for the project root given as argv[2] (defaults to cwd).
// Run: node --experimental-strip-types scripts/smoke-bridge.ts <project-root>
import { bridgeFor, stopAllBridges } from "../extensions/haft/bridge.ts";
import { HAFT_TOOLS } from "../extensions/haft/tools.ts";

const projectRoot = process.argv[2] ?? process.cwd();

const firstLine = (text: string): string => text.split("\n").find((line) => line.trim().length > 0) ?? "<empty>";

const bridge = await bridgeFor(projectRoot, undefined);

const status = await bridge.callTool("haft_query", { action: "status" }, undefined);
console.log(`haft_query(status): ${firstLine(status)}`);

const methodStatus = await bridge.callTool("haft_method", { action: "status" }, undefined);
console.log(`haft_method(status): ${firstLine(methodStatus)}`);

const lifecycle = await bridge.callTool("haft_spec_section", { action: "lifecycle" }, undefined);
console.log(`haft_spec_section(lifecycle): ${firstLine(lifecycle)}`);

const problems = await bridge.callTool("haft_problem", { action: "select" }, undefined);
console.log(`haft_problem(select): ${firstLine(problems)}`);

const governor = await bridge.callTool(
  "haft_query",
  { action: "status", view: "governor" },
  undefined
);
console.log(`governor view (${governor.length} chars):\n${governor}`);

console.log(`registered tool specs: ${HAFT_TOOLS.map((tool) => tool.name).join(", ")}`);

stopAllBridges();
