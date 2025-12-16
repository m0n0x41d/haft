#!/bin/bash
# Quint Code Installer
# Dynamic TUI for multi-platform installation
# Usage: curl -fsSL https://raw.githubusercontent.com/user/quint-code/main/install.sh | bash

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
BLUE='\033[34m'
MAGENTA='\033[35m'
CYAN='\033[36m'
WHITE='\033[37m'

BRIGHT_GREEN='\033[92m'
BRIGHT_CYAN='\033[96m'
BRIGHT_MAGENTA='\033[95m'
BRIGHT_WHITE='\033[97m'
BRIGHT_BLUE='\033[94m'

# ═══════════════════════════════════════════════════════════════════════════════
# Configuration
# ═══════════════════════════════════════════════════════════════════════════════

REPO_URL="https://github.com/m0n0x41d/quint-code"
BRANCH="main"

PLATFORMS=("claude" "cursor" "gemini")
PLATFORM_NAMES=("Claude Code" "Cursor" "Gemini CLI")
PLATFORM_PATHS=(".claude/commands" ".cursor/commands" ".gemini/commands")
PLATFORM_EXT=("md" "md" "toml")

SELECTED=(1 0 0)  # Claude selected by default

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
    local color="$1"
    shift
    printf "${color}%s${RESET}" "$*"
}

cprintln() {
    local color="$1"
    shift
    printf "${color}%s${RESET}\n" "$*"
}

get_platform_name() { echo "${PLATFORM_NAMES[$1]}"; }
get_platform_path() { echo "${PLATFORM_PATHS[$1]}"; }
get_platform_ext() { echo "${PLATFORM_EXT[$1]}"; }
is_selected() { [[ "${SELECTED[$1]}" == "1" ]]; }

# ═══════════════════════════════════════════════════════════════════════════════
# UI Components
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
    cprint "$DIM" "      "
    cprint "$CYAN$BOLD" "↑↓/jk" 
    cprint "$DIM" " Navigate  "
    cprint "$WHITE$BOLD" "Space"
    cprint "$DIM" " Toggle  "
    cprint "$GREEN$BOLD" "Enter"
    cprint "$DIM" " Install  "
    cprint "$RED$BOLD" "q"
    cprintln "$DIM" " Quit"
    echo ""
    cprint "$YELLOW" "   Tip: "
    cprintln "$DIM" "Cursor can import .claude/commands/ — install for Claude Code, use in both!"
    echo ""
}

print_platform_item() {
    local index=$1
    local name=$(get_platform_name $index)
    local is_current=$([[ $index -eq $CURRENT_INDEX ]] && echo 1 || echo 0)

    if [[ "$is_current" == "1" ]]; then
        cprint "$BRIGHT_CYAN$BOLD" "   ▸ "
    else
        printf "     "
    fi

    if is_selected $index; then
        cprint "$BRIGHT_GREEN$BOLD" "[✓]"
    else
        cprint "$DIM" "[ ]"
    fi

    if [[ "$is_current" == "1" ]]; then
        cprint "$BRIGHT_WHITE$BOLD" " $name"
    else
        cprint "$WHITE" " $name"
    fi

    echo ""
}

print_selection() {
    cprintln "$WHITE" "   Select AI coding tools to install FPF commands:"
    echo ""
    local i=0
    for platform in "${PLATFORMS[@]}"; do
        print_platform_item $i
        ((i++))
    done
    echo ""
}

print_summary() {
    local count=0
    local platforms_str=""
    local i=0
    for platform in "${PLATFORMS[@]}"; do
        if is_selected $i;
        then
            ((count++))
            [[ -n "$platforms_str" ]] && platforms_str=", "$platforms_str
            platforms_str+="$(get_platform_name $i)"
        fi
        ((i++))
    done

    if [[ $count -eq 0 ]]; then
        cprintln "$YELLOW" "   ⚠  No platforms selected"
    else
        cprint "$DIM" "   Selected: "
        cprintln "$CYAN" "$platforms_str"
    fi
}

# ═══════════════════════════════════════════════════════════════════════════════
# TUI Event Loop
# ═══════════════════════════════════════════════════════════════════════════════

handle_input() {
    local key
    IFS= read -rsn1 key </dev/tty

    case "$key" in
        $\'x1b\')
            local seq
            read -rsn1 -t 1 seq </dev/tty
            if [[ "$seq" == "[" ]]
            then
                read -rsn1 -t 1 seq </dev/tty
                case "$seq" in
                    'A') ((CURRENT_INDEX > 0)) && ((CURRENT_INDEX--));;
                    'B') ((CURRENT_INDEX < ${#PLATFORMS[@]} - 1)) && ((CURRENT_INDEX++));;
                esac
            fi
            ;;
        ' ') 
            if [[ "${SELECTED[$CURRENT_INDEX]}" == "1" ]]
            then
                SELECTED[$CURRENT_INDEX]=0
            else
                SELECTED[$CURRENT_INDEX]=1
            fi
            ;;
        '') return 1;; 
        'q'|'Q') return 2;; 
        'k') ((CURRENT_INDEX > 0)) && ((CURRENT_INDEX--));;
        'j') ((CURRENT_INDEX < ${#PLATFORMS[@]} - 1)) && ((CURRENT_INDEX++));;
    esac

    return 0
}

run_tui() {
    hide_cursor
    trap 'show_cursor' EXIT

    while true; do
        clear_screen
        print_logo
        print_instructions
        print_selection
        print_summary

        if ! handle_input; then
            local result=$?
            show_cursor
            clear_screen
            if [[ $result -eq 2 ]]
            then
                cprintln "$YELLOW" "Installation cancelled."
                exit 0
            fi
            print_logo
            break
        fi
    done
}

# ═══════════════════════════════════════════════════════════════════════════════
# Installation
# ═══════════════════════════════════════════════════════════════════════════════

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

get_os_arch() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)

    case "$arch" in
        x86_64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) arch="unknown" ;;
    esac

    echo "${os}-${arch}"
}

download_mcp_binary() {
    local target_dir="$1"
    local os_arch=$(get_os_arch)
    local binary_name="quint-mcp-${os_arch}"
    
    # Try to get latest tag if possible, otherwise use 'latest' logic or specific version
    # Since we can't easily parse JSON with just bash without jq, we might try a direct download 
    # of a specific version if known, or try 'latest/download' convention.
    
    # URL Convention: https://github.com/m0n0x41d/quint-code/releases/latest/download/quint-mcp-darwin-arm64
    local url="${REPO_URL}/releases/latest/download/${binary_name}"
    
    local dest_dir="$target_dir/.quint/bin"
    mkdir -p "$dest_dir"
    local dest_file="$dest_dir/quint-mcp"

    if curl -fsSL "$url" -o "$dest_file"; then
        chmod +x "$dest_file"
        return 0
    else
        return 1
    fi
}

download_commands() {
    local index=$1
    local platform="${PLATFORMS[$index]}"
    local ext=$(get_platform_ext $index)
    local target_path=$(get_platform_path $index)
    local full_target="$TARGET_DIR/$target_path"

    mkdir -p "$full_target"

    local script_dir=""
    if [[ -n "${BASH_SOURCE[0]}" ]]
    then
        script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    fi

    local local_dist="$script_dir/dist/$platform"
    local commands=(
        "q0-init" "q1-hypothesize" "q1-extend" "q2-check" "q3-test" "q3-research" "q4-audit" "q5-decide" 
        "q-status" "q-query" "q-decay" "q-reset" "q-actualize"
    )

    for cmd in "${commands[@]}"; do
        local dest="$full_target/${cmd}.${ext}"
        local local_file="$local_dist/${cmd}.${ext}"
        # Check src/commands for both standard and agent commands
        local src_cmd_file="$script_dir/src/commands/${cmd}.md"

        if [[ -f "$local_file" ]]
        then
            cp "$local_file" "$dest"
        elif [[ -f "$src_cmd_file" ]]
        then
            cp "$src_cmd_file" "$dest"
        else
            local url="$base_url/${cmd}.${ext}"
            curl -fsSL "$url" -o "$dest" 2>/dev/null || true
        fi
    done
}

create_quint_structure() {
    local target="$1"
    mkdir -p "$target/.quint/evidence"
    mkdir -p "$target/.quint/decisions"
    mkdir -p "$target/.quint/sessions"
    mkdir -p "$target/.quint/knowledge/L0"
    mkdir -p "$target/.quint/knowledge/L1"
    mkdir -p "$target/.quint/knowledge/L2"
    mkdir -p "$target/.quint/knowledge/invalid"
    mkdir -p "$target/.quint/agents"
    touch "$target/.quint/evidence/.gitkeep"
    touch "$target/.quint/decisions/.gitkeep"
    touch "$target/.quint/sessions/.gitkeep"
    touch "$target/.quint/knowledge/L0/.gitkeep"
    touch "$target/.quint/knowledge/L1/.gitkeep"
    touch "$target/.quint/knowledge/L2/.gitkeep"
    touch "$target/.quint/knowledge/invalid/.gitkeep"
}

uninstall_commands() {
    local index=$1
    local platform="${PLATFORMS[$index]}"
    local ext=$(get_platform_ext $index)
    local target_path=$(get_platform_path $index)
    local commands=(
        "q0-init" "q1-hypothesize" "q1-extend" "q2-check" "q3-test" "q3-research" "q4-audit" "q5-decide" 
        "q-status" "q-query" "q-decay" "q-reset" "q-actualize"
        "abductor" "deductor" "inductor" "decider" "auditor"
    )
    local local_path="$TARGET_DIR/$target_path"
    local locations=($local_path)
    local removed=0
    local removed_from=""
    local checked_paths=""

    for full_target in "${locations[@]}"; do
        [[ -n "$checked_paths" ]] && checked_paths+=', '
        checked_paths+="$full_target"
        for cmd in "${commands[@]}"; do
            local file="$full_target/${cmd}.${ext}"
            if [[ -f "$file" ]]
            then
                rm "$file"
                ((removed++))
                removed_from="$full_target"
            fi
        done
        if [[ -d "$full_target" ]] && [[ -z "$(ls -A "$full_target")" ]]
        then
            rmdir "$full_target" 2>/dev/null || true
        fi
    done

    if [[ $removed -gt 0 ]]
    then
        echo "$removed|$removed_from|$checked_paths"
    else
        echo "0||$checked_paths"
    fi
}

configure_mcp() {
    local target_dir="$1"
    local config_path="$target_dir/.mcp.json" # Project-local config
    local mcp_binary="$target_dir/.quint/bin/quint-core"
    
    # Ensure absolute path for binary
    if [[ "$mcp_binary" != /* ]]; then
        mcp_binary="$(cd "$(dirname "$mcp_binary")" && pwd)/$(basename "$mcp_binary")"
    fi

    # JSON snippet to merge
    local server_json="{\"quint-code\":{\"command\":\"$mcp_binary\",\"args\":[\"-mode\",\"mcp-server\"],\"env\":{}}}"

    if [[ -f "$config_path" ]]; then
        cprintln "$DIM" "   Merging MCP config into $config_path..."
        
        # Use python3 to merge JSON reliably without jq
        if command -v python3 >/dev/null 2>&1; then
            python3 -c "
import json
import os

try:
    with open('$config_path', 'r') as f:
        data = json.load(f)
except Exception:
    data = {}

if 'mcpServers' not in data:
    data['mcpServers'] = {}

new_server = json.loads('$server_json')
data['mcpServers'].update(new_server)

with open('$config_path', 'w') as f:
    json.dump(data, f, indent=2)
"
        else
            # Fallback if python3 is missing (unlikely on macOS/Linux devs, but safe)
            cprintln "$YELLOW" "   ⚠ Python3 not found. Cannot merge JSON automatically."
            cprintln "$WHITE" "   Please add this to $config_path manually:"
            echo "\"mcpServers\": $server_json"
        fi
    else
        cprintln "$DIM" "   Creating new MCP config at $config_path..."
        cat <<EOF > "$config_path"
{
  "mcpServers": {
    "quint-code": {
      "command": "$mcp_binary",
      "args": ["-mode", "mcp-server"],
      "env": {}
    }
  }
}
EOF
    fi
}

install_agents_internal() {
    # Copies agent profiles to .quint/agents for MCP use
    local target="$1"
    local agents_dir="$target/.quint/agents"
    mkdir -p "$agents_dir"
    
    local script_dir=""
    if [[ -n "${BASH_SOURCE[0]}" ]]
    then
        script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    fi
    
    local src_cmds="$script_dir/src/commands"
    local agents=("abductor" "deductor" "inductor" "decider" "auditor")

    for agent in "${agents[@]}"; do
        if [[ -f "$src_cmds/$agent.md" ]]
        then
            cp "$src_cmds/$agent.md" "$agents_dir/"
        fi
    done
}

uninstall_platforms() {
    echo ""
    cprintln "$BRIGHT_WHITE$BOLD" "   Uninstalling Quint Code..."
    echo ""
    local uninstalled_indices=""
    local i=0
    for platform in "${PLATFORMS[@]}"; do
        if is_selected $i
        then
            local name=$(get_platform_name $i)
            local result=$(uninstall_commands $i)
            local count=$(echo "$result" | cut -d'|' -f1)
            local location=$(echo "$result" | cut -d'|' -f2)
            local checked=$(echo "$result" | cut -d'|' -f3)

            if [[ "$count" -gt 0 ]]
            then
                cprint "$GREEN" "   ✓ "
                cprint "$WHITE" "$name"
                cprint "$DIM" " — removed $count commands from "
                cprintln "$DIM" "$location"
                uninstalled_indices="$uninstalled_indices $i"
            else
                cprint "$YELLOW" "   - "
                cprint "$DIM" "$name — no commands found"
                cprintln "$DIM" " (checked: $checked)"
            fi
        fi
        ((i++))
    done
    echo ""
    if [[ -n "$uninstalled_indices" ]]
    then
        cprintln "$BRIGHT_GREEN$BOLD" "   Uninstall complete."
    else
        cprintln "$YELLOW" "   Nothing to uninstall."
    fi
    echo ""
}

install_platforms() {
    echo ""
    cprintln "$BRIGHT_WHITE$BOLD" "   Installing Quint Code..."
    echo ""
    local installed_indices=""
    local i=0
    for platform in "${PLATFORMS[@]}"; do
        if is_selected $i
        then
            local name=$(get_platform_name $i)
            (download_commands $i) &
            spinner $! "Installing $name commands"
            
            # Install Agents to Platform Directory (e.g., .claude/agents)
            # Assumption: All platforms support an 'agents' subdirectory or we create it for structure
            local platform_root=".${PLATFORMS[$i]}" # e.g. .claude
            local agents_target="$TARGET_DIR/$platform_root/agents"
            
            # Copy agents from src/agents to platform agents dir
            if [[ -d "src/agents" ]]; then
                mkdir -p "$agents_target"
                cp src/agents/*.md "$agents_target/"
            fi

            installed_indices="$installed_indices $i"
        fi
        ((i++))
    done

    if [[ ! -d "$TARGET_DIR/.quint" ]]
    then
        (create_quint_structure "$TARGET_DIR") &
        spinner $! "Creating .quint/ structure"
    fi
    
    # Internal agent copy for MCP context lookup (Kernel Knowledge)
    if [[ -d "src/agents" ]]
    then
         (install_agents_internal "$TARGET_DIR") &
         spinner $! "Caching Agent Profiles in .quint"
    fi
    
    # Attempt to download binary first
    cprintln "$DIM" "   Installing MCP Server..."
    
    # Run in background to use spinner
    if (download_mcp_binary "$TARGET_DIR") >/dev/null 2>&1; then
        # This is a bit tricky with spinner since download_mcp_binary is a function
        # We'll just assume it worked if exit code 0
        cprintln "$GREEN" "   ✓ Downloaded pre-built quint-mcp binary"
        configure_mcp "$TARGET_DIR"
    else
        cprintln "$YELLOW" "   ⚠  Could not download pre-built binary. Falling back to source build..."
        
        if command -v go >/dev/null 2>&1
        then
            mkdir -p "$TARGET_DIR/.quint/bin"
            local src_mcp="$TARGET_DIR/src/mcp"
            if [[ ! -d "$src_mcp" && -n "${BASH_SOURCE[0]}" ]]
            then
                 src_mcp="$(dirname "${BASH_SOURCE[0]}")/src/mcp"
            fi

            if [[ -d "$src_mcp" ]]
            then
                (cd "$src_mcp" && go mod tidy) &>/dev/null || true
                (cd "$src_mcp" && go build -o "$TARGET_DIR/.quint/bin/quint-mcp" .) &
                spinner $! "Compiling quint-mcp binary"
                configure_mcp "$TARGET_DIR"
            else
                cprintln "$YELLOW" "   ⚠  Could not find src/mcp source to build server."
            fi
        else
            cprintln "$YELLOW" "   ⚠  Go not found. MCP Server not built. (Install Go to enable FSM enforcement)"
        fi
    fi

    echo ""
    print_success "$installed_indices"
}

print_success() {
    local indices="$1"
    cprintln "$GREEN" "    ╔══════════════════════════════════════════════════════════╗"
    cprintln "$GREEN" "    ║                                                          ║"
    cprintln "$GREEN" "    ║              ✓  Installation Complete!                   ║"
    cprintln "$GREEN" "    ║                                                          ║"
    cprintln "$GREEN" "    ╚══════════════════════════════════════════════════════════╝"
    echo ""
    cprintln "$WHITE" "   Installed for:"
    for i in $indices; do
        local name=$(get_platform_name $i)
        local path=$(get_platform_path $i)
        local loc="$TARGET_DIR/$path"
        cprint "$BRIGHT_GREEN" "     ✓ "
        cprint "$WHITE" "$name"
        cprintln "$DIM" " → $loc"
    done
    echo ""
    cprintln "$BRIGHT_CYAN$BOLD" "   Get started:"
    cprintln "$WHITE" "     /q0-init        Initialize FPF in your project"
    cprintln "$WHITE" "     /q-status       Check current state"
    cprintln "$WHITE" "     /abductor       Adopt Abductor persona"
    echo ""
    cprintln "$DIM" "   Documentation: https://github.com/m0n0x41d/quint-code"
    echo ""
}
print_usage() {
    echo "Quint Code Installer"
    echo ""
    echo "Usage:"
    echo "  ./install.sh              Interactive TUI mode"
    echo "  ./install.sh --claude     Install Claude Code only"
    echo "  ./install.sh --all        Install all platforms"
    echo "  ./install.sh --uninstall  Uninstall mode"
    echo ""
    echo "Platforms:"
    echo "  --claude    Claude Code (.claude/commands/)"
    echo "  --cursor    Cursor (.cursor/commands/)"
    echo "  --gemini    Gemini CLI (.gemini/commands/)"
    echo ""
    echo "Options:"
    echo "  -u, --uninstall  Remove commands instead of installing"
    echo "  -h, --help       Show this help"
    echo ""
    echo "Examples:"
    echo "  ./install.sh --all             Install all platforms (local)"
    echo "  ./install.sh --uninstall --all Uninstall all platforms (local)"
    echo "  ./install.sh -u --cursor       Uninstall Cursor"
    echo ""
}

# ═══════════════════════════════════════════════════════════════════════════════
# Main
# ═══════════════════════════════════════════════════════════════════════════════

main() {
    local cli_mode=false

    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                print_usage
                exit 0
                ;;
            -u|--uninstall)
                UNINSTALL_MODE=true
                shift
                ;;
            --claude)
                cli_mode=true
                SELECTED[0]=1
                shift
                ;;
            --cursor)
                cli_mode=true
                SELECTED[1]=1
                shift
                ;;
            --gemini)
                cli_mode=true
                SELECTED[2]=1
                shift
                ;;
            --all)
                cli_mode=true
                SELECTED=(1 1 1)
                shift
                ;;
            *)
                TARGET_DIR="$1"
                shift
                ;;
        esac
    done

    # Check if running interactively
    # Run TUI if interactive (either direct terminal or curl|bash with /dev/tty)
    if [[ "$cli_mode" == false ]]; then
        if [[ -t 0 && -t 1 ]] || [[ -c /dev/tty ]]; then
            run_tui
        fi
    fi

    # Check if any platform selected
    local any_selected=false
    for sel in "${SELECTED[@]}"; do
        if [[ "$sel" == "1" ]]; then
            any_selected=true
            break
        fi
    done

    if [[ "$any_selected" == false ]]; then
        cprintln "$YELLOW" "No platforms selected. Use --help for usage."
        exit 1
    fi

    if [[ "$UNINSTALL_MODE" == true ]]; then
        uninstall_platforms
    else
        install_platforms
    fi
}

main "$@"
