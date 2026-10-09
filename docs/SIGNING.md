# Подпись сборок

Релизный workflow подписывает сборки **автоматически, если заданы секреты**
репозитория. Без секретов всё собирается и публикуется неподписанным (не падает).

Секреты: **Settings → Secrets and variables → Actions → New repository secret**.

## Windows (Authenticode)

Нужен сертификат code signing в формате `.pfx`.

| Секрет | Что это |
| --- | --- |
| `WIN_CERT_PFX_BASE64` | сертификат `.pfx`, закодированный в base64 |
| `WIN_CERT_PASSWORD` | пароль от `.pfx` |

Получить base64 (Linux/macOS):

```bash
base64 -w0 cert.pfx > cert.pfx.b64     # macOS: base64 -i cert.pfx -o cert.pfx.b64
```

Без подписи Windows SmartScreen показывает предупреждение «Неизвестный издатель».

## macOS (Developer ID + нотаризация)

Нужна подписка Apple Developer Program (≈$99/год): сертификат
**Developer ID Application** и учётные данные для нотаризации.

| Секрет | Что это |
| --- | --- |
| `MACOS_CERT_P12_BASE64` | сертификат Developer ID `.p12` (с приватным ключом) в base64 |
| `MACOS_CERT_PASSWORD` | пароль от `.p12` |
| `MACOS_SIGN_IDENTITY` | имя идентичности, напр. `Developer ID Application: Имя (TEAMID)` |
| `APPLE_ID` | Apple ID (email) |
| `APPLE_TEAM_ID` | Team ID (10 символов) |
| `APPLE_APP_PASSWORD` | app-specific password для Apple ID |

Как сделать `.p12` (в Keychain Access на macOS): экспортировать сертификат
«Developer ID Application» вместе с приватным ключом → сохранить как `.p12` с
паролем, затем в base64:

```bash
base64 -w0 cert.p12 > cert.p12.b64    # macOS: base64 -i cert.p12 -o cert.p12.b64
```

App-specific password создаётся на <https://appleid.apple.com> → Вход и
безопасность → Пароли приложений.

Без подписи и нотаризации Gatekeeper блокирует запуск («не удалось проверить
разработчика»), и пользователю нужно обходить это вручную.

## Как это работает в CI

- Подпись Windows: после сборки установщика, если задан `WIN_CERT_PFX_BASE64`.
- Подпись macOS: сертификат импортируется в временную связку ключей; `.app`
  подписывается, затем `.dmg`; при заданных ключах Apple выполняется нотаризация
  и stapling `.dmg`.

## Если что-то не так

- Секрет не задан → шаг просто пропускается (`if:`), сборка выходит без подписи.
- Ошибка подписи → job падает; смотрите лог шага «Подпись…»/«Нотаризация…».
- Имена секретов менять нельзя без правки `.github/workflows/release.yml`.
