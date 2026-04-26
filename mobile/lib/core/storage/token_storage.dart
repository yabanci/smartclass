import 'package:shared_preferences/shared_preferences.dart';

// shared_preferences used instead of flutter_secure_storage because
// flutter_secure_storage's WebCrypto backend is unreliable in Flutter Web
// (IndexedDB init race condition). Switch to platform-specific keychain
// storage in Phase 5 for production native builds.
class TokenStorage {
  static const _keyAccess = 'sc_access_token';
  static const _keyRefresh = 'sc_refresh_token';
  static const _keyAccessExpiry = 'sc_access_expiry';
  static const _keyRefreshExpiry = 'sc_refresh_expiry';

  Future<SharedPreferences> get _prefs => SharedPreferences.getInstance();

  Future<void> saveTokens({
    required String accessToken,
    required String refreshToken,
    required String accessExpiresAt,
    required String refreshExpiresAt,
  }) async {
    final prefs = await _prefs;
    await Future.wait([
      prefs.setString(_keyAccess, accessToken),
      prefs.setString(_keyRefresh, refreshToken),
      prefs.setString(_keyAccessExpiry, accessExpiresAt),
      prefs.setString(_keyRefreshExpiry, refreshExpiresAt),
    ]);
  }

  Future<String?> getAccessToken() async {
    final prefs = await _prefs;
    return prefs.getString(_keyAccess);
  }

  Future<String?> getRefreshToken() async {
    final prefs = await _prefs;
    return prefs.getString(_keyRefresh);
  }

  Future<bool> isAccessExpired() async {
    final prefs = await _prefs;
    final expiry = prefs.getString(_keyAccessExpiry);
    if (expiry == null) return true;
    try {
      final dt = DateTime.parse(expiry);
      return DateTime.now().isAfter(dt.subtract(const Duration(seconds: 30)));
    } catch (_) {
      return true;
    }
  }

  Future<bool> isRefreshExpired() async {
    final prefs = await _prefs;
    final expiry = prefs.getString(_keyRefreshExpiry);
    if (expiry == null) return true;
    try {
      final dt = DateTime.parse(expiry);
      return DateTime.now().isAfter(dt);
    } catch (_) {
      return true;
    }
  }

  Future<void> clear() async {
    final prefs = await _prefs;
    await Future.wait([
      prefs.remove(_keyAccess),
      prefs.remove(_keyRefresh),
      prefs.remove(_keyAccessExpiry),
      prefs.remove(_keyRefreshExpiry),
    ]);
  }
}
