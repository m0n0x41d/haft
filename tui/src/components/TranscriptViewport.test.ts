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

  assert.equal(element.props.marginTop, -3)
})
