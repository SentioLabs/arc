#!/usr/bin/env bash
#
# Arc installation script
# Usage: curl -fsSL https://raw.githubusercontent.com/sentiolabs/arc/main/scripts/install.sh | bash
#
# Options:
#   --force    Force reinstall even if already up-to-date
#

set -e

# ============ Configuration ============

REPO="sentiolabs/arc"
BINARY_NAME="arc"
FORCE="${FORCE:-false}"

# ============ Output Formatting ============

# Detect terminal capabilities
if [[ -t 1 ]] && command -v tput &> /dev/null && [[ $(tput colors 2>/dev/null || echo 0) -ge 8 ]]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    BOLD='\033[1m'
    DIM='\033[2m'
    NC='\033[0m'
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    BOLD=''
    DIM=''
    NC=''
fi

log_info() {
    echo -e "${BLUE}→${NC} $1"
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}!${NC} $1"
}

log_error() {
    echo -e "${RED}✗${NC} $1" >&2
}

log_step() {
    echo -e "${DIM}  $1${NC}"
}

# ============ Version Detection ============

# Get installed arc version
get_installed_version() {
    if command -v arc &> /dev/null; then
        # arc --version outputs: "arc vX.Y.Z (commit) built date with go"
        # Extract just the version
        local version_output
        version_output=$(arc --version 2>/dev/null || echo "")
        echo "$version_output" | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -1
    fi
}

# Normalize version strings for comparison (remove 'v' prefix)
normalize_version() {
    echo "$1" | sed 's/^v//'
}

# Compare versions: returns 0 if equal, 1 if first > second, 2 if first < second
compare_versions() {
    local v1 v2
    v1=$(normalize_version "$1")
    v2=$(normalize_version "$2")

    if [[ "$v1" == "$v2" ]]; then
        return 0
    fi

    # Use sort -V for version comparison if available
    if printf '%s\n%s' "$v1" "$v2" | sort -V -C 2>/dev/null; then
        return 2  # v1 < v2
    else
        return 1  # v1 > v2
    fi
}

# ============ Platform Detection ============

detect_platform() {
    local os arch

    case "$(uname -s)" in
        Darwin)
            os="darwin"
            ;;
        Linux)
            os="linux"
            ;;
        *)
            log_error "Unsupported operating system: $(uname -s)"
            exit 1
            ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)
            arch="amd64"
            ;;
        aarch64|arm64)
            arch="arm64"
            ;;
        *)
            log_error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac

    echo "${os}_${arch}"
}

# ============ Server Management ============

SERVER_WAS_RUNNING="false"

check_server_running() {
    if ! command -v arc &> /dev/null; then
        return 1
    fi
    arc server status &>/dev/null
}

stop_existing_server() {
    if ! command -v arc &> /dev/null; then
        return 0
    fi

    if check_server_running; then
        SERVER_WAS_RUNNING="true"
        log_step "Stopping arc server..."
        if arc server stop 2>/dev/null; then
            log_step "Server stopped"
        fi
    fi
    return 0
}

restart_server_if_needed() {
    if [[ "$SERVER_WAS_RUNNING" == "true" ]]; then
        log_step "Restarting arc server..."
        if arc server start 2>/dev/null; then
            log_step "Server restarted"
        fi
    fi
}

# ============ macOS Code Signing ============

resign_for_macos() {
    local binary_path=$1

    if [[ "$(uname -s)" != "Darwin" ]]; then
        return 0
    fi

    if ! command -v codesign &> /dev/null; then
        return 0
    fi

    log_step "Re-signing binary for macOS..."
    codesign --remove-signature "$binary_path" 2>/dev/null || true
    if codesign --force --sign - "$binary_path" 2>/dev/null; then
        log_step "Binary signed"
    fi
}

# ============ Release Asset Check ============

release_has_asset() {
    local release_json=$1
    local asset_name=$2

    if echo "$release_json" | grep -Fq "\"name\": \"$asset_name\""; then
        return 0
    fi
    return 1
}

# ============ Installation ============

install_from_release() {
    local platform=$1
    local installed_version=$2
    local tmp_dir

    tmp_dir=$(mktemp -d)

    # Fetch latest release
    log_info "Checking latest release..."
    local latest_url="https://api.github.com/repos/${REPO}/releases/latest"
    local release_json

    if command -v curl &> /dev/null; then
        release_json=$(curl -fsSL "$latest_url" 2>/dev/null)
    elif command -v wget &> /dev/null; then
        release_json=$(wget -qO- "$latest_url" 2>/dev/null)
    else
        log_error "Neither curl nor wget found"
        rm -rf "$tmp_dir"
        return 1
    fi

    local latest_version
    latest_version=$(echo "$release_json" | grep '"tag_name"' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')

    if [[ -z "$latest_version" ]]; then
        log_error "Failed to fetch latest version"
        rm -rf "$tmp_dir"
        return 1
    fi

    # Version comparison
    if [[ -n "$installed_version" ]] && [[ "$FORCE" != "true" ]]; then
        if compare_versions "$installed_version" "$latest_version"; then
            log_success "arc ${installed_version} is already up to date"
            rm -rf "$tmp_dir"
            return 2  # Special return code: already up to date
        fi
        log_info "Updating arc ${installed_version} → ${latest_version}"
    else
        log_info "Installing arc ${latest_version}"
    fi

    # Download
    local archive_name="${BINARY_NAME}_${latest_version#v}_${platform}.tar.gz"
    local download_url="https://github.com/${REPO}/releases/download/${latest_version}/${archive_name}"

    if ! release_has_asset "$release_json" "$archive_name"; then
        log_error "No prebuilt binary for ${platform}"
        rm -rf "$tmp_dir"
        return 1
    fi

    log_info "Downloading ${archive_name}..."
    cd "$tmp_dir"

    if command -v curl &> /dev/null; then
        if ! curl -fsSL --progress-bar -o "$archive_name" "$download_url"; then
            log_error "Download failed"
            cd - > /dev/null || cd "$HOME"
            rm -rf "$tmp_dir"
            return 1
        fi
    elif command -v wget &> /dev/null; then
        if ! wget -q --show-progress -O "$archive_name" "$download_url" 2>/dev/null; then
            # Fallback without progress for older wget
            if ! wget -q -O "$archive_name" "$download_url"; then
                log_error "Download failed"
                cd - > /dev/null || cd "$HOME"
                rm -rf "$tmp_dir"
                return 1
            fi
        fi
    fi

    # Extract
    log_step "Extracting..."
    if ! tar -xzf "$archive_name"; then
        log_error "Failed to extract archive"
        cd - > /dev/null || cd "$HOME"
        rm -rf "$tmp_dir"
        return 1
    fi

    # Determine install location
    local install_dir
    if [[ -w /usr/local/bin ]]; then
        install_dir="/usr/local/bin"
    else
        install_dir="$HOME/.local/bin"
        mkdir -p "$install_dir"
    fi

    # Stop server before replacing binary
    stop_existing_server

    # Install
    log_step "Installing to ${install_dir}..."
    if [[ -w "$install_dir" ]]; then
        mv "$BINARY_NAME" "$install_dir/"
    else
        sudo mv "$BINARY_NAME" "$install_dir/"
    fi

    resign_for_macos "$install_dir/$BINARY_NAME"

    cd - > /dev/null || cd "$HOME"
    rm -rf "$tmp_dir"

    log_success "Installed arc ${latest_version} to ${install_dir}/${BINARY_NAME}"

    # Restart server if it was running before update
    restart_server_if_needed

    # PATH warning
    if [[ ":$PATH:" != *":$install_dir:"* ]]; then
        echo ""
        log_warning "${install_dir} is not in your PATH"
        echo -e "  Add to your shell profile: ${BOLD}export PATH=\"\$PATH:$install_dir\"${NC}"
    fi

    return 0
}

# ============ Verification ============

verify_installation() {
    if ! command -v arc &> /dev/null; then
        return 1
    fi

    echo ""
    echo -e "${BOLD}arc${NC} is ready!"
    echo ""
    arc --version 2>/dev/null || echo "arc (development build)"
    echo ""
    echo "Get started:"
    echo "  arc quickstart      Quick start guide"
    echo "  arc init            Initialize workspace"
    echo "  arc server start    Start the server"
    echo ""
}

# ============ Help ============

show_help() {
    echo "Arc Installer"
    echo ""
    echo "Usage: $0 [options]"
    echo ""
    echo "Options:"
    echo "  --force    Force reinstall even if already up-to-date"
    echo "  --help     Show this help message"
    echo ""
    echo "Examples:"
    echo "  curl -fsSL https://raw.githubusercontent.com/sentiolabs/arc/main/scripts/install.sh | bash"
    echo "  curl -fsSL ... | bash -s -- --force"
    echo ""
}

# ============ Main ============

main() {
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --force|-f)
                FORCE="true"
                shift
                ;;
            --help|-h)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done

    echo ""
    echo -e "${BOLD}Arc Installer${NC}"
    echo ""

    # Detect platform
    local platform
    platform=$(detect_platform)
    log_step "Platform: ${platform}"

    # Check installed version
    local installed_version
    installed_version=$(get_installed_version)
    if [[ -n "$installed_version" ]]; then
        log_step "Installed: ${installed_version}"
    fi

    # Install
    local result
    if install_from_release "$platform" "$installed_version"; then
        verify_installation
        exit 0
    else
        result=$?
        if [[ $result -eq 2 ]]; then
            # Already up to date
            exit 0
        fi
    fi

    # Installation failed
    echo ""
    log_error "Installation failed"
    echo ""
    echo "Manual installation options:"
    echo ""
    echo "  1. Download from https://github.com/${REPO}/releases/latest"
    echo "     Extract and move 'arc' to your PATH"
    echo ""
    echo "  2. Build from source (requires Go 1.23+):"
    echo "     git clone https://github.com/${REPO}.git"
    echo "     cd arc && make build"
    echo ""
    exit 1
}

main "$@"
