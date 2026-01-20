#!/usr/bin/env bash
#
# Arc installation script
# Usage: curl -fsSL https://raw.githubusercontent.com/sentiolabs/arc/main/scripts/install.sh | bash
#

set -e

REPO="sentiolabs/arc"
BINARY_NAME="arc"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}==>${NC} $1"
}

log_success() {
    echo -e "${GREEN}==>${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}==>${NC} $1"
}

log_error() {
    echo -e "${RED}Error:${NC} $1" >&2
}

release_has_asset() {
    local release_json=$1
    local asset_name=$2

    if echo "$release_json" | grep -Fq "\"name\": \"$asset_name\""; then
        return 0
    fi

    return 1
}

# Re-sign binary for macOS to avoid slow Gatekeeper checks
resign_for_macos() {
    local binary_path=$1

    # Only run on macOS
    if [[ "$(uname -s)" != "Darwin" ]]; then
        return 0
    fi

    # Check if codesign is available
    if ! command -v codesign &> /dev/null; then
        log_warning "codesign not found, skipping re-signing"
        return 0
    fi

    log_info "Re-signing binary for macOS..."
    codesign --remove-signature "$binary_path" 2>/dev/null || true
    if codesign --force --sign - "$binary_path"; then
        log_success "Binary re-signed for this machine"
    else
        log_warning "Failed to re-sign binary (non-fatal)"
    fi
}

# Detect OS and architecture
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

# Stop existing server before upgrade
stop_existing_server() {
    # Skip if arc isn't installed
    if ! command -v arc &> /dev/null; then
        return 0
    fi

    log_info "Stopping existing arc server before upgrade..."

    # Try graceful shutdown
    if arc server stop 2>/dev/null; then
        log_success "Stopped existing server"
    else
        log_warning "No server running or failed to stop (continuing anyway)"
    fi

    return 0
}

# Download and install from GitHub releases
install_from_release() {
    log_info "Installing arc from GitHub releases..."

    local platform=$1
    local tmp_dir
    tmp_dir=$(mktemp -d)

    # Get latest release version
    log_info "Fetching latest release..."
    local latest_url="https://api.github.com/repos/${REPO}/releases/latest"
    local version
    local release_json

    if command -v curl &> /dev/null; then
        release_json=$(curl -fsSL "$latest_url")
    elif command -v wget &> /dev/null; then
        release_json=$(wget -qO- "$latest_url")
    else
        log_error "Neither curl nor wget found. Please install one of them."
        return 1
    fi

    version=$(echo "$release_json" | grep '"tag_name"' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')

    if [ -z "$version" ]; then
        log_error "Failed to fetch latest version"
        return 1
    fi

    log_info "Latest version: $version"

    # Download URL (goreleaser format: arc_VERSION_OS_ARCH.tar.gz)
    local archive_name="${BINARY_NAME}_${version#v}_${platform}.tar.gz"
    local download_url="https://github.com/${REPO}/releases/download/${version}/${archive_name}"

    if ! release_has_asset "$release_json" "$archive_name"; then
        log_warning "No prebuilt archive available for platform ${platform}."
        rm -rf "$tmp_dir"
        return 1
    fi

    log_info "Downloading $archive_name..."

    cd "$tmp_dir"
    if command -v curl &> /dev/null; then
        if ! curl -fsSL -o "$archive_name" "$download_url"; then
            log_error "Download failed"
            cd - > /dev/null || cd "$HOME"
            rm -rf "$tmp_dir"
            return 1
        fi
    elif command -v wget &> /dev/null; then
        if ! wget -q -O "$archive_name" "$download_url"; then
            log_error "Download failed"
            cd - > /dev/null || cd "$HOME"
            rm -rf "$tmp_dir"
            return 1
        fi
    fi

    # Extract archive
    log_info "Extracting archive..."
    if ! tar -xzf "$archive_name"; then
        log_error "Failed to extract archive"
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

    # Install binary
    log_info "Installing to $install_dir..."
    if [[ -w "$install_dir" ]]; then
        mv "$BINARY_NAME" "$install_dir/"
    else
        sudo mv "$BINARY_NAME" "$install_dir/"
    fi

    # Re-sign for macOS to avoid Gatekeeper delays
    resign_for_macos "$install_dir/$BINARY_NAME"

    log_success "arc installed to $install_dir/$BINARY_NAME"

    # Check if install_dir is in PATH
    if [[ ":$PATH:" != *":$install_dir:"* ]]; then
        log_warning "$install_dir is not in your PATH"
        echo ""
        echo "Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        echo "  export PATH=\"\$PATH:$install_dir\""
        echo ""
    fi

    cd - > /dev/null || cd "$HOME"
    rm -rf "$tmp_dir"
    return 0
}

# Verify installation
verify_installation() {
    if command -v arc &> /dev/null; then
        log_success "arc is installed and ready!"
        echo ""
        arc --version 2>/dev/null || echo "arc (development build)"
        echo ""
        echo "Get started:"
        echo "  arc quickstart      # Quick start guide"
        echo "  arc init            # Initialize workspace"
        echo "  arc server start    # Start the server"
        echo ""
        return 0
    else
        log_error "arc was installed but is not in PATH"
        return 1
    fi
}

# Main installation flow
main() {
    echo ""
    echo "Arc Installer"
    echo ""

    log_info "Detecting platform..."
    local platform
    platform=$(detect_platform)
    log_info "Platform: $platform"

    # Stop any running server before replacing binary
    stop_existing_server

    # Try downloading from GitHub releases
    if install_from_release "$platform"; then
        verify_installation
        exit 0
    fi

    # Release download failed
    log_error "Installation failed"
    echo ""
    echo "Manual installation options:"
    echo ""
    echo "  1. Download from https://github.com/${REPO}/releases/latest"
    echo "     Extract and move 'arc' to your PATH"
    echo ""
    echo "  2. Install from source (requires Go 1.23+):"
    echo "     git clone https://github.com/${REPO}.git"
    echo "     cd arc && make build"
    echo ""
    echo "  3. Linux packages (deb/rpm/arch) available at:"
    echo "     https://github.com/${REPO}/releases/latest"
    echo ""
    exit 1
}

main "$@"
