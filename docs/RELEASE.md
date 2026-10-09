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

## Автоматический релиз (GitHub Actions)

Workflow `.github/workflows/release.yml` запускается на пуш тега `v*` и собирает
пакеты под все платформы, прикрепляя их к GitHub Release:

- **Linux** (ubuntu-24.04): `.deb`, `.rpm`, `.AppImage`, `.pkg.tar.zst`;
- **Windows** (windows-latest): установщик NSIS `proxy-pool-manager-installer.exe`;
- **macOS** (macos-latest): `.dmg` (universal: arm64 + amd64).

Выпуск:

```bash
git tag v0.2.0
git push origin v0.2.0
```

После этого на вкладке **Releases** появится релиз `v0.2.0` с приложенными
файлами. Описание можно поправить вручную, а через месяц GitHub автоматически
включит архивы исходников.

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
  build/windows/nsis/proxy-pool-manager-installer.exe \
  bin/*.dmg
```

## Подпись (важно для распространения)

- **Windows**: без подписи Authenticode SmartScreen показывает предупреждение.
  Подпись: `wails3 task windows:sign:installer` (нужен сертификат).
- **macOS**: без подписи Developer ID и нотаризации Gatekeeper блокирует запуск
  («не удалось проверить разработчика»). Подпись/нотаризация — только на macOS:
  `wails3 task darwin:sign:notarize` (требует `wails3 setup` и Apple Developer ID).
  В CI это делается добавлением секретов и шагов подписи — см. `BUILD.md`.

Пока релизы не подписаны, предупредите об этом в описании релиза.
