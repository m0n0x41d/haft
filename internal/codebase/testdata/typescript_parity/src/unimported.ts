export function runUnimported(): string {
  return parseResponse("not imported")
}

export function runAmbiguous(): string {
  return duplicate()
}
