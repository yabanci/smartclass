// Firebase Cloud Messaging integration — stub ready for activation.
//
// To activate:
// 1. Create a Firebase project at https://console.firebase.google.com
// 2. Add Android app → download google-services.json → android/app/
// 3. Add iOS app    → download GoogleService-Info.plist → ios/Runner/
// 4. Add to pubspec.yaml:
//      firebase_core: ^3.x.x
//      firebase_messaging: ^15.x.x
// 5. Android: apply plugin 'com.google.gms.google-services' in android/app/build.gradle
//    iOS: run `pod install` from ios/
// 6. Uncomment the firebase imports and body below
// 7. Call FCMService.instance.init(apiClient) from mainWithConfig()

// import 'package:firebase_core/firebase_core.dart';
// import 'package:firebase_messaging/firebase_messaging.dart';

// ignore: unused_import — ApiClient and UserEndpoints used in commented Firebase code below.
import '../api/client.dart';
// ignore: unused_import
import '../api/endpoints/user_endpoints.dart';

class FCMService {
  FCMService._();
  static final FCMService instance = FCMService._();

  bool _initialized = false;

  /// Call once from mainWithConfig() after Firebase is configured.
  /// [client] is unused until Firebase is wired in; keep the signature stable.
  // ignore: avoid_unused_constructor_parameters
  Future<void> init(ApiClient client) async {
    if (_initialized) return;
    _initialized = true;

    // Uncomment when Firebase project is ready:
    // await Firebase.initializeApp();
    // final messaging = FirebaseMessaging.instance;
    // await messaging.requestPermission();
    // final token = await messaging.getToken();
    // if (token != null) {
    //   await UserEndpoints(client).saveFcmToken(token);
    // }
    // messaging.onTokenRefresh.listen((token) {
    //   UserEndpoints(client).saveFcmToken(token);
    // });
    // FirebaseMessaging.onMessage.listen(_handleForeground);
  }

  // void _handleForeground(RemoteMessage message) {
  //   // Show a local notification using flutter_local_notifications
  // }
}
