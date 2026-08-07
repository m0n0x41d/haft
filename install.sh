#!/bin/bash
# Haft Installer
#
# Installs the haft binary globally.
# After installation, run `haft init` in each project.
#
# Usage:
#   curl -fsSL https://quint.codes/install.sh | bash

set -e

BOLD='\033[1m'
DIM='\033[2m'
RESET='\033[0m'
RED='\033[31m'
GREEN='\033[32m'
YELLOW='\033[33m'
CYAN='\033[36m'
WHITE='\033[37m'

REPO="m0n0x41d/haft"
BIN_NAME="haft"
BIN_DIRS=("$HOME/.local/bin" "/usr/local/bin")
LEGACY_OPEN_SLEIGH_INSTALL_DIR="$HOME/.haft/runtimes/open-sleigh/current"
HAFT_EMBED_INSTALL_DIR="$HOME/.haft/runtimes/haft-embed/current"
# Internal validation seam: install one already-built local release archive
# without consulting GitHub. Normal user installs leave this unset.
INSTALL_ARCHIVE_OVERRIDE="${HAFT_INSTALL_ARCHIVE:-}"

print_logo() {
    local ORANGE='\033[38;5;208m'
    local DARK_ORANGE='\033[38;5;202m'
    local LIGHT_YELLOW='\033[38;5;228m'
    echo ""
    printf "${RED}${BOLD}   ██╗  ██╗ █████╗ ███████╗████████╗${RESET}\n"
    printf "${DARK_ORANGE}${BOLD}   ██║  ██║██╔══██╗██╔════╝╚══██╔══╝${RESET}\n"
    printf "${ORANGE}${BOLD}   ███████║███████║█████╗     ██║   ${RESET}\n"
    printf "${YELLOW}${BOLD}   ██╔══██║██╔══██║██╔══╝     ██║   ${RESET}\n"
    printf "${LIGHT_YELLOW}${BOLD}   ██║  ██║██║  ██║██║        ██║   ${RESET}\n"
    printf "${WHITE}${BOLD}   ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝        ╚═╝   ${RESET}\n"
    echo ""
    printf "${DIM}       Decision engineering for AI coding tools${RESET}\n"
    echo ""
}

spinner() {
    local pid=$1 message=$2
    local spin='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏' i=0
    while kill -0 "$pid" 2>/dev/null; do
        printf "\r   ${CYAN}${spin:i++%${#spin}:1}${RESET} %s" "$message"
        sleep 0.1
    done
    printf "\r   ${GREEN}✓${RESET} %s\n" "$message"
}

get_os_arch() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    case "$arch" in
        x86_64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) printf "${RED}   ✗ Unsupported architecture: $arch${RESET}\n"; exit 1 ;;
    esac
    echo "${os}-${arch}"
}

find_bin_dir() {
    for dir in "${BIN_DIRS[@]}"; do
        if [[ -d "$dir" && -w "$dir" ]]; then
            echo "$dir"
            return 0
        fi
    done
    mkdir -p "$HOME/.local/bin"
    echo "$HOME/.local/bin"
}

find_archive_binary() {
    local archive_root="$1"
    local candidates=(
        "$archive_root/$BIN_NAME"
        "$archive_root/bin/$BIN_NAME"
    )

    local candidate
    for candidate in "${candidates[@]}"; do
        if [[ -f "$candidate" ]]; then
            echo "$candidate"
            return 0
        fi
    done

    return 1
}

require_source_build_toolchain() {
    if ! command -v go >/dev/null 2>&1; then
        printf "${RED}   ✗ Go is not installed${RESET}\n"
        exit 1
    fi
}

# The embedding sidecar (haft-embed) powers optional hybrid semantic recall.
# It is OPTIONAL: when absent, haft search degrades to FTS5+PPR. A missing
# toolchain or binary warns and continues rather than aborting installation.
find_archive_haft_embed_runtime() {
    local archive_root="$1"
    local candidates=(
        "$archive_root/runtimes/haft-embed/bin/haft-embed"
        "$archive_root/haft-embed"
        "$archive_root/bin/haft-embed"
    )

    local candidate
    for candidate in "${candidates[@]}"; do
        if [[ -x "$candidate" ]]; then
            echo "$candidate"
            return 0
        fi
    done

    return 1
}

install_haft_embed_runtime_from_binary() {
    local binary="$1"

    if [[ ! -x "$binary" ]]; then
        printf "${YELLOW}   ⚠ haft-embed binary not found at $binary — semantic recall disabled${RESET}\n"
        return 1
    fi

    rm -rf "$HAFT_EMBED_INSTALL_DIR"
    mkdir -p "$HAFT_EMBED_INSTALL_DIR/bin"
    cp "$binary" "$HAFT_EMBED_INSTALL_DIR/bin/haft-embed"
    chmod +x "$HAFT_EMBED_INSTALL_DIR/bin/haft-embed"
}

build_haft_embed_runtime_from_source() {
    local repo_dir="$1"

    if ! command -v cargo >/dev/null 2>&1; then
        printf "${YELLOW}   ⚠ Rust/cargo not found — skipping optional embedding sidecar${RESET}\n"
        printf "${DIM}   Install Rust (https://rustup.rs) and re-run to enable hybrid semantic search; search works without it (FTS5+PPR).${RESET}\n"
        return 0
    fi

    (
        cd "$repo_dir/embed-sidecar"
        cargo build --release
    ) &
    spinner $! "Building embedding sidecar (haft-embed)"
    install_haft_embed_runtime_from_binary "$repo_dir/embed-sidecar/target/release/haft-embed" || true
}

install_from_release_archive() {
    local archive_root="$1"
    local bin_dir="$2"
    local archive_binary
    local archive_haft_embed

    archive_binary=$(find_archive_binary "$archive_root") || {
        printf "${RED}   ✗ Binary not found in archive${RESET}\n"
        exit 1
    }
    cp "$archive_binary" "$bin_dir/$BIN_NAME"
    chmod +x "$bin_dir/$BIN_NAME"

    if archive_haft_embed=$(find_archive_haft_embed_runtime "$archive_root"); then
        install_haft_embed_runtime_from_binary "$archive_haft_embed" || true
    else
        printf "${DIM}   ⓘ Embedding sidecar not in release archive — semantic recall optional, search uses FTS5+PPR${RESET}\n"
    fi
}

install_extracted_release() {
    local archive_root="$1"
    local bin_dir="$2"

    install_from_release_archive "$archive_root" "$bin_dir"

    # macOS: re-sign binary locally to bypass Gatekeeper. Downloaded binaries
    # with foreign ad-hoc signatures can otherwise be killed on launch.
    if [[ "$(uname -s)" == "Darwin" ]]; then
        codesign --remove-signature "$bin_dir/$BIN_NAME" 2>/dev/null || true
        codesign -s - "$bin_dir/$BIN_NAME" 2>/dev/null || true
    fi
}

install_from_source_checkout() {
    local repo_dir="$1"
    local bin_dir="$2"

    (
        cd "$repo_dir"
        go build -o "$bin_dir/$BIN_NAME" -trimpath ./cmd/haft/
    ) &
    spinner $! "Building binary"

    build_haft_embed_runtime_from_source "$repo_dir"
}

remove_managed_legacy_open_sleigh_runtime() {
    local expected="$HOME/.haft/runtimes/open-sleigh/current"
    local parent

    if [[ -z "${HOME:-}" || "$LEGACY_OPEN_SLEIGH_INSTALL_DIR" != "$expected" ]]; then
        printf "${YELLOW}   ⚠ Refusing legacy runtime cleanup: managed path is not exact${RESET}\n"
        return 0
    fi

    # Refuse to traverse a symlinked parent. This leaves an unusual legacy
    # installation untouched instead of risking deletion outside Haft's tree.
    for parent in "$HOME/.haft" "$HOME/.haft/runtimes" "$HOME/.haft/runtimes/open-sleigh"; do
        if [[ -L "$parent" ]]; then
            printf "${YELLOW}   ⚠ Preserved legacy runtime because %s is a symlink${RESET}\n" "$parent"
            return 0
        fi
    done

    if [[ -e "$LEGACY_OPEN_SLEIGH_INSTALL_DIR" || -L "$LEGACY_OPEN_SLEIGH_INSTALL_DIR" ]]; then
        rm -rf -- "$LEGACY_OPEN_SLEIGH_INSTALL_DIR"
        printf "   ${GREEN}✓${RESET} Removed managed v8 runtime at ${WHITE}$LEGACY_OPEN_SLEIGH_INSTALL_DIR${RESET}\n"
    fi

    # Remove only now-empty managed parents. User-owned ~/.open-sleigh and the
    # independent haft-embed runtime are deliberately outside this path.
    rmdir -- "$HOME/.haft/runtimes/open-sleigh" 2>/dev/null || true
}

main() {
    print_logo
    printf "${CYAN}${BOLD}   Installing Haft...${RESET}\n\n"

    local tmp_dir bin_dir os_arch
    tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" EXIT
    bin_dir=$(find_bin_dir)
    os_arch=$(get_os_arch)

    if [[ -n "$INSTALL_ARCHIVE_OVERRIDE" ]]; then
        if [[ ! -f "$INSTALL_ARCHIVE_OVERRIDE" ]]; then
            printf "${RED}   ✗ HAFT_INSTALL_ARCHIVE is not a regular file: %s${RESET}\n" "$INSTALL_ARCHIVE_OVERRIDE"
            exit 1
        fi
        mkdir -p "$tmp_dir/candidate-release"
        tar -xzf "$INSTALL_ARCHIVE_OVERRIDE" -C "$tmp_dir/candidate-release"
        install_extracted_release "$tmp_dir/candidate-release" "$bin_dir"
    else
        # Try downloading the latest published release.
        local api_url="https://api.github.com/repos/${REPO}/releases/latest"
        local download_url
        # Anchor on `/haft-${os_arch}.tar.gz` so the platform-specific CLI
        # archive is selected precisely from the GitHub API response.
        download_url=$(curl -s "$api_url" \
            | grep -E "\"browser_download_url\":[[:space:]]*\".*/haft-${os_arch}\.tar\.gz\"" \
            | sed -E 's/.*"([^"]+)".*/\1/' \
            | head -1)

        if [[ -n "$download_url" ]]; then
            mkdir -p "$tmp_dir/published-release"
            (
                curl -sL "$download_url" -o "$tmp_dir/release.tar.gz"
                tar -xzf "$tmp_dir/release.tar.gz" -C "$tmp_dir/published-release"
            ) &
            spinner $! "Downloading release ($os_arch)"
            install_extracted_release "$tmp_dir/published-release" "$bin_dir"
        else
            printf "${YELLOW}   ⚠ No release found, building from source...${RESET}\n"
            require_source_build_toolchain

            git clone --depth 1 "https://github.com/$REPO.git" "$tmp_dir/repo" 2>/dev/null &
            spinner $! "Cloning repository"

            install_from_source_checkout "$tmp_dir/repo" "$bin_dir"
        fi
    fi

    remove_managed_legacy_open_sleigh_runtime

    printf "   ${GREEN}✓${RESET} Installed to ${WHITE}$bin_dir/$BIN_NAME${RESET}\n"
    if [[ -x "$HAFT_EMBED_INSTALL_DIR/bin/haft-embed" ]]; then
        printf "   ${GREEN}✓${RESET} Installed embedding sidecar to ${WHITE}$HAFT_EMBED_INSTALL_DIR${RESET} (hybrid semantic recall)\n"
    fi

    # Check PATH
    if [[ ":$PATH:" != *":$bin_dir:"* ]]; then
        echo ""
        printf "${YELLOW}   ⚠ $bin_dir is not in your PATH${RESET}\n"
        printf "${DIM}   Add to your shell profile:${RESET}\n"
        printf "${WHITE}   export PATH=\"\$PATH:$bin_dir\"${RESET}\n"
    fi

    echo ""
    printf "${GREEN}    ╔════════════════════════════════════════════════════════════╗${RESET}\n"
    printf "${GREEN}    ║             ✓  Installation Complete!                      ║${RESET}\n"
    printf "${GREEN}    ╚════════════════════════════════════════════════════════════╝${RESET}\n"
    echo ""
    printf "   ${WHITE}${BOLD}Next step:${RESET}\n"
    printf "   In your project directory, run: ${WHITE}haft init${RESET}\n"
    echo ""
}

main "$@"
