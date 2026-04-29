/// <reference types="node" />

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import test from "node:test";

import {
  projectReadiness,
  type ProjectReadiness,
} from "./projectReadiness.ts";

interface ReadinessContractFixture {
  readonly schema: string;
  readonly canonical_statuses: readonly string[];
  readonly unknown_status_examples: readonly string[];
}

const fixturePath = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../../../readiness-contract/canonical-statuses.json",
);

const fixture = JSON.parse(readFileSync(fixturePath, "utf8")) as ReadinessContractFixture;

const TYPESCRIPT_CANONICAL_STATUSES: readonly ProjectReadiness[] = [
  "ready",
  "needs_init",
  "needs_onboard",
  "missing",
];

test("readiness contract fixture is the expected schema version", () => {
  assert.equal(fixture.schema, "haft.readiness-contract.v1");
  assert.ok(fixture.canonical_statuses.length > 0);
  assert.ok(fixture.unknown_status_examples.length > 0);
});

test("readiness contract canonical statuses match the TypeScript union", () => {
  const got = [...fixture.canonical_statuses].sort();
  const want = [...TYPESCRIPT_CANONICAL_STATUSES].sort();

  assert.deepEqual(got, want);
});

for (const unknown of fixture.unknown_status_examples) {
  test(`unknown status ${JSON.stringify(unknown)} cannot fake-ready an unready project`, () => {
    const readiness = projectReadiness({
      status: unknown,
      exists: true,
      has_haft: true,
      has_specs: false,
    });

    assert.equal(
      readiness,
      "needs_onboard",
      `unknown status ${JSON.stringify(unknown)} must not silently mark an onboarding project as ready`,
    );
  });

  test(`unknown status ${JSON.stringify(unknown)} cannot fake-ready a missing project`, () => {
    const readiness = projectReadiness({
      status: unknown,
      exists: false,
      has_haft: false,
      has_specs: false,
    });

    assert.equal(
      readiness,
      "missing",
      `unknown status ${JSON.stringify(unknown)} must not override a missing project`,
    );
  });
}
