export function parseResponse(value: string): string {
  return value.trim()
}

export const advanceRevision = (): number => 1

export const persistSession = (): number => 2

export function duplicate(): string {
  return "helpers"
}
