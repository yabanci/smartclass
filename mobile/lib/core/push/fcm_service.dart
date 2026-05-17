// Firebase Cloud Messaging integration — stub ready for activation.
//
// Backend wiring is already complete:
//   POST   /api/v1/me/device-tokens     {token, platform}
//   DELETE /api/v1/me/device-tokens/:t
//
// When Firebase is configured server-side (FIREBASE_PROJECT_ID +
// FIREBASE_SERVICE_ACCOUNT_JSON envs), notification.Service fans out
// every created notification to each user's registered tokens.
//
// To activate on mobile:
// 1. Create a Firebase project at https://console.firebase.google.com
// 2. Add Android app → download google-services.json → android/app/
// 3. Add iOS app    → download GoogleService-Info.plist → ios/Runner/
// 4. Add to pubspec.yaml:
//      firebase_core: ^3.x.x
//      firebase_messaging: ^15.x.x
// 5. Android: apply plugin 'com.google.gms.google-services' in android/app/build.gradle
//    iOS: run `pod install` from ios/
// 6. Uncomment the firebase imports and the body in init()
// 7. Call FCMService.instance.init(apiClient) from mainWithConfig()

import 'dart:io' show Platform;

import 'package:flutter/foundation.dart';

// import 'package:firebase_core/firebase_core.dart';
// import 'package:firebase_messaging/firebase_messaging.dart';

import '../api/client.dart';
import '../api/endpoints/user_endpoints.dart';

class FCMService {
  FCMService._();
  static final FCMService instance = FCMService._();

  bool _initialized = false;
  String? _currentToken;

  String _platformName() {
    if (kIsWeb) return 'web';
    if (Platform.isAndroid) return 'android';
    if (Platform.isIOS) return 'ios';
    return 'web';
  }

  Future<void> init(ApiClient client) async {
    if (_initialized) return;
    _initialized = true;

    // Uncomment when Firebase project is ready:
    // await Firebase.initializeApp();
    // final messaging = FirebaseMessaging.instance;
    // final settings = await messaging.requestPermission();
    // if (settings.authorizationStatus == AuthorizationStatus.denied) return;
    // final token = await messaging.getToken();
    // if (token != null) {
    //   await _saveToken(client, token);
    // }
    // messaging.onTokenRefresh.listen((token) => _saveToken(client, token));
    // FirebaseMessaging.onMessage.listen(_handleForeground);
  }

  // Used by the commented Firebase wiring in init(). Kept ready so activation
  // requires only uncommenting — no further refactor.
  // ignore: unused_element
  Future<void> _saveToken(ApiClient client, String token) async {
    if (_currentToken == token) return;
    try {
      await UserEndpoints(client).registerDeviceToken(
        token: token,
        platform: _platformName(),
      );
      _currentToken = token;
    } catch (_) {
      // Network errors are non-fatal — the next refresh or app start retries.
    }
  }

  Future<void> unregister(ApiClient client) async {
    final token = _currentToken;
    if (token == null) return;
    try {
      await UserEndpoints(client).unregisterDeviceToken(token);
    } catch (_) {}
    _currentToken = null;
  }

  // void _handleForeground(RemoteMessage message) {
  //   // Show a local notification using flutter_local_notifications
  // }
}
