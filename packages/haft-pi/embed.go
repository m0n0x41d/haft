// Package haftpi embeds the @haft/pi Pi package assets into the haft binary
// so `haft init --pi` can materialize a working local-path package without
// npm publishing. Runtime needs no node_modules: the extension's only import
// (typebox) is a Pi-bundled core package resolved via peerDependencies.
package haftpi

import "embed"

// Assets carries everything Pi needs to load the package from a local path:
// the extension sources, prompt templates, Agent Skills, and the manifest.
// Tests, scripts, lockfile, and tsconfig are repo-development concerns and
// stay out of the embedded set.
//
//go:embed all:extensions all:prompts all:skills package.json README.md
var Assets embed.FS
