# Релиз

Как выпустить версию и опубликовать сборки в GitHub Releases.

## Версионирование

Версия дублируется в нескольких файлах, поэтому перед релизом поднимите её одной
командой (она обновит `build/config.yml`, `build/linux/nfpm/nfpm.yaml`,
`build/darwin/Info.plist`, `build/darwin/Info.dev.plist`, `build/windows/info.json`,
`frontend/package.json`):

```bash
scripts/set-version.sh 0.2.0
git commit -am "chore: версия 0.2.0"
```

## Проверки (CI)

Workflow `.github/workflows/ci.yml` запускается на каждый пуш в `main` и на
pull request и проверяет качество кода до релиза:

- **Backend (Go)**: `gofmt`, `go build`, `go vet`, `go test ./internal/...`;
- **Frontend (React/TS)**: `npm ci`, `npm run typecheck`, `npm run build`.

Тяжёлые сборки пакетов здесь не выполняются — их делает `release.yml` по тегу.

## Автоматический релиз (GitHub Actions)

Workflow `.github/workflows/release.yml` запускается на пуш тега `v*` (а также
вручную — **Actions → Release → Run workflow**). Сборка идёт матрицей на
«родных» раннерах, затем артефакты всех платформ скачиваются и публикуются одним
job-ом:

- **Linux** (ubuntu-24.04): `.deb`, `.rpm`, `.AppImage`, `.pkg.tar.zst`;
- **Windows** (windows-latest): установщик NSIS `proxy-pool-manager-amd64-installer.exe`
  и портативный `proxy-pool-manager-windows-amd64.zip` (запуск без установки);
- **macOS** (macos-latest): `.dmg` (universal: arm64 + amd64).

При ручном запуске (`workflow_dispatch`) релиз **не** публикуется — собранные
файлы остаются в артефактах запуска. Это удобно для проверки сборки.

Выпуск:

```bash
git tag v0.2.0
git push origin v0.2.0
```

После этого на вкладке **Releases** появится релиз `v0.2.0` с приложенными
файлами и авто-сгенерированным описанием (`generate_release_notes`).

Перезапуск, если сборка упала: **Actions → Release → Re-run failed jobs**.
Чтобы пересобрать под тем же тегом заново — удалить и заново запушить тег:

```bash
git push origin :refs/tags/v0.2.0
git push origin v0.2.0
```

## Ручной релиз (без CI)

Соберите пакеты под каждую платформу (на её же системе) и опубликуйте:

```bash
# Linux
wails3 package

# Windows (на Windows; либо на Linux с установленным nsis)
wails3 task windows:package

# macOS (на macOS)
wails3 task darwin:package:universal
wails3 task darwin:create:dmg
```

Публикация через GitHub CLI (`gh`):

```bash
gh release create v0.2.0 \
  --title "Proxy Pool Manager v0.2.0" \
  --generate-notes \
  bin/*.deb bin/*.rpm bin/*.AppImage bin/*.pkg.tar.zst \
  bin/*-installer.exe bin/*-windows-amd64.zip \
  bin/*.dmg
```

## Об AppImage

AppImage собирается задачей `linux:create:appimage` через `linuxdeploy` и намеренно
**не бандлит библиотеки** (`--exclude-library='*'`): GTK4/WebKitGTK/libsoup берутся
из системы. Итог — ~5 МБ вместо ~80 МБ у self-contained варианта. Такой образ
работает на дистрибутивах со свежим стеком GTK4 + WebKitGTK 6.0 (Debian 13+,
Ubuntu 24.04+, Fedora 39+, Arch/Manjaro, openSUSE Tumbleweed). Бандлинг GTK4, к
тому же, давал ошибки загрузки системных модулей из-за рассинхрона версий.

Если нужен переносимый образ, собираемый со всеми библиотеками, — замените тело
задачи `linux:create:appimage` на `wails3 generate appimage` (тогда образ вырастет
до ~80 МБ, но не будет требовать системного стека).

## Подпись (важно для распространения)

- **Windows**: без подписи Authenticode SmartScreen показывает предупреждение.
- **macOS**: без подписи Developer ID и нотаризации Gatekeeper блокирует запуск.

Подпись в CI включается автоматически при наличии секретов — см.
[SIGNING.md](SIGNING.md). Пока релизы не подписаны, предупредите об этом в описании релиза.
