// L4: Renders parsed ANSI spans as Ink <Text> elements.
// Thin bridge from L1 (ansi.ts Span[]) to React/Ink.

import React from "react"
import { Text } from "ink"
import { parseAnsi, type Span } from "../rendering/ansi.js"

interface Props {
  children: string
}

/**
 * Parses an ANSI-escaped string and renders it with Ink's <Text> styling.
 * Use for pre-highlighted code from cli-highlight.
 */
export const AnsiText = React.memo(function AnsiText({ children }: Props) {
  if (!children) return null

  const spans = parseAnsi(children)

  if (spans.length === 0) return null

  if (spans.length === 1 && !hasStyle(spans[0]!)) {
    return <Text>{spans[0]!.text}</Text>
  }

  return (
    <Text>
      {spans.map((span, i) =>
        hasStyle(span)
          ? <Text
              key={i}
              color={span.style.fg}
              backgroundColor={span.style.bg}
              bold={span.style.bold}
              dimColor={span.style.dim}
              italic={span.style.italic}
              underline={span.style.underline}
              strikethrough={span.style.strikethrough}
              inverse={span.style.inverse}
            >{span.text}</Text>
          : span.text
      )}
    </Text>
  )
})

function hasStyle(span: Span): boolean {
  const s = span.style
  return s.fg !== undefined
    || s.bg !== undefined
    || s.bold === true
    || s.dim === true
    || s.italic === true
    || s.underline === true
    || s.strikethrough === true
    || s.inverse === true
}
