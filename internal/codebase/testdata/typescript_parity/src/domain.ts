export type ApplicationState = "draft" | "ready"

export type EntityId = string & { readonly __brand: "EntityId" }

export function parseEntityId(raw: string): EntityId {
  return raw as EntityId
}

export function makeEntityId(raw: string): EntityId {
  return parseEntityId(raw)
}

export const readinessCounts = (values: string[]): number => {
  return values.filter((value) => parseEntityId(value).length > 0).length
}

export const stateLabel: Record<ApplicationState, string> = {
  draft: "Draft",
  ready: "Ready",
}

export class ClientError extends Error {}

export function sessionFromRequest(requestId: string): EntityId {
  return makeEntityId(requestId)
}
