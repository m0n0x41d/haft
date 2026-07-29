import { advanceRevision, persistSession } from "./helpers"

export const session = {
  selectPersona(): number {
    advanceRevision()
    return persistSession()
  },
}
