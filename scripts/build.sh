#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
GO=${GO:-go}
platform=${GOOS:-$("$GO" env GOOS)}
arch=${GOARCH:-$("$GO" env GOARCH)}
# Match the documented minimum OS instead of inheriting the build host version.
if [ "$platform" = darwin ]; then
  export MACOSX_DEPLOYMENT_TARGET=13.0
  export CGO_CFLAGS="${CGO_CFLAGS:--O2 -g} -mmacosx-version-min=13.0"
  export CGO_CXXFLAGS="${CGO_CXXFLAGS:--O2 -g} -mmacosx-version-min=13.0"
  export CGO_LDFLAGS="${CGO_LDFLAGS:--O2 -g} -mmacosx-version-min=13.0"
fi
# GLFW's bundled XInput declarations use the legacy WINBOOL alias.
# Current Windows ARM64 MinGW/clang headers expose BOOL instead.
if [ "$platform" = windows ] && [ "$arch" = arm64 ]; then
  export CGO_CFLAGS="${CGO_CFLAGS:-} -DWINBOOL=BOOL"
fi
out="dist/${platform}-${arch}"
mkdir -p "$out"
if [ "$platform" = windows ]; then
  # Embed the Explorer icon without changing the MCP console/stdio behavior.
  resource="cmd/mcp-connect/icon_windows_${arch}.syso"
  trap 'rm -f "$resource"' EXIT
  GOOS= GOARCH= "$GO" run github.com/tc-hib/go-winres@v0.3.3 simply \
    --arch "$arch" --out cmd/mcp-connect/icon --manifest none --icon assets/icon.png
fi
if [ "$platform" = darwin ]; then
  bundle="$out/MCP Connect.app"
  mkdir -p "$bundle/Contents/MacOS" "$bundle/Contents/Resources"
  "$GO" build -trimpath -buildvcs=false -ldflags='-s -w' -o "$bundle/Contents/MacOS/mcp-connect" ./cmd/mcp-connect
  min_version=$(otool -l "$bundle/Contents/MacOS/mcp-connect" | awk '$1 == "minos" {print $2; exit}')
  case "$min_version" in
    13.0|13.0.0) ;;
    *) echo "Unexpected macOS deployment target: $min_version (expected 13.0)" >&2; exit 1 ;;
  esac
  cp assets/icon.icns "$bundle/Contents/Resources/icon.icns"
  cat > "$bundle/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleName</key><string>MCP Connect</string>
<key>CFBundleDisplayName</key><string>MCP Connect</string>
<key>CFBundleIdentifier</key><string>eu.onecmcp.connect</string>
<key>CFBundleVersion</key><string>27</string>
<key>CFBundleShortVersionString</key><string>0.4.9</string>
<key>CFBundleExecutable</key><string>mcp-connect</string>
<key>CFBundleIconFile</key><string>icon.icns</string>
<key>LSMinimumSystemVersion</key><string>13.0</string>
<key>NSHighResolutionCapable</key><true/>
<key>NSPrincipalClass</key><string>NSApplication</string>
</dict></plist>
PLIST
  cp LICENSE "$bundle/Contents/Resources/LICENSE.txt"
  cp THIRD_PARTY_NOTICES.txt "$bundle/Contents/Resources/THIRD_PARTY_NOTICES.txt"
  cp docs/INSTALL.uk.md "$bundle/Contents/Resources/README.uk.md"
  codesign --force --deep --sign - "$bundle"
  ditto -c -k --sequesterRsrc --keepParent "$bundle" "dist/mcp-connect-${platform}-${arch}-portable.zip"
else
  suffix=""; if [ "$platform" = windows ]; then suffix=.exe; fi
  # Keep the Windows console subsystem: MCP stdio must remain usable.
  "$GO" build -trimpath -buildvcs=false -ldflags='-s -w' -o "$out/mcp-connect$suffix" ./cmd/mcp-connect
  if [ "$platform" = windows ]; then
    cp docs/INSTALL.uk.md "$out/README.uk.md"
    cp LICENSE "$out/LICENSE.txt"
    cp THIRD_PARTY_NOTICES.txt "$out/THIRD_PARTY_NOTICES.txt"
    if command -v 7z >/dev/null 2>&1; then
      (cd "$out" && 7z a -tzip "../mcp-connect-${platform}-${arch}-portable.zip" "mcp-connect.exe" "README.uk.md" "LICENSE.txt" "THIRD_PARTY_NOTICES.txt")
    elif command -v zip >/dev/null 2>&1; then
      (cd "$out" && zip -q "../mcp-connect-${platform}-${arch}-portable.zip" "mcp-connect.exe" "README.uk.md" "LICENSE.txt" "THIRD_PARTY_NOTICES.txt")
    else
      echo "ZIP packager missing: install 7-Zip or zip" >&2
      exit 1
    fi
  else
    cp docs/INSTALL.uk.md "$out/README.uk.md"
    cp LICENSE "$out/LICENSE.txt"
    cp THIRD_PARTY_NOTICES.txt "$out/THIRD_PARTY_NOTICES.txt"
    cp assets/icon.png "$out/icon.png"
    cp assets/mcp-connect.desktop "$out/mcp-connect.desktop"
    tar -czf "dist/mcp-connect-${platform}-${arch}-portable.tar.gz" -C "$out" mcp-connect README.uk.md LICENSE.txt THIRD_PARTY_NOTICES.txt icon.png mcp-connect.desktop
  fi
fi
