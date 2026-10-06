#!/usr/bin/env sh
# Устанавливает .desktop-файл и иконку в ~/.local для ТЕКУЩЕГО пользователя.
#
# Нужно, потому что на Linux (GTK4) Wails не может задать иконку окна из кода —
# GNOME/KDE берут иконку из .desktop-файла, id которого совпадает с app_id окна
# (мы задаём ProgramName=proxy-pool-manager в main.go).
#
# После установки перезапусти приложение.
set -eu

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BIN="$ROOT/bin/proxy-pool-manager"
ICON_DIR="$HOME/.local/share/icons/hicolor/256x256/apps"
APP_DIR="$HOME/.local/share/applications"

if [ ! -x "$BIN" ]; then
  echo "Не найден бинарник: $BIN" >&2
  echo "Сначала соберите: wails3 build" >&2
  exit 1
fi

mkdir -p "$ICON_DIR" "$APP_DIR"
cp "$ROOT/build/appicon.png" "$ICON_DIR/proxy-pool-manager.png"

cat > "$APP_DIR/proxy-pool-manager.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Proxy Pool Manager
Comment=Сбор, проверка и управление пулом прокси
Exec="$BIN"
Icon=proxy-pool-manager
Terminal=false
Categories=Network;Development;
StartupWMClass=proxy-pool-manager
EOF

update-desktop-database "$APP_DIR" 2>/dev/null || true
gtk-update-icon-cache -f -t "$HOME/.local/share/icons/hicolor" 2>/dev/null || true

echo "Установлено:"
echo "  $APP_DIR/proxy-pool-manager.desktop"
echo "  $ICON_DIR/proxy-pool-manager.png"
echo "Перезапустите приложение, чтобы появилась иконка."
