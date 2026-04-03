// Keypress parser (originally from enquirer, adapted for haft).
// Local copy to avoid deep-import into ink/build/ which is blocked by package exports.

import { Buffer } from "node:buffer"

const metaKeyCodeRe = /^(?:\x1b)([a-zA-Z0-9])$/
const fnKeyRe = /^(?:\x1b+)(O|N|\[|\[\[)(?:(\d+)(?:;(\d+))?([~^$])|(?:1;)?(\d+)?([a-zA-Z]))/

const keyName: Record<string, string> = {
  OP: "f1", OQ: "f2", OR: "f3", OS: "f4",
  "[11~": "f1", "[12~": "f2", "[13~": "f3", "[14~": "f4",
  "[[A": "f1", "[[B": "f2", "[[C": "f3", "[[D": "f4", "[[E": "f5",
  "[15~": "f5", "[17~": "f6", "[18~": "f7", "[19~": "f8",
  "[20~": "f9", "[21~": "f10", "[23~": "f11", "[24~": "f12",
  "[A": "up", "[B": "down", "[C": "right", "[D": "left",
  "[E": "clear", "[F": "end", "[H": "home",
  OA: "up", OB: "down", OC: "right", OD: "left",
  OE: "clear", OF: "end", OH: "home",
  "[1~": "home", "[2~": "insert", "[3~": "delete", "[4~": "end",
  "[5~": "pageup", "[6~": "pagedown",
  "[[5~": "pageup", "[[6~": "pagedown",
  "[7~": "home", "[8~": "end",
  "[a": "up", "[b": "down", "[c": "right", "[d": "left", "[e": "clear",
  "[2$": "insert", "[3$": "delete", "[5$": "pageup", "[6$": "pagedown",
  "[7$": "home", "[8$": "end",
  Oa: "up", Ob: "down", Oc: "right", Od: "left", Oe: "clear",
  "[2^": "insert", "[3^": "delete", "[5^": "pageup", "[6^": "pagedown",
  "[7^": "home", "[8^": "end",
  "[Z": "tab",
}

export const nonAlphanumericKeys = [...Object.values(keyName), "backspace"]

const shiftKeys = new Set(["[a","[b","[c","[d","[e","[2$","[3$","[5$","[6$","[7$","[8$","[Z"])
const ctrlKeys = new Set(["Oa","Ob","Oc","Od","Oe","[2^","[3^","[5^","[6^","[7^","[8^"])

export interface ParsedKey {
  name: string
  ctrl: boolean
  meta: boolean
  shift: boolean
  option: boolean
  sequence: string
  raw: string | undefined
  code?: string
}

export default function parseKeypress(s: string | Buffer = ""): ParsedKey {
  let parts: RegExpExecArray | null
  let str: string

  if (Buffer.isBuffer(s)) {
    if (s[0]! > 127 && s[1] === undefined) {
      s[0]! -= 128
      str = "\x1b" + String(s)
    } else {
      str = String(s)
    }
  } else if (typeof s !== "string") {
    str = String(s)
  } else {
    str = s || ""
  }

  const key: ParsedKey = {
    name: "",
    ctrl: false,
    meta: false,
    shift: false,
    option: false,
    sequence: str,
    raw: str,
  }

  key.sequence = key.sequence || str || key.name

  if (str === "\r") {
    key.raw = undefined
    key.name = "return"
  } else if (str === "\n") {
    key.name = "enter"
  } else if (str === "\t") {
    key.name = "tab"
  } else if (str === "\b" || str === "\x1b\b") {
    key.name = "backspace"
    key.meta = str.charAt(0) === "\x1b"
  } else if (str === "\x7f" || str === "\x1b\x7f") {
    // 0x7f is DEL — most terminals send this for Backspace.
    // Real forward-delete sends \x1b[3~ which hits the keyName table as "delete".
    key.name = "backspace"
    key.meta = str.charAt(0) === "\x1b"
  } else if (str === "\x1b" || str === "\x1b\x1b") {
    key.name = "escape"
    key.meta = str.length === 2
  } else if (str === " " || str === "\x1b ") {
    key.name = "space"
    key.meta = str.length === 2
  } else if (str.length === 1 && str <= "\x1a") {
    key.name = String.fromCharCode(str.charCodeAt(0) + "a".charCodeAt(0) - 1)
    key.ctrl = true
  } else if (str.length === 1 && str >= "0" && str <= "9") {
    key.name = "number"
  } else if (str.length === 1 && str >= "a" && str <= "z") {
    key.name = str
  } else if (str.length === 1 && str >= "A" && str <= "Z") {
    key.name = str.toLowerCase()
    key.shift = true
  } else if ((parts = metaKeyCodeRe.exec(str))) {
    key.meta = true
    key.shift = /^[A-Z]$/.test(parts[1]!)
  } else if ((parts = fnKeyRe.exec(str))) {
    const segs = [...str]
    if (segs[0] === "\u001b" && segs[1] === "\u001b") {
      key.option = true
    }
    const code = [parts[1], parts[2], parts[4], parts[6]].filter(Boolean).join("")
    const modifier = (Number(parts[3] || parts[5] || 1)) - 1
    key.ctrl = !!(modifier & 4)
    key.meta = !!(modifier & 10)
    key.shift = !!(modifier & 1)
    key.code = code
    key.name = keyName[code] ?? ""
    key.shift = shiftKeys.has(code) || key.shift
    key.ctrl = ctrlKeys.has(code) || key.ctrl
  }

  return key
}
