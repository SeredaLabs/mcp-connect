#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
GO=${GO:-go}
platform=${GOOS:-$("$GO" env GOOS)}
arch=${GOARCH:-$("$GO" env GOARCH)}
version=$(sed -n 's/^const Version = "\([0-9.]*\)"/\1/p' internal/core/store.go)
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Invalid application version' >&2; exit 1; }
out="dist/${platform}-${arch}"
stage="dist/installers/${platform}-${arch}"
mkdir -p "$stage"
case "$platform" in
  darwin)
    mkdir -p "$stage/root/Applications"
    ditto "$out/MCP Connect.app" "$stage/root/Applications/MCP Connect.app"
    pkgbuild --analyze --root "$stage/root" "$stage/components.plist"
    /usr/libexec/PlistBuddy -c 'Set :0:BundleIsRelocatable false' "$stage/components.plist"
    /usr/libexec/PlistBuddy -c 'Set :0:BundleIsVersionChecked true' "$stage/components.plist"
    pkgbuild --root "$stage/root" --component-plist "$stage/components.plist" \
      --identifier "eu.onecmcp.connect.pkg" --version "$version" \
      --install-location / --ownership recommended "$stage/application.pkg"
    cat > "$stage/Distribution.xml" <<XML
<?xml version="1.0" encoding="utf-8"?>
<installer-gui-script minSpecVersion="2">
  <title>MCP Connect</title>
  <options customize="never" require-scripts="false"/>
  <domains enable_localSystem="true" enable_currentUserHome="false" enable_anywhere="false"/>
  <volume-check><allowed-os-versions><os-version min="13.0"/></allowed-os-versions></volume-check>
  <choices-outline><line choice="application"/></choices-outline>
  <choice id="application" visible="false"><pkg-ref id="eu.onecmcp.connect.pkg"/></choice>
  <pkg-ref id="eu.onecmcp.connect.pkg" version="$version" onConclusion="none">application.pkg</pkg-ref>
</installer-gui-script>
XML
    productbuild --distribution "$stage/Distribution.xml" --package-path "$stage" \
      "dist/mcp-connect-darwin-${arch}-installer.pkg"
    ;;
  windows)
    compiler=${ISCC:-iscc}
    MSYS_NO_PATHCONV=1 MSYS2_ARG_CONV_EXCL='*' "$compiler" "/DAppVersion=$version" "/DTargetArch=$arch" \
      "/DSourceDir=$(cygpath -w "$PWD/$out")" "/DOutputPath=$(cygpath -w "$PWD/dist")" \
      "$(cygpath -w "$PWD/packaging/windows/setup.iss")"
    ;;
  linux)
    mkdir -p "$stage/root/DEBIAN" "$stage/root/opt/mcp-connect" \
      "$stage/root/usr/bin" "$stage/root/usr/share/applications" \
      "$stage/root/usr/share/icons/hicolor/512x512/apps" "$stage/root/usr/share/doc/mcp-connect"
    install -m755 "$out/mcp-connect" "$stage/root/opt/mcp-connect/mcp-connect"
    ln -sfn /opt/mcp-connect/mcp-connect "$stage/root/usr/bin/mcp-connect"
    install -m644 assets/icon.png "$stage/root/usr/share/icons/hicolor/512x512/apps/mcp-connect.png"
    sed 's|^Exec=mcp-connect$|Exec=/opt/mcp-connect/mcp-connect|' assets/mcp-connect.desktop \
      > "$stage/root/usr/share/applications/mcp-connect.desktop"
    for file in README.uk.md LICENSE.txt THIRD_PARTY_NOTICES.txt; do
      install -m644 "$out/$file" "$stage/root/usr/share/doc/mcp-connect/$file"
    done
    cat > "$stage/root/DEBIAN/control" <<CONTROL
Package: mcp-connect
Version: $version
Section: utils
Priority: optional
Architecture: $arch
Maintainer: SeredaLabs <322079711+SeredaLabs@users.noreply.github.com>
Homepage: https://github.com/SeredaLabs/mcp-connect
Depends: libc6 (>= 2.39), libgl1, libegl1, libx11-6, libxcursor1, libxi6, libxinerama1, libxrandr2, libxxf86vm1, libxkbcommon0, libwayland-client0, libwayland-cursor0, dbus-user-session
Recommends: gnome-keyring | kwallet6
Description: Connect AI clients to HTTPS MCP servers
 Compact desktop utility with operating-system credential storage.
 AI clients and a Secret Service provider are installed separately.
CONTROL
    dpkg-deb --root-owner-group --build "$stage/root" "dist/mcp-connect-linux-${arch}.deb"
    ;;
  *) echo "Unsupported installer platform: $platform" >&2; exit 1 ;;
esac
