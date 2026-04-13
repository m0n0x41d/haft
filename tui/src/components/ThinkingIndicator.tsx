import React, { useState, useEffect, useRef } from "react"
import { Box, Text } from "ink"

const VERBS = [
  "Forging", "Weaving", "Casting", "Shaping", "Tracing",
  "Probing", "Binding", "Honing", "Carving", "Fusing",
]

const DOT = "\u25CF"
const SMALL = "\u2219"
const WIDTH = 5 // 5 positions for wider animation

interface Props {
  model: string
}

export function ThinkingIndicator({ model }: Props) {
  const [dotFrame, setDotFrame] = useState(SMALL.repeat(WIDTH))
  const [verbTick, setVerbTick] = useState(0)

  const stateRef = useRef({
    mode: "sweep" as "sweep" | "blink" | "burst",
    pos: 0,
    dir: 1,
    counter: 0,
  })

  // Chaotic dot animation
  useEffect(() => {
    const render = () => {
      const s = stateRef.current
      const dots = Array(WIDTH).fill(SMALL)

      if (s.mode === "blink") {
        const mid = Math.floor(WIDTH / 2)
        if (s.counter % 2 === 0) dots[mid] = DOT
        s.counter++
        if (s.counter > 4 + Math.floor(Math.random() * 4)) {
          s.mode = "sweep"; s.counter = 0; s.pos = mid
        }
      } else if (s.mode === "burst") {
        dots[s.pos] = DOT
        s.pos += s.dir
        if (s.pos >= WIDTH - 1) { s.pos = WIDTH - 1; s.dir = -1 }
        if (s.pos <= 0) { s.pos = 0; s.dir = 1 }
        s.counter++
        if (s.counter > 8 + Math.floor(Math.random() * 8)) {
          s.mode = "sweep"; s.counter = 0
        }
      } else {
        dots[s.pos] = DOT
        s.counter++
        if (s.counter % 2 === 0) {
          s.pos += s.dir
          if (s.pos >= WIDTH - 1) { s.pos = WIDTH - 1; s.dir = -1 }
          if (s.pos <= 0) { s.pos = 0; s.dir = 1 }
        }
        if (s.counter > 8 && Math.random() < 0.08) {
          s.mode = "blink"; s.counter = 0; s.pos = Math.floor(WIDTH / 2)
        } else if (s.counter > 12 && Math.random() < 0.05) {
          s.mode = "burst"; s.counter = 0
        }
      }

      setDotFrame(dots.join(""))
    }

    render()
    let timer: ReturnType<typeof setTimeout>
    const tick = () => {
      render()
      const s = stateRef.current
      const ms = s.mode === "burst" ? 50 + Math.random() * 30
        : s.mode === "blink" ? 250 + Math.random() * 150
        : 180 + Math.random() * 80
      timer = setTimeout(tick, ms)
    }
    timer = setTimeout(tick, 200)
    return () => clearTimeout(timer)
  }, [])

  // Scanning verb highlight
  useEffect(() => {
    const timer = setInterval(() => setVerbTick((t) => t + 1), 100)
    return () => clearInterval(timer)
  }, [])

  const verbIdx = Math.floor(Date.now() / 10000) % VERBS.length
  const verb = VERBS[verbIdx]
  const wordLen = verb.length
  const cycleLen = wordLen * 2
  const pos = verbTick % cycleLen
  const highlightPos = pos < wordLen ? pos : cycleLen - pos - 1

  const chars = verb.split("").map((ch, i) => {
    if (i === highlightPos) {
      return <Text key={i} color="yellow" bold>{ch}</Text>
    }
    return <Text key={i} dimColor>{ch}</Text>
  })

  return (
    <Box paddingX={1} marginTop={1}>
      <Text color="yellow">{dotFrame} </Text>
      <Text>{chars}</Text>
      <Text dimColor>{"\u2026"}</Text>
    </Box>
  )
}
