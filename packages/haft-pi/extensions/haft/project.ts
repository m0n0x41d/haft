import { existsSync } from "node:fs";
import { dirname, join, resolve } from "node:path";

import type { PiContext } from "./pi-api.ts";

export function contextCwd(ctx: PiContext): string {
  const cwd = ctx.cwd;
  if (typeof cwd === "string" && cwd.trim().length > 0) {
    return cwd;
  }

  return process.cwd();
}

export function findHaftProjectRoot(cwd: string): string | undefined {
  let current = resolve(cwd);

  for (;;) {
    const candidate = join(current, ".haft");
    if (existsSync(candidate)) {
      return current;
    }

    const parent = dirname(current);
    if (parent === current) {
      return undefined;
    }

    current = parent;
  }
}

export function requireHaftProjectRoot(ctx: PiContext): string {
  const projectRoot = findHaftProjectRoot(contextCwd(ctx));
  if (projectRoot !== undefined) {
    return projectRoot;
  }

  throw new Error("No `.haft` directory found from the current Pi working directory.");
}
