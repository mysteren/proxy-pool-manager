# Сборка и установка

Документ описывает, как собрать приложение и упаковать его для установки в систему.
Отдельно для **Linux** и **macOS**.

## Общие требования

- **Go 1.25+**
- **Node.js 18+**
- **Wails v3** (`wails3 version`)
- **Linux**: компилятор C (CGO обязателен для WebKitGTK), `gcc`/`clang` + заголовки GTK4/WebKitGTK 6.0
- **macOS**: Xcode Command Line Tools

`nfpm` отдельно ставить **не нужно** — упаковщик встроен в `wails3 tool package`.
Для AppImage при первой сборке скачиваются `linuxdeploy` и GTK-плагин (~20 МБ).

Версия приложения задаётся в двух местах и должна совпадать:

- `build/config.yml` → `info.version`
- `build/linux/nfpm/nfpm.yaml` → `version`

## Linux

### Просто бинарник (для локального запуска)

```bash
wails3 build
./bin/proxy-pool-manager
```

Чтобы в меню и на панели задач появилась **иконка**, на Linux (GTK4) её нужно
взять из `.desktop`-файла — из кода окна задать нельзя. Для локального запуска:

```bash
wails3 task linux:install:desktop
```

Скрипт кладёт `.desktop` и иконку в `~/.local` текущего пользователя.
После этого перезапустите приложение (при необходимости — сессию GNOME/KDE).

### Пакеты для установки в систему

Все форматы сразу:

```bash
wails3 package
```

Отдельно:

```bash
wails3 task linux:create:deb       # bin/proxy-pool-manager.deb
wails3 task linux:create:rpm       # bin/proxy-pool-manager.rpm
wails3 task linux:create:appimage  # bin/proxy-pool-manager-x86_64.AppImage
wails3 task linux:create:aur       # bin/proxy-pool-manager.pkg.tar.zst
```

Что попадает в пакет (`build/linux/nfpm/nfpm.yaml`):

| Файл | Путь в системе |
| --- | --- |
| бинарник | `/usr/local/bin/proxy-pool-manager` |
| иконки | `/usr/share/icons/hicolor/{128x128,256x256,512x512}/apps/proxy-pool-manager.png` |
| `.desktop` | `/usr/share/applications/proxy-pool-manager.desktop` |

Зависимости: `libgtk-4-1`, `libwebkitgtk-6.0-4` (для rpm — `gtk4`, `webkitgtk6.0`;
для Arch — `gtk4`, `webkitgtk-6.0`).

### Установка

```bash
# Debian / Ubuntu / Mint
sudo apt install ./bin/proxy-pool-manager.deb

# Fedora / RHEL / openSUSE
sudo dnf install ./bin/proxy-pool-manager.rpm

# Arch Linux (из собранного архива)
sudo pacman -U ./bin/proxy-pool-manager.pkg.tar.zst

# AppImage — без установки
chmod +x ./bin/proxy-pool-manager-x86_64.AppImage
./bin/proxy-pool-manager-x86_64.AppImage
```

`.desktop` устанавливается вместе с пакетом, `postinstall` обновляет
desktop- и mime-базы, поэтому иконка и запись в меню появляются сразу.

### Локальная установка `.desktop` (без пакета)

```bash
wails3 task linux:install:desktop
```

Использует `build/linux/install-desktop.sh`. Иконка ставится в
`~/.local/share/icons/hicolor/256x256/apps/`.

## macOS

Сборка `.app`/`.dmg` и подпись должны выполняться **на macOS**: нотаризация
Apple требует ключей и утилит, которых нет на Linux.

### На macOS

```bash
# .app-бандл
wails3 task darwin:package

# универсальный бинарник (arm64 + amd64) в .app
wails3 task darwin:package:universal

# .dmg для распространения
wails3 task darwin:package:dmg

# подпись Developer ID
wails3 task darwin:sign -- --identity "Developer ID Application: ..."

# подпись + нотаризация (нужна настройка: wails3 setup)
wails3 task darwin:sign:notarize
```

Для распространения вне App Store нужен Apple Developer ID: настройте профиль
один раз через `wails3 setup`, после чего `sign`/`sign:notarize` берут данные из
`~/.config/wails/defaults.yaml`.

### Кросс-сборка с Linux (без подписи)

Требуется Docker:

```bash
wails3 task setup:docker            # образ wails-cross (~800 МБ, однократно)
wails3 task darwin:package:universal
```

Получится `bin/proxy-pool-manager.app` без code signing. Подписать и нотаризовать
его нужно на macOS перед распространением (`codesign:skip` при кросс-сборке).

## Метаданные приложения

Правится в:

- `build/config.yml` — `companyName`, `productName`, `productIdentifier`,
  `description`, `version` (используется генераторами ассетов).
- `build/linux/nfpm/nfpm.yaml` — описание пакета, maintainer, homepage, содержимое.
- `build/darwin/Info.plist` и `Info.dev.plist` — `CFBundleName`,
  `CFBundleIdentifier`, версии, копирайт.
- `build/linux/Taskfile.yml` (задача `generate:dotdesktop`) — `Name`, `Comment`,
  `Categories`, `Keywords` генерируемого `.desktop`.
