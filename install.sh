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

REPO="m0n0x41d/quint-code"
BIN_NAME="haft"
BIN_DIRS=("$HOME/.local/bin" "/usr/local/bin")

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

main() {
    print_logo
    printf "${CYAN}${BOLD}   Installing Haft...${RESET}\n\n"

    local tmp_dir bin_dir os_arch
    tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" EXIT
    bin_dir=$(find_bin_dir)
    os_arch=$(get_os_arch)

    # Try downloading release
    local api_url="https://api.github.com/repos/${REPO}/releases/latest"
    local download_url
    download_url=$(curl -s "$api_url" | grep "browser_download_url.*${os_arch}.tar.gz" | sed -E 's/.*"([^"]+)".*/\1/' | head -1)

    if [[ -n "$download_url" ]]; then
        (
            cd "$tmp_dir"
            curl -sL "$download_url" -o release.tar.gz
            tar -xzf release.tar.gz
        ) &
        spinner $! "Downloading release ($os_arch)"

        # goreleaser puts binary at archive root, not in bin/
        if [[ -f "$tmp_dir/$BIN_NAME" ]]; then
            cp "$tmp_dir/$BIN_NAME" "$bin_dir/$BIN_NAME"
        elif [[ -f "$tmp_dir/bin/$BIN_NAME" ]]; then
            cp "$tmp_dir/bin/$BIN_NAME" "$bin_dir/$BIN_NAME"
        else
            printf "${RED}   ✗ Binary not found in archive${RESET}\n"
            exit 1
        fi
        chmod +x "$bin_dir/$BIN_NAME"

        # macOS: re-sign binary locally to bypass Gatekeeper
        # Downloaded binaries with foreign ad-hoc signatures get killed
        if [[ "$(uname -s)" == "Darwin" ]]; then
            codesign --remove-signature "$bin_dir/$BIN_NAME" 2>/dev/null || true
            codesign -s - "$bin_dir/$BIN_NAME" 2>/dev/null || true
        fi
    else
        printf "${YELLOW}   ⚠ No release found, building from source...${RESET}\n"

        if ! command -v go >/dev/null 2>&1; then
            printf "${RED}   ✗ Go is not installed${RESET}\n"
            exit 1
        fi

        git clone --depth 1 "https://github.com/$REPO.git" "$tmp_dir/repo" 2>/dev/null &
        spinner $! "Cloning repository"

        (cd "$tmp_dir/repo" && go build -o "$bin_dir/$BIN_NAME" -trimpath ./cmd/haft/) &
        spinner $! "Building binary"
    fi

    printf "   ${GREEN}✓${RESET} Installed to ${WHITE}$bin_dir/$BIN_NAME${RESET}\n"

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
