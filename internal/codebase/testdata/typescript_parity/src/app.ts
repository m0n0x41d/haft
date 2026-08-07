import { apiClient } from "@core/api-client"
import { parseEntityId, readinessCounts } from "@core/domain"
import { parseResponse } from "@core/helpers"
import { session } from "@core/session"

export const createApplication = (): string => {
  parseEntityId("entity-1")
  readinessCounts(["entity-1"])
  session.selectPersona()
  return apiClient.me()
}

export function mapEntities(values: string[]): string[] {
  return values.map(parseResponse)
}
