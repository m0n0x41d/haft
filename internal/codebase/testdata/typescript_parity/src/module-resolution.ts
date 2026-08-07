import {
  ApplicationState,
  parseEntityId,
  stateLabel,
} from "@multi/domain"
import { workspaceSubpath } from "@parity/ui/sub"
import {
  reexportedDefault,
  twiceRenamed,
  wildcardTarget,
} from "./reexports"
import { declaredHelper } from "./types/contracts"

export function runMultipleAliasTarget(): string {
  return parseEntityId("multi")
}

export function runWorkspaceSubpath(): string {
  return workspaceSubpath()
}

export function runTwiceRenamed(): string {
  return twiceRenamed()
}

export function runReexportedDefault(): string {
  return reexportedDefault()
}

export function runWildcard(): string {
  return wildcardTarget()
}

export function runDeclaration(): string {
  return declaredHelper("declared")
}

export class Service {
  status(): string {
    return "ready"
  }
}

export function buildService(): Service {
  return new Service()
}

export function readStateLabel(state: ApplicationState): string {
  return stateLabel[state]
}

export class ReceiverClient {
  fetch(): string {
    return "fetched"
  }

  static ping(): string {
    return "pong"
  }
}

export function useAnnotatedReceiver(client: ReceiverClient): string {
  return client.fetch()
}

export function useInferredReceiver(): string {
  const client = new ReceiverClient()
  return client.fetch()
}

export class ReceiverHolder {
  client: ReceiverClient

  constructor(client: ReceiverClient) {
    this.client = client
  }

  run(): string {
    return this.client.fetch()
  }
}

export function useStaticReceiver(): string {
  return ReceiverClient.ping()
}
