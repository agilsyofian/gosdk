#!/bin/bash

# ==============================================================================
# Script Name : release.sh
# Location    : SDK/release.sh
# Description : Script to create or update git tags for Go SDK monorepo modules
#               (github.com/agilsyofian/gosdk).
#               Supports interactive, single-module, and batch (--all) modes,
#               with optional remote push support.
# ==============================================================================

set -e

# Terminal color formatting
if [ -t 1 ]; then
    COLOR_RESET="\033[0m"
    COLOR_BOLD="\033[1m"
    COLOR_GREEN="\033[0;32m"
    COLOR_YELLOW="\033[1;33m"
    COLOR_BLUE="\033[0;34m"
    COLOR_CYAN="\033[0;36m"
    COLOR_RED="\033[0;31m"
else
    COLOR_RESET=""
    COLOR_BOLD=""
    COLOR_GREEN=""
    COLOR_YELLOW=""
    COLOR_BLUE=""
    COLOR_CYAN=""
    COLOR_RED=""
fi

# 1. Ensure the script always runs from the root of the SDK repository
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR" || exit 1

# 2. Verify we are inside a Git repository
if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo -e "${COLOR_RED}Error: This directory is not a Git repository.${COLOR_RESET}" >&2
    exit 1
fi

# Helper: Retrieve list of all modules (relative subpaths)
get_all_modules() {
    find . -name "go.mod" -not -path "*/vendor/*" | sort | while IFS= read -r f; do
        dir=$(dirname "$f" | sed 's|^\./||')
        echo "$dir"
    done
}

# Helper: Get the latest tag for a module
get_latest_tag() {
    local mod="$1"
    git tag -l "${mod}/v*" --sort=-v:refname 2>/dev/null | head -n 1
}

# Helper: Suggest the next patch version (e.g. v1.0.0 -> v1.0.1)
suggest_next_version() {
    local latest_tag="$1"
    if [ -z "$latest_tag" ]; then
        echo "v1.0.0"
        return
    fi
    local ver="${latest_tag##*/}"
    ver="${ver#v}"
    if [[ "$ver" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
        local major="${BASH_REMATCH[1]}"
        local minor="${BASH_REMATCH[2]}"
        local patch="${BASH_REMATCH[3]}"
        local next_patch=$((patch + 1))
        echo "v${major}.${minor}.${next_patch}"
    else
        echo "v1.0.0"
    fi
}

# Helper: Normalize and validate semver format (vX.Y.Z or X.Y.Z)
normalize_version() {
    local ver="$1"
    if [[ ! "$ver" =~ ^v ]]; then
        ver="v${ver}"
    fi
    if [[ ! "$ver" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
        echo ""
    else
        echo "$ver"
    fi
}

# Helper: Display usage instructions
show_help() {
    echo -e "${COLOR_BOLD}Go SDK Monorepo Release & Tagging Tool${COLOR_RESET}"
    echo "========================================================================"
    echo -e "${COLOR_BOLD}Usage:${COLOR_RESET}"
    echo "  ./release.sh                              (Interactive Mode)"
    echo "  ./release.sh <module> <version> [options]"
    echo "  ./release.sh <module>/<version> [options]"
    echo "  ./release.sh --all <version> [options]    (Batch Release All Modules)"
    echo ""
    echo -e "${COLOR_BOLD}Options:${COLOR_RESET}"
    echo "  -a, --all <version>          Tag all SDK modules simultaneously to target version"
    echo "  -p, --push                   Push tag(s) to remote repository after tagging"
    echo "  -r, --remote <name>          Git remote name (default: origin)"
    echo "  -c, --commit <hash|branch>   Target commit/branch (default: HEAD)"
    echo "  -m, --message <msg>          Annotated tag message"
    echo "  -f, --force                  Force update existing tag"
    echo "  -d, --dry-run                Simulate tag creation without running git tag"
    echo "  -l, --list                   Display table of all modules and their latest tags"
    echo "  -h, --help                   Display this help message"
    echo ""
    echo -e "${COLOR_BOLD}Examples:${COLOR_RESET}"
    echo "  ./release.sh"
    echo "  ./release.sh storage/objectstorage/s3 v1.0.0"
    echo "  ./release.sh storage/database/postgres v1.0.0 -p"
    echo "  ./release.sh --all v1.0.0 -m \"Initial Release v1.0.0 for all Go SDK modules\""
    echo "  ./release.sh --all v1.0.0 --push"
    echo "  ./release.sh --list"
}

# Helper: Display list of all modules and latest tags
list_modules() {
    echo -e "${COLOR_BOLD}SDK Modules & Current Tags:${COLOR_RESET}"
    echo "--------------------------------------------------------------------------------"
    printf "%-3s %-32s %-22s %-15s\n" "No" "Module Subpath" "Latest Tag" "Suggested Next"
    echo "--------------------------------------------------------------------------------"
    local idx=1
    while IFS= read -r mod; do
        [ -z "$mod" ] && continue
        local latest
        latest="$(get_latest_tag "$mod")"
        local next
        next="$(suggest_next_version "$latest")"
        if [ -z "$latest" ]; then
            latest="${COLOR_YELLOW}(no tag yet)${COLOR_RESET}"
        else
            latest="${COLOR_GREEN}${latest}${COLOR_RESET}"
        fi
        printf "%-3d %-32s %-31b %-15s\n" "$idx" "$mod" "$latest" "$next"
        idx=$((idx + 1))
    done < <(get_all_modules)
    echo "--------------------------------------------------------------------------------"
}

# Helper: Push tag to remote repository
push_tag() {
    local tag_name="$1"
    local remote_name="$2"
    local is_dry_run="$3"

    if [ "$is_dry_run" = true ]; then
        echo -e "${COLOR_CYAN}[DRY-RUN] git push ${remote_name} ${tag_name}${COLOR_RESET}"
        return 0
    fi

    # Check if remote exists
    if ! git remote get-url "$remote_name" >/dev/null 2>&1; then
        echo -e "${COLOR_YELLOW}Warning: Remote '${remote_name}' is not configured. Skipping remote push.${COLOR_RESET}"
        return 0
    fi

    echo -e "${COLOR_BLUE}Pushing tag '${tag_name}' to remote '${remote_name}'...${COLOR_RESET}"
    git push "$remote_name" "$tag_name"
    echo -e "${COLOR_GREEN}✓ Tag '${tag_name}' successfully pushed to ${remote_name}.${COLOR_RESET}"
}

# Helper: Tag a single module
apply_tag() {
    local mod="$1"
    local raw_ver="$2"
    local commit="$3"
    local msg="$4"
    local force="$5"
    local push="$6"
    local remote="$7"
    local is_dry_run="$8"

    local ver
    ver="$(normalize_version "$raw_ver")"
    if [ -z "$ver" ]; then
        echo -e "${COLOR_RED}Error: Version format '${raw_ver}' is invalid. Use semver format like v1.0.0 or 1.0.0${COLOR_RESET}" >&2
        return 1
    fi

    local tag_name="${mod}/${ver}"

    if [ "$is_dry_run" = true ]; then
        echo -e "${COLOR_CYAN}[DRY-RUN] git tag -a ${tag_name} -m \"${msg}\" ${commit}${COLOR_RESET}"
        if [ "$push" = true ]; then
            echo -e "${COLOR_CYAN}[DRY-RUN] git push ${remote} ${tag_name}${COLOR_RESET}"
        fi
        return 0
    fi

    # Check if tag already exists
    if git rev-parse -q --verify "refs/tags/${tag_name}" >/dev/null 2>&1; then
        if [ "$force" = false ]; then
            echo -e "${COLOR_YELLOW}Tag '${tag_name}' already exists.${COLOR_RESET}"
            read -p "Do you want to overwrite this tag? (y/N): " confirm
            if [[ ! "$confirm" =~ ^[yY]$ ]]; then
                echo -e "${COLOR_YELLOW}Cancelled.${COLOR_RESET}"
                return 0
            fi
        fi
        local force_flag="-f"
        echo -e "${COLOR_YELLOW}Updating tag '${tag_name}' to commit ${commit}...${COLOR_RESET}"
    else
        local force_flag=""
        echo -e "${COLOR_BLUE}Creating new tag '${tag_name}' at commit ${commit}...${COLOR_RESET}"
    fi

    git tag $force_flag -a "$tag_name" -m "$msg" "$commit"
    echo -e "${COLOR_GREEN}✓ Tag '${tag_name}' successfully created.${COLOR_RESET}"

    if [ "$push" = true ]; then
        push_tag "$tag_name" "$remote" false
    fi
}

# ==============================================================================
# CLI Argument Parsing
# ==============================================================================
PUSH=false
REMOTE="origin"
COMMIT="HEAD"
MESSAGE=""
FORCE=false
DRY_RUN=false
TARGET_ALL=false
ALL_VERSION=""
CLI_MOD=""
CLI_VER=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)
            show_help
            exit 0
            ;;
        -l|--list)
            list_modules
            exit 0
            ;;
        -p|--push)
            PUSH=true
            shift
            ;;
        -r|--remote)
            REMOTE="$2"
            shift 2
            ;;
        -c|--commit)
            COMMIT="$2"
            shift 2
            ;;
        -m|--message)
            MESSAGE="$2"
            shift 2
            ;;
        -f|--force)
            FORCE=true
            shift
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -a|--all)
            TARGET_ALL=true
            ALL_VERSION="$2"
            shift 2
            ;;
        *)
            if [ -z "$CLI_MOD" ]; then
                CLI_MOD="$1"
            elif [ -z "$CLI_VER" ]; then
                CLI_VER="$1"
            fi
            shift
            ;;
    esac
done

# ==============================================================================
# Mode: Batch All Modules
# ==============================================================================
if [ "$TARGET_ALL" = true ]; then
    if [ -z "$ALL_VERSION" ]; then
        echo -e "${COLOR_RED}Error: Option --all requires a version parameter. Example: --all v1.0.0${COLOR_RESET}" >&2
        exit 1
    fi

    ver="$(normalize_version "$ALL_VERSION")"
    if [ -z "$ver" ]; then
        echo -e "${COLOR_RED}Error: Version format '${ALL_VERSION}' is invalid.${COLOR_RESET}" >&2
        exit 1
    fi

    if [ -z "$MESSAGE" ]; then
        MESSAGE="Release ${ver} for Go SDK modules"
    fi

    echo -e "${COLOR_BOLD}Batch Releasing All Go SDK Modules to ${COLOR_GREEN}${ver}${COLOR_RESET}"
    echo "Target Commit : $COMMIT"
    echo "Tag Message   : $MESSAGE"
    echo "Remote Push   : $PUSH (Remote: $REMOTE)"
    echo "Dry Run       : $DRY_RUN"
    echo "------------------------------------------------------------------------"

    if [ "$DRY_RUN" = false ]; then
        read -p "Proceed with creating tags for all modules? (y/N): " confirm
        if [[ ! "$confirm" =~ ^[yY]$ ]]; then
            echo -e "${COLOR_YELLOW}Operation cancelled.${COLOR_RESET}"
            exit 0
        fi
    fi

    count=0
    while IFS= read -r mod; do
        [ -z "$mod" ] && continue
        apply_tag "$mod" "$ver" "$COMMIT" "$MESSAGE" "$FORCE" "$PUSH" "$REMOTE" "$DRY_RUN"
        count=$((count + 1))
    done < <(get_all_modules)

    echo "------------------------------------------------------------------------"
    echo -e "${COLOR_GREEN}Complete! Total ${count} modules processed.${COLOR_RESET}"
    exit 0
fi

# ==============================================================================
# Mode: CLI Single Module
# ==============================================================================
if [ -n "$CLI_MOD" ]; then
    # Support format "module/v1.0.0"
    if [[ "$CLI_MOD" =~ /v[0-9] ]] && [ -z "$CLI_VER" ]; then
        CLI_VER="${CLI_MOD##*/}"
        CLI_MOD="${CLI_MOD%/*}"
    fi

    if [ -z "$CLI_VER" ]; then
        echo -e "${COLOR_RED}Error: Target version is not specified.${COLOR_RESET}" >&2
        echo "Usage: ./release.sh $CLI_MOD <version>"
        exit 1
    fi

    # Validate module directory
    if [ ! -f "${CLI_MOD}/go.mod" ]; then
        echo -e "${COLOR_RED}Error: Directory '${CLI_MOD}' does not contain a valid go.mod file.${COLOR_RESET}" >&2
        exit 1
    fi

    if [ -z "$MESSAGE" ]; then
        MESSAGE="Release ${CLI_MOD}/${CLI_VER}"
    fi

    apply_tag "$CLI_MOD" "$CLI_VER" "$COMMIT" "$MESSAGE" "$FORCE" "$PUSH" "$REMOTE" "$DRY_RUN"
    exit 0
fi

# ==============================================================================
# Mode: Interactive
# ==============================================================================
clear 2>/dev/null || true
echo -e "${COLOR_BOLD}========================================================================${COLOR_RESET}"
echo -e "${COLOR_BOLD}               Go SDK Monorepo Tagging & Release Tool                   ${COLOR_RESET}"
echo -e "${COLOR_BOLD}========================================================================${COLOR_RESET}"
echo ""

list_modules
echo ""

modules_array=()
while IFS= read -r mod; do
    [ -n "$mod" ] && modules_array+=("$mod")
done < <(get_all_modules)

total_mods=${#modules_array[@]}

echo -e "Select option:"
echo -e "  [1-${total_mods}] : Select individual module number"
echo -e "  [A]     : Tag ALL modules simultaneously (${total_mods} modules)"
echo -e "  [Q]     : Quit"
echo ""

read -p "Enter choice: " choice

if [[ "$choice" =~ ^[qQ]$ ]]; then
    echo "Exiting."
    exit 0
fi

if [[ "$choice" =~ ^[aA]$ ]]; then
    read -p "Enter version for ALL modules (e.g. v1.0.0): " input_ver
    ver="$(normalize_version "$input_ver")"
    if [ -z "$ver" ]; then
        echo -e "${COLOR_RED}Invalid version format!${COLOR_RESET}"
        exit 1
    fi
    read -p "Tag message (Press Enter for default: 'Release ${ver}'): " input_msg
    [ -z "$input_msg" ] && input_msg="Release ${ver}"

    read -p "Push to remote origin after tagging? (y/N): " do_push
    PUSH_FLAG=false
    [[ "$do_push" =~ ^[yY]$ ]] && PUSH_FLAG=true

    for mod in "${modules_array[@]}"; do
        apply_tag "$mod" "$ver" "HEAD" "$input_msg" false "$PUSH_FLAG" "origin" false
    done
    echo -e "${COLOR_GREEN}✓ Successfully tagged all ${total_mods} modules to ${ver}!${COLOR_RESET}"
    exit 0
fi

if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] || [ "$choice" -gt "$total_mods" ]; then
    echo -e "${COLOR_RED}Invalid selection!${COLOR_RESET}"
    exit 1
fi

selected_mod="${modules_array[$((choice - 1))]}"
latest_tag="$(get_latest_tag "$selected_mod")"
suggested_ver="$(suggest_next_version "$latest_tag")"

echo ""
echo -e "Selected Module : ${COLOR_CYAN}${selected_mod}${COLOR_RESET}"
echo -e "Current Tag     : ${COLOR_YELLOW}${latest_tag:-no tag yet}${COLOR_RESET}"
echo -e "Suggested Next  : ${COLOR_GREEN}${suggested_ver}${COLOR_RESET}"
echo ""

read -p "Enter tag version [default: ${suggested_ver}]: " input_ver
[ -z "$input_ver" ] && input_ver="$suggested_ver"

ver="$(normalize_version "$input_ver")"
if [ -z "$ver" ]; then
    echo -e "${COLOR_RED}Invalid version format!${COLOR_RESET}"
    exit 1
fi

read -p "Tag message [default: 'Release ${selected_mod}/${ver}']: " input_msg
[ -z "$input_msg" ] && input_msg="Release ${selected_mod}/${ver}"

read -p "Push to remote origin after completion? (y/N): " do_push
PUSH_FLAG=false
[[ "$do_push" =~ ^[yY]$ ]] && PUSH_FLAG=true

apply_tag "$selected_mod" "$ver" "HEAD" "$input_msg" false "$PUSH_FLAG" "origin" false
