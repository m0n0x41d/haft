// L0 bundle bootstrap: force Ink's optional devtools path off before loading
// the real TUI entry. Installed bundles must not depend on local node_modules.

process.env.DEV = "false"

await import("./index.js")

export {}
