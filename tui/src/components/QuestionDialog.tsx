import React, { useState } from "react"
import { Box, Text, useInput } from "ink"

interface Props {
  question: string
  options?: string[]
  onRespond: (answer: string) => void
  width: number
}

export const QuestionDialog = React.memo(function QuestionDialog({ question, options, onRespond, width }: Props) {
  const [answer, setAnswer] = useState("")
  const [selectedOption, setSelectedOption] = useState(0)

  const boxWidth = Math.min(width - 4, 70)

  useInput((input, key) => {
    if (options && options.length > 0) {
      if (key.upArrow) setSelectedOption((s) => Math.max(0, s - 1))
      if (key.downArrow) setSelectedOption((s) => Math.min(options.length - 1, s + 1))
      if (key.return) onRespond(options[selectedOption])
      return
    }

    // Free text mode
    if (key.return && answer.trim()) {
      onRespond(answer.trim())
      return
    }
    if (key.backspace || key.delete) {
      setAnswer((a) => a.slice(0, -1))
      return
    }
    if (input && !key.ctrl && !key.meta) {
      setAnswer((a) => a + input)
    }
  })

  return (
    <Box
      flexDirection="column"
      borderStyle="round"
      borderColor="cyan"
      paddingX={2}
      paddingY={1}
      width={boxWidth}
    >
      <Text color="cyan" bold>Agent question</Text>
      <Text wrap="wrap">{question}</Text>

      {options && options.length > 0 ? (
        <Box flexDirection="column" marginTop={1}>
          {options.map((opt, i) => (
            <Box key={opt}>
              <Text color={i === selectedOption ? "cyan" : undefined}>
                {i === selectedOption ? "> " : "  "}
              </Text>
              <Text bold={i === selectedOption}>{opt}</Text>
            </Box>
          ))}
          <Text dimColor>↑↓ navigate · Enter select</Text>
        </Box>
      ) : (
        <Box flexDirection="column" marginTop={1}>
          <Box>
            <Text bold color="white">{"> "}</Text>
            <Text>{answer}</Text>
            <Text color="white" inverse> </Text>
          </Box>
          {!answer && <Text dimColor>Type your answer...</Text>}
        </Box>
      )}
    </Box>
  )
})
