package code

import "fmt"

// GoTestEnvironment fixes the compiler selection parameters supported by the
// local Go1.25 adapter. Values follow Go1.25 internal/buildcfg defaults; they are
// declarations for the caller to verify, not observations of its process.
func GoTestEnvironment(config Config) (map[string]string, error) {
	match := toolchainPattern.FindStringSubmatch(config.Toolchain)
	if match == nil || match[1] != "25" || config.CGOEnabled || len(config.ToolTags) != 0 {
		return nil, fmt.Errorf("Go1.25 without cgo or custom compiler tool tags is required for this test-build adapter")
	}
	env := map[string]string{"GOOS": config.GOOS, "GOARCH": config.GOARCH, "CGO_ENABLED": "0", "GOTOOLCHAIN": "local", "GOWORK": "off", "GOFLAGS": "", "GOENV": "off", "GOEXPERIMENT": "", "GOFIPS140": "off"}
	switch config.GOARCH {
	case "386":
		env["GO386"] = "sse2"
	case "amd64":
		env["GOAMD64"] = "v1"
	case "arm":
		env["GOARM"] = "7,hardfloat"
	case "arm64":
		env["GOARM64"] = "v8.0"
	case "mips", "mipsle":
		env["GOMIPS"] = "hardfloat"
	case "mips64", "mips64le":
		env["GOMIPS64"] = "hardfloat"
	case "ppc64", "ppc64le":
		env["GOPPC64"] = "power8"
	case "riscv64":
		env["GORISCV64"] = "rva20u64"
	case "wasm":
		env["GOWASM"] = ""
	case "loong64", "s390x": // No architecture feature environment in Go1.25.
	default:
		return nil, fmt.Errorf("unsupported Go1.25 target architecture %s", config.GOARCH)
	}
	return env, nil
}
