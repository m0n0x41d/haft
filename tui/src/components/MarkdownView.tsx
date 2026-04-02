import React from "react"
import { Box, Text } from "ink"

interface Props {
  text: string
  width: number
}

// Markdown renderer matching CC's inline rendering patterns
export function MarkdownView({ text, width }: Props) {
  const lines = text.split("\n")
  const elements: React.ReactNode[] = []
  let inCodeBlock = false
  let codeLang = ""

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]

    // Code block boundaries
    if (line.trimStart().startsWith("```")) {
      if (inCodeBlock) {
        inCodeBlock = false
        elements.push(<Text key={`cend-${i}`} dimColor>{"```"}</Text>)
      } else {
        inCodeBlock = true
        codeLang = line.trimStart().slice(3).trim()
        elements.push(<Text key={`cstart-${i}`} dimColor>{"```"}{codeLang}</Text>)
      }
      continue
    }

    if (inCodeBlock) {
      elements.push(<Text key={`code-${i}`} color="white">{"  "}{line}</Text>)
      continue
    }

    // Headers
    if (line.startsWith("# ")) {
      elements.push(<Text key={`h-${i}`} bold color="white">{line.slice(2)}</Text>)
      continue
    }
    if (line.startsWith("## ")) {
      elements.push(<Text key={`h2-${i}`} bold>{line.slice(3)}</Text>)
      continue
    }
    if (line.startsWith("### ")) {
      elements.push(<Text key={`h3-${i}`} bold>{line.slice(4)}</Text>)
      continue
    }

    // List items — CC uses • (U+2022)
    if (line.match(/^\s*[-*]\s/)) {
      elements.push(<Text key={`li-${i}`}>{line.replace(/^(\s*)[-*]\s/, "$1\u2022 ")}</Text>)
      continue
    }
    if (line.match(/^\s*\d+\.\s/)) {
      elements.push(<Text key={`ol-${i}`}>{line}</Text>)
      continue
    }

    // Blockquotes — CC uses │ (U+2502) with cyan dimColor
    if (line.startsWith("> ")) {
      elements.push(
        <Text key={`bq-${i}`}>
          <Text dimColor color="cyan">{"\u2502"} </Text>
          <Text dimColor>{line.slice(2)}</Text>
        </Text>
      )
      continue
    }

    // Horizontal rule
    if (line.match(/^-{3,}$/) || line.match(/^\*{3,}$/)) {
      elements.push(<Text key={`hr-${i}`} dimColor>{"\u2500".repeat(Math.min(width - 2, 60))}</Text>)
      continue
    }

    // Empty line
    if (line.trim() === "") {
      elements.push(<Text key={`empty-${i}`}> </Text>)
      continue
    }

    // Regular text with inline formatting
    elements.push(<InlineMarkdown key={`p-${i}`} text={line} />)
  }

  return (
    <Box flexDirection="column">
      {elements}
    </Box>
  )
}

// Handles **bold**, `code`, *italic* within a single line
function InlineMarkdown({ text }: { text: string }) {
  if (!text) return <Text> </Text>

  const parts: React.ReactNode[] = []
  let remaining = text
  let key = 0

  while (remaining.length > 0) {
    const boldMatch = remaining.match(/\*\*(.+?)\*\*/)
    const codeMatch = remaining.match(/`([^`]+)`/)
    const italicMatch = remaining.match(/(?<!\*)\*([^*]+)\*(?!\*)/)

    const matches = [
      boldMatch ? { type: "bold" as const, match: boldMatch, idx: boldMatch.index! } : null,
      codeMatch ? { type: "code" as const, match: codeMatch, idx: codeMatch.index! } : null,
      italicMatch ? { type: "italic" as const, match: italicMatch, idx: italicMatch.index! } : null,
    ].filter(Boolean).sort((a, b) => a!.idx - b!.idx)

    if (matches.length === 0) {
      parts.push(<Text key={key++}>{remaining}</Text>)
      break
    }

    const first = matches[0]!
    if (first.idx > 0) {
      parts.push(<Text key={key++}>{remaining.slice(0, first.idx)}</Text>)
    }

    switch (first.type) {
      case "bold":
        parts.push(<Text key={key++} bold>{first.match[1]}</Text>)
        break
      case "code":
        // white on blackBright with padding
        parts.push(<Text key={key++} color="white" backgroundColor="blackBright"> {first.match[1]} </Text>)
        break
      case "italic":
        parts.push(<Text key={key++} italic>{first.match[1]}</Text>)
        break
    }

    remaining = remaining.slice(first.idx + first.match[0].length)
  }

  return <Text>{parts}</Text>
}
