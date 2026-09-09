#!/bin/sh
# Installs Nota.
#
#   curl -fsSL https://raw.githubusercontent.com/vishnu-kyatannawar/nota/main/install.sh | sh
#
# No sudo: everything lands under ~/.local. Nota is a Qt 6 and KDE Frameworks 6
# application, and a KF6 binary is not portable between distributions — the
# libraries it links are the ones the distribution ships — so rather than
# downloading a binary that would fail to start on half the machines that ran
# it, this builds from source against the frameworks you already have. That is
# also why it checks for every one of them up front and prints your
# distribution's install line instead of leaving cmake to fail halfway.
set -eu

REPO="vishnu-kyatannawar/nota"
PREFIX="${NOTA_PREFIX:-${HOME}/.local}"
REF="${NOTA_REF:-}"

die() { echo "error: $*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || missing_tools="${missing_tools:-} $1"; }

# --- platform -----------------------------------------------------------------
os=$(uname -s | tr '[:upper:]' '[:lower:]')
[ "$os" = "linux" ] || die "Nota is a KDE Plasma application and runs on Linux only (found: $os)"

# --- what the distribution calls the packages ---------------------------------
if command -v pacman >/dev/null 2>&1; then
  install_line="sudo pacman -S --needed cmake extra-cmake-modules ninja git qt6-base qt6-declarative kirigami kirigami-addons ki18n kcoreaddons kconfig kcrash kitemmodels syntax-highlighting kcolorscheme kiconthemes qqc2-desktop-style breeze-icons"
elif command -v dnf >/dev/null 2>&1; then
  install_line="sudo dnf install cmake extra-cmake-modules ninja-build git qt6-qtbase-devel qt6-qtdeclarative-devel kf6-kirigami-devel kf6-kirigami-addons-devel kf6-ki18n-devel kf6-kcoreaddons-devel kf6-kconfig-devel kf6-kcrash-devel kf6-kitemmodels-devel kf6-syntax-highlighting-devel kf6-kcolorscheme-devel kf6-kiconthemes-devel qqc2-desktop-style"
elif command -v apt >/dev/null 2>&1; then
  install_line="sudo apt install cmake extra-cmake-modules ninja-build git qt6-base-dev qt6-declarative-dev libkf6kirigami-dev libkf6i18n-dev libkf6coreaddons-dev libkf6config-dev libkf6crash-dev libkf6itemmodels-dev libkf6syntaxhighlighting-dev libkf6colorscheme-dev libkf6iconthemes-dev qml6-module-org-kde-kirigami qqc2-desktop-style"
elif command -v zypper >/dev/null 2>&1; then
  install_line="sudo zypper install cmake extra-cmake-modules ninja git qt6-base-devel qt6-declarative-devel kf6-kirigami-devel kf6-ki18n-devel kf6-kcoreaddons-devel kf6-kconfig-devel kf6-kcrash-devel kf6-kitemmodels-devel kf6-syntax-highlighting-devel kf6-kcolorscheme-devel kf6-kiconthemes-devel qqc2-desktop-style"
else
  install_line="(see https://github.com/${REPO}#build for your distribution)"
fi

# --- the tools ----------------------------------------------------------------
missing_tools=""
need git
need cmake
need curl

# --- the frameworks -----------------------------------------------------------
# KDE Frameworks ship CMake config files rather than pkg-config files, so this
# looks for the config a find_package() call would.
cmake_pkg() {
  for root in /usr/lib /usr/lib64 /usr/lib/x86_64-linux-gnu /usr/local/lib /usr/local/lib64; do
    [ -f "${root}/cmake/$1/$1Config.cmake" ] && return 0
  done
  return 1
}

missing_pkgs=""
[ -d /usr/share/ECM/cmake ] || [ -d /usr/share/ECM/modules ] || missing_pkgs="${missing_pkgs} extra-cmake-modules"
for pkg in Qt6Core Qt6Quick Qt6QuickControls2 Qt6Widgets \
           KF6CoreAddons KF6I18n KF6Config KF6Crash KF6ItemModels \
           KF6ColorScheme KF6IconThemes KF6SyntaxHighlighting \
           KF6Kirigami KF6KirigamiAddons; do
  cmake_pkg "$pkg" || missing_pkgs="${missing_pkgs} ${pkg}"
done

if [ -n "${missing_tools# }" ] || [ -n "${missing_pkgs# }" ]; then
  echo "Nota needs Qt 6.9+ and KDE Frameworks 6.10+ to build." >&2
  [ -n "${missing_tools# }" ] && echo "  missing tools:      ${missing_tools# }" >&2
  [ -n "${missing_pkgs# }" ] && echo "  missing frameworks: ${missing_pkgs# }" >&2
  echo "" >&2
  echo "Install them with:" >&2
  echo "  ${install_line}" >&2
  echo "" >&2
  echo "then run this again." >&2
  die "missing build dependencies"
fi

# --- fetch --------------------------------------------------------------------
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

if [ -z "$REF" ]; then
  # The newest release tag, or the default branch when there are no tags yet.
  REF=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null |
        sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1) || REF=""
  [ -n "$REF" ] || REF="main"
fi

echo "Fetching Nota (${REF})..."
git clone --quiet --depth 1 --branch "$REF" "https://github.com/${REPO}.git" "${tmp}/nota" 2>/dev/null ||
  git clone --quiet --depth 1 "https://github.com/${REPO}.git" "${tmp}/nota" ||
  die "could not clone https://github.com/${REPO}.git"

# Releases up to v4.6.1 are the Go application, which has no CMake build. Rather
# than hard-coding a version to skip past, this checks what it actually got: no
# CMakeLists.txt means the tag predates the rewrite, so fall back to the
# development branch. Once a KDE release is tagged, the check simply passes.
if [ ! -f "${tmp}/nota/CMakeLists.txt" ]; then
  echo "note: release ${REF} predates the KDE rewrite — building the development branch instead"
  rm -rf "${tmp}/nota"
  git clone --quiet --depth 1 "https://github.com/${REPO}.git" "${tmp}/nota" ||
    die "could not clone https://github.com/${REPO}.git"
  [ -f "${tmp}/nota/CMakeLists.txt" ] || die "no CMake build found in ${REPO}"
fi

# --- build --------------------------------------------------------------------
generator=""
command -v ninja >/dev/null 2>&1 && generator="-G Ninja"

echo "Building..."
# shellcheck disable=SC2086
cmake -S "${tmp}/nota" -B "${tmp}/build" ${generator} \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_INSTALL_PREFIX="$PREFIX" \
  -DBUILD_TESTING=OFF >"${tmp}/cmake.log" 2>&1 ||
  { tail -20 "${tmp}/cmake.log" >&2; die "configuring failed"; }

cmake --build "${tmp}/build" >"${tmp}/build.log" 2>&1 ||
  { tail -30 "${tmp}/build.log" >&2; die "building failed"; }

# --- install ------------------------------------------------------------------
cmake --install "${tmp}/build" >/dev/null || die "installing to ${PREFIX} failed"

command -v update-desktop-database >/dev/null 2>&1 &&
  update-desktop-database "${PREFIX}/share/applications" 2>/dev/null || true
command -v gtk-update-icon-cache >/dev/null 2>&1 &&
  gtk-update-icon-cache -f -t "${PREFIX}/share/icons/hicolor" 2>/dev/null || true

version=$("${PREFIX}/bin/nota" --version 2>/dev/null | head -n1)
echo "Installed ${version:-Nota} to ${PREFIX}/bin/nota"

case ":${PATH}:" in
  *":${PREFIX}/bin:"*) ;;
  *) echo "note: ${PREFIX}/bin is not on your PATH — add it to your shell profile" ;;
esac

# Installing is not the same as running. A copy left in another prefix — an
# older install.sh run, or a distribution package — keeps winning if its
# directory comes first on PATH, and the symptom is an update that appears to
# have done nothing at all. Say so plainly rather than let it be a mystery.
running=$(command -v nota 2>/dev/null || true)
if [ -n "$running" ] && [ "$running" != "${PREFIX}/bin/nota" ]; then
  other=$("$running" --version 2>/dev/null | head -n1)
  echo ""
  echo "warning: typing 'nota' still runs ${running} (${other:-unknown version}),"
  echo "         not the ${version:-copy} just installed, because its directory comes"
  echo "         first on your PATH. Desktop launchers resolve it the same way."
  echo ""
  echo "         Remove the other copy, or put ${PREFIX}/bin ahead of it on PATH."
fi
