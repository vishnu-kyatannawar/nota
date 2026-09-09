# Maintainer: Vishnu Kyatannawar <vishnukyatannawar@gmail.com>
#
# This tracks the development branch on purpose: it is the -git package, and
# pkgver() derives its version from CMakeLists.txt plus the commit, so it
# always sorts above the last release. For a release build instead, drop the
# -git suffix, point source= at the latest release tarball and delete pkgver().

pkgname=nota-git
pkgver=5.0.0.r73.g46b53c3
pkgrel=1
pkgdesc="Daily workplans and notes, stored as plain markdown"
arch=('x86_64' 'aarch64')
url="https://github.com/vishnu-kyatannawar/nota"
license=('MIT')

depends=('qt6-base' 'qt6-declarative'
         'kirigami' 'kirigami-addons'
         'ki18n' 'kcoreaddons' 'kconfig' 'kcrash' 'kitemmodels'
         'syntax-highlighting' 'kcolorscheme' 'kiconthemes'
         'qqc2-desktop-style' 'breeze-icons' 'hicolor-icon-theme')
makedepends=('cmake' 'extra-cmake-modules' 'ninja' 'git')
checkdepends=()

provides=("nota=$pkgver")
conflicts=('nota')

source=("$pkgname::git+$url.git")
sha256sums=('SKIP')

pkgver() {
  cd "$srcdir/$pkgname"
  # The version CMake declares, plus how far past it we are and the commit.
  local version
  version=$(sed -n 's/^project(nota VERSION \([0-9.]*\).*/\1/p' CMakeLists.txt)
  printf "%s.r%s.g%s" "${version:-0}" "$(git rev-list --count HEAD)" "$(git rev-parse --short HEAD)"
}

build() {
  # CMAKE_BUILD_TYPE=None is the Arch convention: it leaves the flags from
  # makepkg.conf in place rather than overriding them.
  cmake -S "$pkgname" -B build -G Ninja \
    -DCMAKE_INSTALL_PREFIX=/usr \
    -DCMAKE_BUILD_TYPE=None \
    -DBUILD_TESTING=ON \
    -Wno-dev
  cmake --build build
}

check() {
  ctest --test-dir build --output-on-failure
}

package() {
  DESTDIR="$pkgdir" cmake --install build
  install -Dm644 "$srcdir/$pkgname/LICENSE" \
    "$pkgdir/usr/share/licenses/${pkgname}/LICENSE"
}
