#!/usr/bin/env sh
# Устанавливает версию приложения во всех местах, где она дублируется.
# Использование: scripts/set-version.sh 0.2.0
set -eu

V="${1:-}"
if [ -z "$V" ]; then
  echo "Использование: scripts/set-version.sh X.Y.Z" >&2
  exit 1
fi

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

# config.yml (info.version) и nfpm.yaml (версия пакета).
V="$V" perl -pi -e 's/^(\s*version:\s*)"[^"]*"/$1"$ENV{V}"/' \
  build/config.yml build/linux/nfpm/nfpm.yaml

# Info.plist (CFBundleVersion / CFBundleShortVersionString).
V="$V" perl -0777 -pi -e \
  's{(<key>(?:CFBundleVersion|CFBundleShortVersionString)</key>\s*<string>)[^<]*(</string>)}{$1$ENV{V}$2}g' \
  build/darwin/Info.plist build/darwin/Info.dev.plist

# Windows info.json (file_version / ProductVersion).
V="$V" perl -pi -e 's/("(?:file_version|ProductVersion)":\s*)"[^"]*"/$1"$ENV{V}"/g' \
  build/windows/info.json

# frontend/package.json.
V="$V" perl -pi -e 's/^(\s*"version":\s*)"[^"]*"/$1"$ENV{V}"/' frontend/package.json

echo "Версия установлена: $V"
