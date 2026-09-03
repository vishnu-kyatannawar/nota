#!/bin/sh
# Installs Nota from the latest GitHub release.
#
#   curl -fsSL https://raw.githubusercontent.com/vishnu-kyatannawar/nota/main/install.sh | sh
#
# No sudo: the binary lands in ~/.local/bin. The download is checked against the
# release's published SHA-256 before anything is executed or installed.
set -eu

REPO="vishnu-kyatannawar/nota"
BIN_DIR="${NOTA_BIN_DIR:-${HOME}/.local/bin}"
APP_DIR="${HOME}/.local/share/applications"

die() { echo "error: $*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "$1 is required but not installed"; }

need curl
need tar

# --- platform -----------------------------------------------------------------
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux) ;;
  darwin) die "macOS builds are not published yet" ;;
  *) die "unsupported operating system: $os (on Windows use install.ps1)" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) die "arm64 builds are not published yet" ;;
  *) die "unsupported architecture: $arch" ;;
esac

# --- runtime dependency -------------------------------------------------------
# Checked up front so a missing library fails here with a usable message, rather
# than later as a window that will not open.
if ! ldconfig -p 2>/dev/null | grep -q 'libwebkitgtk-6\.0\|libwebkit2gtk-4\.1'; then
  echo "Nota needs the WebKitGTK runtime. Install it with:" >&2
  if   command -v apt    >/dev/null 2>&1; then echo "  sudo apt install libgtk-4-1 libwebkitgtk-6.0-4" >&2
  elif command -v dnf    >/dev/null 2>&1; then echo "  sudo dnf install gtk4 webkitgtk6.0" >&2
  elif command -v pacman >/dev/null 2>&1; then echo "  sudo pacman -S gtk4 webkitgtk-6.0" >&2
  elif command -v zypper >/dev/null 2>&1; then echo "  sudo zypper install gtk4 webkit2gtk3-soup2" >&2
  else echo "  (see the README for your distribution)" >&2
  fi
  die "missing WebKitGTK"
fi

# --- resolve the latest release ----------------------------------------------
tag=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
      sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
[ -n "$tag" ] || die "could not determine the latest release of ${REPO}"

version=${tag#v}
asset="nota_${version}_${os}_${arch}.tar.gz"
base="https://github.com/${REPO}/releases/download/${tag}"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

echo "Downloading Nota ${tag}..."
curl -fsSL "${base}/${asset}"      -o "${tmp}/${asset}"      || die "download failed: ${base}/${asset}"
curl -fsSL "${base}/checksums.txt" -o "${tmp}/checksums.txt" || die "could not fetch checksums"

# --- verify before trusting ---------------------------------------------------
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$tmp" && grep " ${asset}\$" checksums.txt | sha256sum -c -) >/dev/null ||
    die "checksum mismatch for ${asset} — refusing to install"
elif command -v shasum >/dev/null 2>&1; then
  (cd "$tmp" && grep " ${asset}\$" checksums.txt | shasum -a 256 -c -) >/dev/null ||
    die "checksum mismatch for ${asset} — refusing to install"
else
  die "neither sha256sum nor shasum is available; cannot verify the download"
fi

# --- install ------------------------------------------------------------------
tar -xzf "${tmp}/${asset}" -C "$tmp"
mkdir -p "$BIN_DIR"
install -m 0755 "${tmp}/nota" "${BIN_DIR}/nota"

ICON_DIR="${HOME}/.local/share/icons/hicolor/256x256/apps"
if [ -f "${tmp}/nota.png" ]; then
  mkdir -p "$ICON_DIR"
  install -m 0644 "${tmp}/nota.png" "${ICON_DIR}/nota.png"
fi

mkdir -p "$APP_DIR"
cat > "${APP_DIR}/nota.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Name=Nota
Comment=Daily workplans and notes, stored as plain markdown
Exec=${BIN_DIR}/nota
Icon=nota
Terminal=false
Categories=Office;Utility;
StartupWMClass=nota
DESKTOP
command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$APP_DIR" 2>/dev/null || true

echo "Installed Nota ${tag} to ${BIN_DIR}/nota"
case ":${PATH}:" in
  *":${BIN_DIR}:"*) ;;
  *) echo "note: ${BIN_DIR} is not on your PATH — add it to your shell profile" ;;
esac
