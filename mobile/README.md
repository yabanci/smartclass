# smartclass — Flutter mobile app

Native Android + iOS client for the Smart Classroom platform. Connects to the same Go backend as the React web frontend.

## Stack

| | |
|---|---|
| Flutter | 3.41.7 (stable) |
| State | Riverpod 2.x |
| Navigation | go_router 14.x |
| HTTP | Dio 5.x |
| Secure storage | flutter_secure_storage 9.x — Keychain (iOS) / EncryptedSharedPreferences (Android) |
| WebSocket | web_socket_channel 3.x |
| Charts | fl_chart 0.70.x |
| i18n | flutter_localizations — EN / RU / KK |

## Flavors

| Flavor | Entry point | API |
|---|---|---|
| `dev` | `lib/main_dev.dart` | `http://localhost:8080/api/v1` |
| `prod` | `lib/main_prod.dart` | `https://api.smartclass.kz/api/v1` |

`AppConfig` in `lib/config/app_config.dart` holds all flavor-specific values; never hardcode URLs.

## Quickstart

```bash
flutter pub get

# Run dev flavor (connects to local backend)
flutter run -t lib/main_dev.dart

# Run prod flavor
flutter run -t lib/main_prod.dart
```

Backend must be running (`make up` from repo root) for the dev flavor to work.

## Tests

```bash
flutter test                        # unit tests (test/)
flutter test integration_test/      # integration tests — requires running emulator/device
```

Tests use `mocktail` for mocking. Never mock the system under test.

## Project structure

```
lib/
  config/           AppConfig + flavors
  core/
    api/            Dio client + endpoint classes (auth, classroom, device, …)
    connection/     ConnectionResolver — auto-switches dev/local/remote
    i18n/           Generated ARB localizations
    push/           FCMService stub (ready for Firebase activation)
    router/         go_router config
    storage/        TokenStorage — flutter_secure_storage wrapper
    utils/          friendlyError()
    ws/             WsClient — WebSocket hub with auto-reconnect
  features/         Page-level widgets (one folder per screen)
  shared/
    models/         Data classes (fromJson/toJson)
    providers/      Riverpod providers (auth, classroom, device, schedule, …)
    widgets/        Reusable UI (AppButton, AppCard, OfflineBanner, …)
  main.dart         Default entry (dev)
  main_dev.dart     Dev flavor entry
  main_prod.dart    Prod flavor entry
  app.dart          MaterialApp + router wiring
```

## Adding Firebase push notifications

FCM is stubbed in `lib/core/push/fcm_service.dart`. To activate:

1. Create a Firebase project and add Android + iOS apps.
2. Place `google-services.json` → `android/app/` and `GoogleService-Info.plist` → `ios/Runner/`.
3. Add to `pubspec.yaml`: `firebase_core`, `firebase_messaging`.
4. Uncomment the body in `FCMService.init()` and follow the inline comments.

## CI

GitHub Actions (`.github/workflows/ci.yml`, `mobile` job):

- `flutter analyze --fatal-infos`
- `flutter test --reporter expanded`
- `flutter build apk --release` → artifact `smartclass-release.apk` (retained 14 days)

iOS CI build requires macOS runner + Apple code-signing secrets — not configured yet.
