/// <reference types="node" />

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import test from "node:test";

import { visibleTranscriptText } from "./api.ts";

interface ParityCase {
  readonly name: string;
  readonly input: string;
  readonly expected: string;
}

interface ParityFixture {
  readonly schema: string;
  readonly cases: readonly ParityCase[];
}

const fixturePath = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../../../transcript-parity/cases.json",
);

const fixture = JSON.parse(readFileSync(fixturePath, "utf8")) as ParityFixture;

test("transcript parity fixture is the expected schema version", () => {
  assert.equal(fixture.schema, "haft.transcript-parity.v1");
  assert.ok(fixture.cases.length > 0, "fixture must declare at least one case");
});

for (const parityCase of fixture.cases) {
  test(`transcript normalizer parity: ${parityCase.name}`, () => {
    const actual = visibleTranscriptText(parityCase.input);

    assert.equal(actual, parityCase.expected);
  });
}
