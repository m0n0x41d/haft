#!/bin/bash
# Quint Code Installer
#
# Installs:
#   1. quint-code MCP binary (FPF tool server)
#   2. quint-code VS Code extension (@quint chat participant for Copilot)
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/m0n0x41d/quint-code/main/install.sh | bash

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
BIN_NAME="quint-code"
EXT_ID="quint-code.quint-code"
BIN_DIRS=("$HOME/.local/bin" "/usr/local/bin")

print_logo() {
    local ORANGE='\033[38;5;208m'
    local DARK_ORANGE='\033[38;5;202m'
    local LIGHT_YELLOW='\033[38;5;228m'
    echo ""
    printf "${RED}${BOLD}    ██████╗ ██╗   ██╗██╗███╗   ██╗████████╗    ██████╗ ██████╗ ██████╗ ███████╗${RESET}\n"
    printf "${DARK_ORANGE}${BOLD}   ██╔═══██╗██║   ██║██║████╗  ██║╚══██╔══╝   ██╔════╝██╔═══██╗██╔══██╗██╔════╝${RESET}\n"
    printf "${ORANGE}${BOLD}   ██║   ██║██║   ██║██║██╔██╗ ██║   ██║      ██║     ██║   ██║██║  ██║█████╗  ${RESET}\n"
    printf "${YELLOW}${BOLD}   ██║▄▄ ██║██║   ██║██║██║╚██╗██║   ██║      ██║     ██║   ██║██║  ██║██╔══╝  ${RESET}\n"
    printf "${LIGHT_YELLOW}${BOLD}   ╚██████╔╝╚██████╔╝██║██║ ╚████║   ██║      ╚██████╗╚██████╔╝██████╔╝███████╗${RESET}\n"
    printf "${WHITE}${BOLD}    ╚══▀▀═╝  ╚═════╝ ╚═╝╚═╝  ╚═══╝   ╚═╝       ╚═════╝ ╚═════╝ ╚══════╝ ╚══════╝${RESET}\n"
    echo ""
    printf "${DIM}       First Principles Framework for AI-assisted engineering${RESET}\n"
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

find_vscode_cli() {
    for cmd in code code-insiders codium; do
        if command -v "$cmd" >/dev/null 2>&1; then
            echo "$cmd"
            return 0
        fi
    done
    return 1
}

install_vscode_extension() {
    local repo_dir="$1"
    local vscode_cli

    vscode_cli=$(find_vscode_cli) || {
        printf "   ${YELLOW}⚠ VS Code CLI not found, skipping extension install${RESET}\n"
        printf "   ${DIM}Install VS Code and re-run, or install the extension manually${RESET}\n"
        return 0
    }

    if ! command -v node >/dev/null 2>&1; then
        printf "   ${YELLOW}⚠ Node.js not found, skipping extension install${RESET}\n"
        printf "   ${DIM}Install Node.js (>=18) and re-run to install the VS Code extension${RESET}\n"
        return 0
    fi

    local ext_dir="$repo_dir"

    (cd "$ext_dir" && npm install --ignore-scripts 2>/dev/null && npm run build 2>/dev/null) &
    spinner $! "Building VS Code extension"

    local ext_install_dir
    case "$(uname -s)" in
        Darwin) ext_install_dir="$HOME/.vscode/extensions/${EXT_ID}-0.1.0" ;;
        *)      ext_install_dir="$HOME/.vscode/extensions/${EXT_ID}-0.1.0" ;;
    esac

    rm -rf "$ext_install_dir"
    mkdir -p "$ext_install_dir"

    cp "$ext_dir/package.json" "$ext_install_dir/"
    cp -r "$ext_dir/dist" "$ext_install_dir/"
    cp -r "$ext_dir/media" "$ext_install_dir/"

    printf "   ${GREEN}✓${RESET} VS Code extension installed to ${WHITE}${ext_install_dir}${RESET}\n"
    printf "   ${DIM}Reload VS Code to activate @quint in Copilot Chat${RESET}\n"
}

main() {
    print_logo
    printf "${CYAN}${BOLD}   Installing Quint Code...${RESET}\n\n"

    local tmp_dir bin_dir os_arch
    tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" EXIT
    bin_dir=$(find_bin_dir)
    os_arch=$(get_os_arch)

    local repo_cloned=false

    # --- 1. Install MCP binary ---
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

        cp "$tmp_dir/bin/$BIN_NAME" "$bin_dir/$BIN_NAME"
        chmod +x "$bin_dir/$BIN_NAME"

        # macOS: re-sign binary locally to bypass Gatekeeper
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
        repo_cloned=true

        (cd "$tmp_dir/repo/src/mcp" && go build -o "$bin_dir/$BIN_NAME" -trimpath .) &
        spinner $! "Building binary"
    fi

    printf "   ${GREEN}✓${RESET} MCP server installed to ${WHITE}$bin_dir/$BIN_NAME${RESET}\n"

    # Check PATH
    if [[ ":$PATH:" != *":$bin_dir:"* ]]; then
        echo ""
        printf "${YELLOW}   ⚠ $bin_dir is not in your PATH${RESET}\n"
        printf "${DIM}   Add to your shell profile:${RESET}\n"
        printf "${WHITE}   export PATH=\"\$PATH:$bin_dir\"${RESET}\n"
        echo ""
    fi

    # --- 2. Install VS Code extension ---
    if [[ "$repo_cloned" != true ]]; then
        git clone --depth 1 "https://github.com/$REPO.git" "$tmp_dir/repo" 2>/dev/null &
        spinner $! "Cloning repository"
    fi

    install_vscode_extension "$tmp_dir/repo"

    echo ""
    printf "${GREEN}    ╔════════════════════════════════════════════════════════════╗${RESET}\n"
    printf "${GREEN}    ║             ✓  Installation Complete!                      ║${RESET}\n"
    printf "${GREEN}    ╚════════════════════════════════════════════════════════════╝${RESET}\n"
    echo ""
    printf "   ${WHITE}${BOLD}Next steps:${RESET}\n"
    printf "   1. cd /path/to/your/project && ${WHITE}quint-code init${RESET}\n"
    printf "   2. Open VS Code and use ${WHITE}@quint${RESET} in Copilot Chat\n"
    echo ""
}

main "$@"
