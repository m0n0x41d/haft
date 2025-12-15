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
# Configuration (using indexed arrays for bash 3.x compatibility)
# ═══════════════════════════════════════════════════════════════════════════════

REPO_URL="https://github.com/m0n0x41d/quint-code"
BRANCH="main"

# Platform data (parallel arrays)
PLATFORMS=("claude" "cursor" "gemini" "codex")
PLATFORM_NAMES=("Claude Code" "Cursor" "Gemini CLI" "Codex CLI")
PLATFORM_PATHS=(".claude/commands" ".cursor/commands" ".gemini/commands" ".codex/prompts")
PLATFORM_SCOPE=("local" "local" "local" "global")
PLATFORM_EXT=("md" "md" "toml" "md")

# Selection state (0=false, 1=true)
SELECTED=(1 0 0 0)  # Claude selected by default

CURRENT_INDEX=0
INSTALL_GLOBAL=false
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

# Get platform info by index
get_platform_name() { echo "${PLATFORM_NAMES[$1]}"; }
get_platform_path() { echo "${PLATFORM_PATHS[$1]}"; }
get_platform_scope() { echo "${PLATFORM_SCOPE[$1]}"; }
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
    cprintln "$WHITE$BOLD" "    ╚══▀▀═╝  ╚═════╝ ╚═╝╚═╝  ╚═══╝   ╚═╝       ╚═════╝ ╚═════╝ ╚═════╝ ╚══════╝"
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
    local scope=$(get_platform_scope $index)
    local is_current=$([[ $index -eq $CURRENT_INDEX ]] && echo 1 || echo 0)

    # Cursor indicator
    if [[ "$is_current" == "1" ]]; then
        cprint "$BRIGHT_CYAN$BOLD" "   ▸ "
    else
        printf "     "
    fi

    # Checkbox
    if is_selected $index; then
        cprint "$BRIGHT_GREEN$BOLD" "[✓]"
    else
        cprint "$DIM" "[ ]"
    fi

    # Platform name
    if [[ "$is_current" == "1" ]]; then
        cprint "$BRIGHT_WHITE$BOLD" " $name"
    else
        cprint "$WHITE" " $name"
    fi

    # Scope badge
    if [[ "$scope" == "global" ]]; then
        cprint "$YELLOW$DIM" "  (global only)"
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
        if is_selected $i; then
            ((count++))
            [[ -n "$platforms_str" ]] && platforms_str+=", "
            platforms_str+=$(get_platform_name $i)
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
    # Read from /dev/tty to support curl | bash
    IFS= read -rsn1 key </dev/tty

    case "$key" in
        $'\x1b')  # Escape sequence start
            local seq
            read -rsn1 -t 1 seq </dev/tty
            if [[ "$seq" == "[" ]]; then
                read -rsn1 -t 1 seq </dev/tty
                case "$seq" in
                    'A') # Up arrow
                        ((CURRENT_INDEX > 0)) && ((CURRENT_INDEX--))
                        ;;
                    'B') # Down arrow
                        ((CURRENT_INDEX < ${#PLATFORMS[@]} - 1)) && ((CURRENT_INDEX++))
                        ;;
                esac
            fi
            ;;
        ' ')  # Space - toggle
            if [[ "${SELECTED[$CURRENT_INDEX]}" == "1" ]]; then
                SELECTED[$CURRENT_INDEX]=0
            else
                SELECTED[$CURRENT_INDEX]=1
            fi
            ;;
        '')  # Enter - confirm
            return 1
            ;;
        'q'|'Q')  # Quit
            return 2
            ;;
        'k')  # vim up
            ((CURRENT_INDEX > 0)) && ((CURRENT_INDEX--))
            ;;
        'j')  # vim down
            ((CURRENT_INDEX < ${#PLATFORMS[@]} - 1)) && ((CURRENT_INDEX++))
            ;;
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
            if [[ $result -eq 2 ]]; then
                cprintln "$YELLOW" "Installation cancelled."
                exit 0
            fi
            # Show logo before installation output
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

download_commands() {
    local index=$1
    local platform="${PLATFORMS[$index]}"
    local ext=$(get_platform_ext $index)
    local target_path=$(get_platform_path $index)
    local scope=$(get_platform_scope $index)
    local full_target

    if [[ "$scope" == "global" ]]; then
        full_target="$HOME/$target_path"
    elif [[ "$INSTALL_GLOBAL" == true ]]; then
        full_target="$HOME/$target_path"
    else
        full_target="$TARGET_DIR/$target_path"
    fi

    mkdir -p "$full_target"

    # Determine script location for local installs
    local script_dir=""
    if [[ -n "${BASH_SOURCE[0]}" ]]; then
        script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    fi

    local local_dist="$script_dir/dist/$platform"
    local base_url="https://raw.githubusercontent.com/m0n0x41d/quint-code/$BRANCH/dist/$platform"

    local commands=(
        "q0-init"
        "q1-hypothesize"
        "q1-extend"
        "q2-check"
        "q3-test"
        "q3-research"
        "q4-audit"
        "q5-decide"
        "q-status"
        "q-query"
        "q-decay"
        "q-reset"
    )

    for cmd in "${commands[@]}"; do
        local dest="$full_target/${cmd}.${ext}"
        local local_file="$local_dist/${cmd}.${ext}"

        # Try local dist first, then remote
        if [[ -f "$local_file" ]]; then
            cp "$local_file" "$dest"
        else
            local url="$base_url/${cmd}.${ext}"
            curl -fsSL "$url" -o "$dest" 2>/dev/null || true
        fi
    done
}

create_fpf_structure() {
    local target="$1"

    mkdir -p "$target/.fpf/evidence"
    mkdir -p "$target/.fpf/decisions"
    mkdir -p "$target/.fpf/sessions"
    mkdir -p "$target/.fpf/knowledge/L0"
    mkdir -p "$target/.fpf/knowledge/L1"
    mkdir -p "$target/.fpf/knowledge/L2"
    mkdir -p "$target/.fpf/knowledge/invalid"

    touch "$target/.fpf/evidence/.gitkeep"
    touch "$target/.fpf/decisions/.gitkeep"
    touch "$target/.fpf/sessions/.gitkeep"
    touch "$target/.fpf/knowledge/L0/.gitkeep"
    touch "$target/.fpf/knowledge/L1/.gitkeep"
    touch "$target/.fpf/knowledge/L2/.gitkeep"
    touch "$target/.fpf/knowledge/invalid/.gitkeep"
}

uninstall_commands() {
    local index=$1
    local platform="${PLATFORMS[$index]}"
    local ext=$(get_platform_ext $index)
    local target_path=$(get_platform_path $index)
    local scope=$(get_platform_scope $index)

    local commands=(
        "q0-init"
        "q1-hypothesize"
        "q1-extend"
        "q2-check"
        "q3-test"
        "q3-research"
        "q4-audit"
        "q5-decide"
        "q-status"
        "q-query"
        "q-decay"
        "q-reset"
    )

    # Always check BOTH local and global locations
    local local_path="$TARGET_DIR/$target_path"
    local global_path="$HOME/$target_path"
    local locations=("$global_path" "$local_path")

    # For global-only platforms, only check global
    if [[ "$scope" == "global" ]]; then
        locations=("$global_path")
    fi

    local removed=0
    local removed_from=""
    local checked_paths=""

    for full_target in "${locations[@]}"; do
        [[ -n "$checked_paths" ]] && checked_paths+=", "
        checked_paths+="$full_target"

        for cmd in "${commands[@]}"; do
            local file="$full_target/${cmd}.${ext}"
            if [[ -f "$file" ]]; then
                rm "$file"
                ((removed++))
                removed_from="$full_target"
            fi
        done

        # Remove directory if empty
        if [[ -d "$full_target" ]] && [[ -z "$(ls -A "$full_target")" ]]; then
            rmdir "$full_target" 2>/dev/null || true
        fi
    done

    # Return count, location, and checked paths
    if [[ $removed -gt 0 ]]; then
        echo "$removed|$removed_from|$checked_paths"
    else
        echo "0||$checked_paths"
    fi
}

uninstall_platforms() {
    echo ""
    cprintln "$BRIGHT_WHITE$BOLD" "   Uninstalling Quint Code..."
    echo ""

    local uninstalled_indices=""

    local i=0
    for platform in "${PLATFORMS[@]}"; do
        if is_selected $i; then
            local name=$(get_platform_name $i)

            local result
            result=$(uninstall_commands $i)

            # Parse result: count|removed_from|checked_paths
            local count=$(echo "$result" | cut -d'|' -f1)
            local location=$(echo "$result" | cut -d'|' -f2)
            local checked=$(echo "$result" | cut -d'|' -f3)

            if [[ "$count" -gt 0 ]]; then
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
    if [[ -n "$uninstalled_indices" ]]; then
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

    local any_local=false
    local installed_indices=""

    local i=0
    for platform in "${PLATFORMS[@]}"; do
        if is_selected $i; then
            local name=$(get_platform_name $i)
            local scope=$(get_platform_scope $i)

            (download_commands $i) &
            spinner $! "Installing $name commands"

            installed_indices="$installed_indices $i"

            if [[ "$scope" == "local" ]]; then
                any_local=true
            fi
        fi
        ((i++))
    done

    # Create .fpf structure for local installs
    if [[ "$any_local" == true && "$INSTALL_GLOBAL" == false ]]; then
        if [[ ! -d "$TARGET_DIR/.fpf" ]]; then
            (create_fpf_structure "$TARGET_DIR") &
            spinner $! "Creating .fpf/ structure"
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
        local scope=$(get_platform_scope $i)
        local loc="$TARGET_DIR/$path"
        [[ "$scope" == "global" ]] && loc="$HOME/$path"
        [[ "$INSTALL_GLOBAL" == true ]] && loc="$HOME/$path"

        cprint "$BRIGHT_GREEN" "     ✓ "
        cprint "$WHITE" "$name"
        cprintln "$DIM" " → $loc"
    done

    echo ""
    cprintln "$BRIGHT_CYAN$BOLD" "   Get started:"
    cprintln "$WHITE" "     /q0-init        Initialize FPF in your project"
    cprintln "$WHITE" "     /q-status       Check current state"
    echo ""
    cprintln "$DIM" "   Documentation: https://github.com/m0n0x41d/quint-code"
    echo ""
}

# ═══════════════════════════════════════════════════════════════════════════════
# CLI Mode (non-interactive)
# ═══════════════════════════════════════════════════════════════════════════════

print_usage() {
    echo "Quint Code Installer"
    echo ""
    echo "Usage:"
    echo "  ./install.sh              Interactive TUI mode"
    echo "  ./install.sh --claude     Install Claude Code only"
    echo "  ./install.sh --all        Install all platforms"
    echo "  ./install.sh -g           Install globally"
    echo "  ./install.sh --uninstall  Uninstall mode"
    echo ""
    echo "Platforms:"
    echo "  --claude    Claude Code (.claude/commands/)"
    echo "  --cursor    Cursor (.cursor/commands/)"
    echo "  --gemini    Gemini CLI (.gemini/commands/)"
    echo "  --codex     Codex CLI (~/.codex/prompts/)"
    echo ""
    echo "Options:"
    echo "  -g, --global     Install/uninstall from home directory"
    echo "  -u, --uninstall  Remove commands instead of installing"
    echo "  -h, --help       Show this help"
    echo ""
    echo "Examples:"
    echo "  ./install.sh --all -g          Install all platforms globally"
    echo "  ./install.sh --uninstall --all Uninstall all platforms (local)"
    echo "  ./install.sh -u -g --cursor    Uninstall Cursor globally"
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
            -g|--global)
                INSTALL_GLOBAL=true
                shift
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
            --codex)
                cli_mode=true
                SELECTED[3]=1
                shift
                ;;
            --all)
                cli_mode=true
                SELECTED=(1 1 1 1)
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
