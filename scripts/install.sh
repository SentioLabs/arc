#!/usr/bin/env bash
# Bootstrap Arc. Existing installations update with `arc self update`.
# Legacy updaters still call this URL with --force --tag=<release>.
set -euo pipefail

REPO="sentiolabs/arc"
FORCE="${FORCE:-false}"
TAG="${TAG:-}"

usage() {
    cat <<'HELP'
Arc Installer
Usage: install.sh [--force|-f] [--tag TAG|--tag=TAG]

Installs the latest stable release, or a specific --tag.
Existing installations: use arc self update (native, checksum-verified updates).
--force reinstalls; --force and --tag remain supported for older Arc updaters.
FORCE and TAG may also be set as environment variables.
HELP
}
fail() { echo "Error: $*" >&2; exit 1; }
while [[ $# -gt 0 ]]; do
    case "$1" in
        --force|-f) FORCE=true; shift ;;
        --tag) [[ $# -ge 2 && -n "$2" ]] || fail '--tag requires a value'; TAG="$2"; shift 2 ;;
        --tag=*) TAG="${1#*=}"; [[ -n "$TAG" ]] || fail '--tag requires a value'; shift ;;
        --help|-h) usage; exit 0 ;;
        *) fail "Unknown option: $1" ;;
    esac
done
[[ -z "$TAG" || "$TAG" =~ ^v?[0-9A-Za-z][0-9A-Za-z.+-]*$ ]] || fail 'Invalid release tag'

installed="$(command -v arc || true)"
if [[ -n "$installed" && "$FORCE" != true && -z "$TAG" ]]; then
    echo 'Arc is already installed. Run: arc self update'
    exit 0
fi
case "$(uname -s)" in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) fail 'Supported operating systems: Linux and macOS' ;;
esac
case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) fail 'Supported architectures: amd64 and arm64' ;;
esac

download() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$1" -o "$2"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$2" "$1"
    else
        fail 'Install curl or wget first'
    fi
}

# Install beside the existing executable for legacy updates; resolve symlinks.
if [[ -n "$installed" ]]; then
    while [[ -L "$installed" ]]; do
        link="$(readlink "$installed")"
        if [[ "$link" = /* ]]; then installed="$link"; else installed="$(dirname "$installed")/$link"; fi
    done
    install_dir="$(cd "$(dirname "$installed")" && pwd -P)"
    target="$install_dir/$(basename "$installed")"
elif [[ -w /usr/local/bin ]]; then
    install_dir=/usr/local/bin
    target="$install_dir/arc"
else
    install_dir="$HOME/.local/bin"
    mkdir -p "$install_dir"
    target="$install_dir/arc"
fi
[[ -w "$install_dir" ]] || fail "Not writable: $install_dir. Use your package manager or install to a user-writable directory."

tmp_dir="$(mktemp -d)"
restart=false
replacement=''
server_running() {
    [[ -n "$installed" ]] && "$installed" server status --json 2>/dev/null | grep -Eq '"running"[[:space:]]*:[[:space:]]*true'
}
cleanup() {
    result=$?
    trap - EXIT
    [[ -z "$replacement" ]] || rm -f "$replacement"
    rm -rf "$tmp_dir"
    if [[ "$restart" == true ]] && ! server_running; then
        if ! "$target" server start; then
            echo "Error: server restart failed; run '$target server start' to retry" >&2
            result=1
        fi
    fi
    exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if [[ -z "$TAG" ]]; then
    download "https://api.github.com/repos/$REPO/releases/latest" "$tmp_dir/release.json"
    TAG="$(sed -nE 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' "$tmp_dir/release.json" | head -1)"
fi
[[ -n "$TAG" && "$TAG" =~ ^v?[0-9A-Za-z][0-9A-Za-z.+-]*$ ]] || fail 'Could not resolve release tag'
archive="arc_${TAG#v}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$TAG"
echo "Downloading Arc $TAG..."
download "$base/$archive" "$tmp_dir/$archive"
download "$base/checksums.txt" "$tmp_dir/checksums.txt"
expected="$(awk -v asset="$archive" '$2 == asset || $2 == "*" asset {print $1}' "$tmp_dir/checksums.txt")"
[[ "$expected" =~ ^[[:xdigit:]]{64}$ ]] || fail "Missing or ambiguous checksum for $archive"
if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$tmp_dir/$archive")"
elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$tmp_dir/$archive")"
else
    fail 'Install sha256sum or shasum first'
fi
[[ "${actual%% *}" == "$expected" ]] || fail 'Archive checksum mismatch'
# Extract only the binary, never arbitrary archive paths or links.
tar -xOzf "$tmp_dir/$archive" arc > "$tmp_dir/arc"
[[ -s "$tmp_dir/arc" ]] || fail 'Archive contains no arc binary'
chmod 755 "$tmp_dir/arc"

# Stage on the target filesystem before stopping the daemon.
replacement="$(mktemp "$install_dir/.arc-install.XXXXXX")"
cp "$tmp_dir/arc" "$replacement"
chmod 755 "$replacement"
if [[ "$os" == darwin ]] && command -v codesign >/dev/null 2>&1; then
    codesign --force --sign - "$replacement"
fi
if server_running; then
    restart=true
    "$installed" server stop
    if server_running; then fail 'Server is still running after stop; installation aborted'; fi
fi
mv -f "$replacement" "$target"
replacement=''
echo "Installed Arc $TAG to $target"
if [[ ":$PATH:" != *":$install_dir:"* ]]; then
    echo "Add $install_dir to your PATH."
fi
echo 'Get started: arc quickstart'
