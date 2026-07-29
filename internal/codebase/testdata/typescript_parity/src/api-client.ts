import { parseResponse } from "./helpers"

export const apiClient = {
  me(): string {
    return parseResponse("me")
  },
}
