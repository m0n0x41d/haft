import { strict as assert } from "node:assert"
import { test } from "node:test"
import type React from "react"
import { TranscriptViewport } from "./TranscriptViewport.js"

test("applies the computed crop offset for a scrolled viewport", () => {
  const measureRef = () => () => {}
  const element = TranscriptViewport({
    entries: [],
    measureRef,
    viewport: {
      start: 1,
      end: 4,
      viewTop: 7,
      viewBottom: 15,
      cropTop: 3,
      topSpacer: 4,
      bottomSpacer: 0,
      totalLines: 18,
    },
    toolHistoryExpanded: false,
    width: 80,
  }) as React.ReactElement
  const [topSpacer, croppedViewport] = element.props.children as React.ReactElement[]

  assert.equal(element.props.marginTop, -4)
  assert.equal(topSpacer.props.height, 4)
  assert.equal(croppedViewport.props.marginTop, -3)
})

test("renders a bottom spacer for virtualized rows below the mounted range", () => {
  const measureRef = () => () => {}
  const element = TranscriptViewport({
    entries: [],
    measureRef,
    viewport: {
      start: 0,
      end: 1,
      viewTop: 0,
      viewBottom: 4,
      cropTop: 0,
      topSpacer: 0,
      bottomSpacer: 6,
      totalLines: 10,
    },
    toolHistoryExpanded: false,
    width: 80,
  }) as React.ReactElement
  const children = element.props.children as React.ReactElement[]
  const bottomSpacer = children[2]

  assert.equal(bottomSpacer.props.height, 6)
})
