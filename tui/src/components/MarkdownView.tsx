import React from "react"
import { Box, Text } from "ink"
import { AnsiText } from "./AnsiText.js"
import { highlightCode } from "../rendering/highlight.js"

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
  let codeLines: string[] = []
  let codeStartIdx = 0

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]

    // Code block boundaries
    if (line.trimStart().startsWith("```")) {
      if (inCodeBlock) {
        // Closing fence — highlight accumulated code and emit
        const code = codeLines.join("\n")
        const highlighted = highlightCode(code, codeLang)
        elements.push(
          <Box key={`code-${codeStartIdx}`} flexDirection="column" paddingLeft={2}>
            <AnsiText>{highlighted}</AnsiText>
          </Box>
        )
        elements.push(<Text key={`cend-${i}`} dimColor>{"```"}</Text>)
        inCodeBlock = false
        codeLines = []
        codeLang = ""
      } else {
        inCodeBlock = true
        codeStartIdx = i
        codeLang = line.trimStart().slice(3).trim()
        elements.push(<Text key={`cstart-${i}`} dimColor>{"```"}{codeLang}</Text>)
      }
      continue
    }

    if (inCodeBlock) {
      codeLines.push(line)
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
      const indent = line.match(/^(\s*)/)?.[1] ?? ""
      const content = line.replace(/^\s*[-*]\s/, "")
      elements.push(
        <Text key={`li-${i}`}>
          <Text>{indent}{"\u2022"} </Text>
          <InlineMarkdown text={content} />
        </Text>
      )
      continue
    }
    if (line.match(/^\s*\d+\.\s/)) {
      const match = line.match(/^(\s*\d+\.\s)(.*)/)
      if (match) {
        elements.push(
          <Text key={`ol-${i}`}>
            <Text>{match[1]}</Text>
            <InlineMarkdown text={match[2]} />
          </Text>
        )
      } else {
        elements.push(<Text key={`ol-${i}`}>{line}</Text>)
      }
      continue
    }

    // Blockquotes — CC uses │ (U+2502) with cyan dimColor
    if (line.startsWith("> ")) {
      elements.push(
        <Text key={`bq-${i}`}>
          <Text dimColor color="cyan">{"\u2502"} </Text>
          <Text dimColor><InlineMarkdown text={line.slice(2)} /></Text>
        </Text>
      )
      continue
    }

    // Horizontal rule
    if (line.match(/^-{3,}$/) || line.match(/^\*{3,}$/)) {
      elements.push(<Text key={`hr-${i}`} dimColor>{"\u2500".repeat(Math.min(width - 2, 60))}</Text>)
      continue
    }

    // Table rows (pipe-delimited)
    if (line.trimStart().startsWith("|") && line.trimEnd().endsWith("|")) {
      // Collect consecutive table lines
      const tableLines: string[] = [line]
      while (i + 1 < lines.length && lines[i + 1].trimStart().startsWith("|") && lines[i + 1].trimEnd().endsWith("|")) {
        i++
        tableLines.push(lines[i])
      }
      elements.push(<TableBlock key={`tbl-${i}`} lines={tableLines} width={width} />)
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

  // Flush unclosed code block (e.g., truncated response)
  if (inCodeBlock && codeLines.length > 0) {
    const code = codeLines.join("\n")
    const highlighted = highlightCode(code, codeLang)
    elements.push(
      <Box key={`code-${codeStartIdx}`} flexDirection="column" paddingLeft={2}>
        <AnsiText>{highlighted}</AnsiText>
      </Box>
    )
  }

  return (
    <Box flexDirection="column">
      {elements}
    </Box>
  )
}

// --- Table rendering ---

function TableBlock({ lines, width }: { lines: string[]; width: number }) {
  // Parse cells from pipe-delimited lines, skip separator rows (|---|---|)
  const isSeparator = (line: string) => /^\|[\s\-:|]+\|$/.test(line.trim())
  const parseCells = (line: string) =>
    line.split("|").slice(1, -1).map((c) => c.trim())

  const dataRows = lines.filter((l) => !isSeparator(l))
  if (dataRows.length === 0) return null

  const allCells = dataRows.map(parseCells)
  const colCount = Math.max(...allCells.map((r) => r.length))

  // Compute column widths from content
  const colWidths: number[] = Array(colCount).fill(0)
  for (const row of allCells) {
    for (let c = 0; c < colCount; c++) {
      colWidths[c] = Math.max(colWidths[c], (row[c] ?? "").length)
    }
  }

  // Cap total width
  const totalPad = colCount * 3 + 1 // " | " between cols + outer borders
  const maxContent = width - totalPad - 2
  if (maxContent > 0) {
    const totalContent = colWidths.reduce((a, b) => a + b, 0)
    if (totalContent > maxContent) {
      const scale = maxContent / totalContent
      for (let c = 0; c < colCount; c++) {
        colWidths[c] = Math.max(3, Math.floor(colWidths[c] * scale))
      }
    }
  }

  return (
    <Box flexDirection="column">
      {allCells.map((row, ri) => (
        <Box key={ri}>
          <Text dimColor>{"\u2502"}</Text>
          {Array.from({ length: colCount }, (_, c) => {
            const cell = (row[c] ?? "").slice(0, colWidths[c])
            const pad = " ".repeat(Math.max(0, colWidths[c] - cell.length))
            return (
              <React.Fragment key={c}>
                <Text bold={ri === 0}> {cell}{pad} </Text>
                <Text dimColor>{"\u2502"}</Text>
              </React.Fragment>
            )
          })}
        </Box>
      ))}
    </Box>
  )
}

// --- Inline formatting ---
// Handles **bold**, `code`, *italic*, ~~strikethrough~~, [text](url) within a single line

type InlineMatch = { type: "bold" | "code" | "italic" | "strikethrough" | "link"; match: RegExpMatchArray; idx: number }

function InlineMarkdown({ text }: { text: string }) {
  if (!text) return <Text> </Text>

  const parts: React.ReactNode[] = []
  let remaining = text
  let key = 0

  while (remaining.length > 0) {
    const boldMatch = remaining.match(/\*\*(.+?)\*\*/)
    const codeMatch = remaining.match(/`([^`]+)`/)
    const italicMatch = remaining.match(/(?<!\*)\*([^*]+)\*(?!\*)/)
    const strikeMatch = remaining.match(/~~(.+?)~~/)
    const linkMatch = remaining.match(/\[([^\]]+)\]\(([^)]+)\)/)

    const candidates: (InlineMatch | null)[] = [
      boldMatch ? { type: "bold", match: boldMatch, idx: boldMatch.index! } : null,
      codeMatch ? { type: "code", match: codeMatch, idx: codeMatch.index! } : null,
      italicMatch ? { type: "italic", match: italicMatch, idx: italicMatch.index! } : null,
      strikeMatch ? { type: "strikethrough", match: strikeMatch, idx: strikeMatch.index! } : null,
      linkMatch ? { type: "link", match: linkMatch, idx: linkMatch.index! } : null,
    ]
    const matches = candidates.filter(Boolean).sort((a, b) => a!.idx - b!.idx) as InlineMatch[]

    if (matches.length === 0) {
      parts.push(<Text key={key++}>{remaining}</Text>)
      break
    }

    const first = matches[0]
    if (first.idx > 0) {
      parts.push(<Text key={key++}>{remaining.slice(0, first.idx)}</Text>)
    }

    switch (first.type) {
      case "bold":
        parts.push(<Text key={key++} bold>{first.match[1]}</Text>)
        break
      case "code":
        parts.push(<Text key={key++} color="white" backgroundColor="blackBright"> {first.match[1]} </Text>)
        break
      case "italic":
        parts.push(<Text key={key++} italic>{first.match[1]}</Text>)
        break
      case "strikethrough":
        parts.push(<Text key={key++} strikethrough dimColor>{first.match[1]}</Text>)
        break
      case "link":
        // Show link text underlined, URL in dim after
        parts.push(<Text key={key++} underline color="blue">{first.match[1]}</Text>)
        parts.push(<Text key={key++} dimColor> ({first.match[2]})</Text>)
        break
    }

    remaining = remaining.slice(first.idx + first.match[0].length)
  }

  return <Text>{parts}</Text>
}
