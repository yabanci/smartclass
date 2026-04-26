import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:shared_preferences/shared_preferences.dart';

// Native: flutter_secure_storage uses Keychain (iOS) / EncryptedSharedPreferences (Android).
// Web: falls back to shared_preferences — flutter_secure_storage WebCrypto backend has an
// IndexedDB init race condition that makes it unreliable in Flutter Web (as of v9).
class TokenStorage {
  static const _keyAccess = 'sc_access_token';
  static const _keyRefresh = 'sc_refresh_token';
  static const _keyAccessExpiry = 'sc_access_expiry';
  static const _keyRefreshExpiry = 'sc_refresh_expiry';

  static const _secure = FlutterSecureStorage(
    aOptions: AndroidOptions(encryptedSharedPreferences: true),
    iOptions: IOSOptions(accessibility: KeychainAccessibility.first_unlock),
  );

  Future<SharedPreferences> get _prefs => SharedPreferences.getInstance();

  Future<void> saveTokens({
    required String accessToken,
    required String refreshToken,
    required String accessExpiresAt,
    required String refreshExpiresAt,
  }) async {
    if (kIsWeb) {
      final prefs = await _prefs;
      await Future.wait([
        prefs.setString(_keyAccess, accessToken),
        prefs.setString(_keyRefresh, refreshToken),
        prefs.setString(_keyAccessExpiry, accessExpiresAt),
        prefs.setString(_keyRefreshExpiry, refreshExpiresAt),
      ]);
    } else {
      await Future.wait([
        _secure.write(key: _keyAccess, value: accessToken),
        _secure.write(key: _keyRefresh, value: refreshToken),
        _secure.write(key: _keyAccessExpiry, value: accessExpiresAt),
        _secure.write(key: _keyRefreshExpiry, value: refreshExpiresAt),
      ]);
    }
  }

  Future<String?> getAccessToken() async {
    if (kIsWeb) {
      return (await _prefs).getString(_keyAccess);
    }
    return _secure.read(key: _keyAccess);
  }

  Future<String?> getRefreshToken() async {
    if (kIsWeb) {
      return (await _prefs).getString(_keyRefresh);
    }
    return _secure.read(key: _keyRefresh);
  }

  Future<bool> isAccessExpired() async {
    final expiry = kIsWeb
        ? (await _prefs).getString(_keyAccessExpiry)
        : await _secure.read(key: _keyAccessExpiry);
    if (expiry == null) return true;
    try {
      final dt = DateTime.parse(expiry);
      return DateTime.now().isAfter(dt.subtract(const Duration(seconds: 30)));
    } catch (_) {
      return true;
    }
  }

  Future<bool> isRefreshExpired() async {
    final expiry = kIsWeb
        ? (await _prefs).getString(_keyRefreshExpiry)
        : await _secure.read(key: _keyRefreshExpiry);
    if (expiry == null) return true;
    try {
      final dt = DateTime.parse(expiry);
      return DateTime.now().isAfter(dt);
    } catch (_) {
      return true;
    }
  }

  Future<void> clear() async {
    if (kIsWeb) {
      final prefs = await _prefs;
      await Future.wait([
        prefs.remove(_keyAccess),
        prefs.remove(_keyRefresh),
        prefs.remove(_keyAccessExpiry),
        prefs.remove(_keyRefreshExpiry),
      ]);
    } else {
      await Future.wait([
        _secure.delete(key: _keyAccess),
        _secure.delete(key: _keyRefresh),
        _secure.delete(key: _keyAccessExpiry),
        _secure.delete(key: _keyRefreshExpiry),
      ]);
    }
  }
}
