#!/bin/bash
# Quint Code Installer
#
# This script installs the Quint Code FPF engine. It automatically detects
# your OS and architecture, downloads the latest pre-compiled binary and
# associated command/agent assets from GitHub Releases, and installs them.
#
# It also provides a fallback to build from source if Go is installed.
#
# Usage (One-liner):
# curl -fsSL https://raw.githubusercontent.com/m0n0x41d/quint-code/main/install.sh | bash

set -e

# ═══════════════════════════════════════════════════════════════════════════════
# ANSI Colors & Styles
# ═══════════════════════════════════════════════════════════════════════════════

BOLD='\033[1m'
DIM='\033[2m'
RESET='\033[0m'
RED='\033[31m'
GREEN='\033[32m'
YELLOW='\033[33m'
CYAN='\033[36m'
WHITE='\033[37m'
BRIGHT_GREEN='\033[92m'
BRIGHT_CYAN='\033[96m'

# ═══════════════════════════════════════════════════════════════════════════════
# Configuration
# ═══════════════════════════════════════════════════════════════════════════════

REPO="m0n0x41d/quint-code"
INSTALL_DIR_BASE=".quint"

# For TUI - command installation targets
PLATFORMS=("claude" "cursor" "gemini")
PLATFORM_NAMES=("Claude Code" "Cursor" "Gemini CLI")
PLATFORM_PATHS=(".claude/commands" ".cursor/commands" ".gemini/commands")
PLATFORM_EXTS=("md" "md" "toml")

SELECTED=(1 0 0)
CURRENT_INDEX=0
UNINSTALL_MODE=false
TARGET_DIR="$(pwd)"

# ═══════════════════════════════════════════════════════════════════════════════
# Utility Functions
# ═══════════════════════════════════════════════════════════════════════════════

hide_cursor() { printf '\033[?25l'; }
show_cursor() { printf '\033[?25h'; }
clear_screen() { printf '\033[2J\033[H'; }

cprint() {
    local color="$1"; shift
    printf "${color}%s${RESET}" "$*"
}
cprintln() {
    local color="$1"; shift
    printf "${color}%s${RESET}\n" "$*"
}

spinner() {
    local pid=$1
    local message=$2
    local spin='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
    local i=0

    while kill -0 "$pid" 2>/dev/null; do
        printf "\r   ${CYAN}${spin:i++%${#spin}:1}${RESET} %s" "$message"
        sleep 0.1
    done
    printf "\r   ${GREEN}✓${RESET} %s\n" "$message"
}

# ═══════════════════════════════════════════════════════════════════════════════
# TUI - Identical to the previous version for user familiarity
# ═══════════════════════════════════════════════════════════════════════════════

print_logo() {
    local ORANGE='\033[38;5;208m'
    local DARK_ORANGE='\033[38;5;202m'
    local LIGHT_YELLOW='\033[38;5;228m'
    echo ""
    cprintln "$RED$BOLD" "    ██████╗ ██╗   ██╗██╗███╗   ██╗████████╗    ██████╗ ██████╗ ██████╗ ███████╗"
    cprintln "$DARK_ORANGE$BOLD" "   ██╔═══██╗██║   ██║██║████╗  ██║╚══██╔══╝   ██╔════╝██╔═══██╗██╔══██╗██╔════╝"
    cprintln "$ORANGE$BOLD" "   ██║   ██║██║   ██║██║██╔██╗ ██║   ██║      ██║     ██║   ██║██║  ██║█████╗  "
    cprintln "$YELLOW$BOLD" "   ██║▄▄ ██║██║   ██║██║██║╚██╗██║   ██║      ██║     ██║   ██║██║  ██║██╔══╝  "
    cprintln "$LIGHT_YELLOW$BOLD" "   ╚██████╔╝╚██████╔╝██║██║ ╚████║   ██║      ╚██████╗╚██████╔╝██████╔╝███████╗"
    cprintln "$WHITE$BOLD" "    ╚══▀▀═╝  ╚═════╝ ╚═╝╚═╝  ╚═══╝   ╚═╝       ╚═════╝ ╚═════╝ ╚══════╝ ╚══════╝"
    echo ""
    cprintln "$DIM" "       Distilled First Principles Framework for AI Tools"
    echo ""
}

print_instructions() {
    cprint "$DIM" "      "; cprint "$CYAN$BOLD" "↑↓/jk"; cprint "$DIM" " Navigate  "; cprint "$WHITE$BOLD" "Space"; cprint "$DIM" " Toggle  "; cprint "$GREEN$BOLD" "Enter"; cprint "$DIM" " Confirm  "; cprint "$RED$BOLD" "q"; cprintln "$DIM" " Quit"
    echo ""
}

print_platform_item() {
    local index=$1; local name="${PLATFORM_NAMES[$index]}"; local is_current=$([[ $index -eq $CURRENT_INDEX ]] && echo 1 || echo 0)
    if [[ "$is_current" == "1" ]]; then cprint "$BRIGHT_CYAN$BOLD" "   ▸ "; else printf "     "; fi
    if [[ "${SELECTED[$index]}" == "1" ]]; then cprint "$BRIGHT_GREEN$BOLD" "[✓]"; else cprint "$DIM" "[ ]"; fi
    if [[ "$is_current" == "1" ]]; then cprint "$BRIGHT_WHITE$BOLD" " $name"; else cprint "$WHITE" " $name"; fi
    echo ""
}

print_selection() {
    cprintln "$WHITE" "   Select AI coding tools to install Quint commands for:"
    echo ""
    for i in "${!PLATFORMS[@]}"; do print_platform_item $i; done
    echo ""
}

handle_input() {
    local key
    IFS= read -rsn1 key </dev/tty
    case "$key" in
        $'\x1b')
            local seq
            read -rsn1 -t 1 seq </dev/tty
            if [[ "$seq" == "[" ]]; then
                read -rsn1 -t 1 seq </dev/tty
                case "$seq" in
                    'A') ((CURRENT_INDEX > 0)) && ((CURRENT_INDEX--)) ;;
                    'B') ((CURRENT_INDEX < ${#PLATFORMS[@]} - 1)) && ((CURRENT_INDEX++)) ;;
                esac
            fi
            ;;
        ' ')
            if [[ "${SELECTED[$CURRENT_INDEX]}" == "1" ]]; then
                SELECTED[$CURRENT_INDEX]=0
            else
                SELECTED[$CURRENT_INDEX]=1
            fi
            ;;
        '')
            return 1
            ;;
        'q'|'Q')
            return 2
            ;;
        'k')
            ((CURRENT_INDEX > 0)) && ((CURRENT_INDEX--))
            ;;
        'j')
            ((CURRENT_INDEX < ${#PLATFORMS[@]} - 1)) && ((CURRENT_INDEX++))
            ;;
    esac
    return 0
}

run_tui() {
    hide_cursor; trap 'show_cursor' EXIT
    while true; do
        clear_screen; print_logo; print_instructions; print_selection
        if ! handle_input; then
            local result=$?; show_cursor; clear_screen
            if [[ $result -eq 2 ]]; then cprintln "$YELLOW" "Installation cancelled."; exit 0; fi
            print_logo; break
        fi
    done
}

# ═══════════════════════════════════════════════════════════════════════════════
# Core Installation Logic
# ═══════════════════════════════════════════════════════════════════════════════

# Detects OS and architecture
get_os_arch() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    case "$arch" in
        x86_64) arch="amd64" ;; 
        aarch64|arm64) arch="arm64" ;; 
        *) cprintln "$YELLOW" "   ⚠ Unsupported architecture: $arch"; exit 1 ;; 
    esac
    echo "${os}-${arch}"
}

# Downloads and extracts the latest release archive
download_and_extract() {
    local dest_dir="$1"
    local os_arch
    os_arch=$(get_os_arch)
    
    local api_url="https://api.github.com/repos/${REPO}/releases/latest"
    
    # Use grep and sed for portability instead of jq
    local download_url
    download_url=$(curl -s "$api_url" | grep "browser_download_url.*${os_arch}.tar.gz" | sed -E 's/.*"([^"]+)".*/\1/')

    if [[ -z "$download_url" ]]; then
        cprintln "$RED" "   ✗ Could not find a release asset for your platform ($os_arch)."
        cprintln "$DIM" "   URL: $api_url"
        cprintln "$DIM" "   Looking for: *${os_arch}.tar.gz"
        return 1
    fi
    
    local filename
    filename=$(basename "$download_url")

    (
        cd "$dest_dir"
        curl -# -L "$download_url" -o "$filename"
        tar -xzf "$filename"
        rm "$filename"
    ) & 
    spinner $! "Downloading and extracting latest release ($filename)"
    return 0
}

# Copies commands from the extracted archive to the final destinations
copy_commands() {
    local source_base="$1"
    
    local i=0
    for platform in "${PLATFORMS[@]}"; do
        if [[ "${SELECTED[$i]}" == "1" ]]; then
            local p_name="${PLATFORM_NAMES[$i]}"
            local p_path="${PLATFORM_PATHS[$i]}"
            local p_ext="${PLATFORM_EXTS[$i]}"
            local full_target_path="$TARGET_DIR/$p_path"
            
            (
                mkdir -p "$full_target_path"
                # The archive contains a 'commands' directory with subdirs for each platform
                local source_dir="$source_base/commands/$platform"
                if [[ -d "$source_dir" ]]; then
                    cp "$source_dir"/*."$p_ext" "$full_target_path/"
                else
                    # Fallback for older archive structure if needed, or just note it.
                    # In our new release.yml, this structure is guaranteed.
                    : # No-op, just to show where logic would go
                fi
            ) & 
            spinner $! "Installing commands for $p_name"
        fi
        ((i++))
    done
}

# Creates the base .quint directory structure
create_quint_structure() {
    local target="$1"
    mkdir -p "$target/$INSTALL_DIR_BASE/bin"
    mkdir -p "$target/$INSTALL_DIR_BASE/evidence"
    mkdir -p "$target/$INSTALL_DIR_BASE/decisions"
    mkdir -p "$target/$INSTALL_DIR_BASE/sessions"
    mkdir -p "$target/$INSTALL_DIR_BASE/knowledge/L0"
    mkdir -p "$target/$INSTALL_DIR_BASE/knowledge/L1"
    mkdir -p "$target/$INSTALL_DIR_BASE/knowledge/L2"
    mkdir -p "$target/$INSTALL_DIR_BASE/knowledge/invalid"
    touch "$target/$INSTALL_DIR_BASE/evidence/.gitkeep"
}

# Configures the .mcp.json file
configure_mcp() {
    local target_dir="$1"
    local config_path="$target_dir/.mcp.json"
    local mcp_binary="$target_dir/$INSTALL_DIR_BASE/bin/quint-mcp"
    
    # Ensure absolute path for binary in config
    if [[ "$mcp_binary" != /* ]]; then
        mcp_binary="$(cd "$(dirname "$mcp_binary")" && pwd)/$(basename "$mcp_binary")"
    fi

    local server_json='{"quint-code":{"command":"'"$mcp_binary"'","args":["-mode","server"],"env":{}}}'

    if [[ -f "$config_path" ]]; then
        cprintln "$DIM" "   Merging MCP config into $config_path..."
        if command -v python3 >/dev/null 2>&1; then
            python3 -c "
import json, os
try:
    with open('$config_path', 'r') as f: data = json.load(f)
except Exception: data = {}
if 'mcpServers' not in data: data['mcpServers'] = {}
new_server = json.loads('$server_json')
data['mcpServers'].update(new_server)
with open('$config_path', 'w') as f: json.dump(data, f, indent=2)
"
        else
            cprintln "$YELLOW" "   ⚠ Python3 not found. Cannot merge JSON. Please add manually."
        fi
    else
        cprintln "$DIM" "   Creating new MCP config at $config_path..."
        echo '{"mcpServers": '"$server_json"'}' > "$config_path"
    fi
}

install() {
    cprintln "$BRIGHT_CYAN$BOLD" "   Starting Quint Code Installation..."
    echo ""
    
    local quint_dir="$TARGET_DIR/$INSTALL_DIR_BASE"
    create_quint_structure "$TARGET_DIR"
    
    if ! download_and_extract "$quint_dir"; then
        cprintln "$YELLOW" "   ⚠ Could not download pre-built package. Attempting to build from source..."
        if command -v go >/dev/null 2>&1; then
            local src_dir="src/mcp"
            if [[ ! -d "$src_dir" ]]; then 
                cprintln "$RED" "   ✗ Go is installed, but src/mcp not found. Cannot build from source."; 
                exit 1; 
            fi
            (cd "$src_dir" && go build -o "$quint_dir/bin/quint-mcp" -trimpath .)
            spinner $! "Compiling quint-mcp binary"
            
            # Run build.sh to generate commands if not present
            if [[ ! -d "dist" ]]; then ./build.sh; fi
            cp -r dist/* "$quint_dir/commands/"

        else
            cprintln "$RED" "   ✗ Go is not installed. Cannot build from source."
            cprintln "$RED" "   Installation failed. Please install Go or check network connection."
            exit 1
        fi
    fi
    
    copy_commands "$quint_dir"
    configure_mcp "$TARGET_DIR"

    echo ""
    cprintln "$GREEN" "    ╔══════════════════════════════════════════════════════════╗"
    cprintln "$GREEN" "    ║                                                          ║"
    cprintln "$GREEN" "    ║              ✓  Installation Complete!                   ║"
    cprintln "$GREEN" "    ║                                                          ║"
    cprintln "$GREEN" "    ╚══════════════════════════════════════════════════════════╝"
    echo ""
    cprintln "$BRIGHT_CYAN$BOLD" "   Get started by running: ${BOLD}/q0-init${RESET}"
    echo ""
}

# ═══════════════════════════════════════════════════════════════════════════════
# Main Execution
# ═══════════════════════════════════════════════════════════════════════════════

main() {
    local cli_mode=false
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help) print_usage; exit 0 ;;
            -u|--uninstall) UNINSTALL_MODE=true; shift ;;
            --claude) cli_mode=true; SELECTED=(1 0 0); shift ;;
            --cursor) cli_mode=true; SELECTED=(0 1 0); shift ;;
            --gemini) cli_mode=true; SELECTED=(0 0 1); shift ;;
            --all) cli_mode=true; SELECTED=(1 1 1); shift ;;
            *) TARGET_DIR="$1"; shift ;;
        esac
    done

    if [[ "$cli_mode" == false ]]; then
        if [[ -t 0 && -t 1 ]] || [[ -c /dev/tty ]]; then
            run_tui
        fi
    fi

    # ... rest of the logic for install/uninstall based on flags/TUI
    install
}

# Call main with all script arguments
main "$@"