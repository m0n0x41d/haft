import defaultHandler from "./default/default-handler"
import { barrelTarget } from "./barrel"
import { parseResponse } from "./helpers"
import { inheritedHelper } from "@inherited/inherited"
import { workspaceWidget } from "@parity/ui"

export class Worker {
  step(): string {
    return "step"
  }

  run(): string {
    return this.step()
  }
}

export function runDefault(): string {
  return defaultHandler()
}

export function runBarrel(): string {
  return barrelTarget()
}

export function runInherited(): string {
  return inheritedHelper()
}

export function runWorkspace(): string {
  return workspaceWidget()
}

export function runShadowed(parseResponse: (value: string) => string): string[] {
  return ["local"].map(parseResponse)
}
