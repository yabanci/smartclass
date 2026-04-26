import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class TokenStorage {
  static const _keyAccess = 'sc_access_token';
  static const _keyRefresh = 'sc_refresh_token';
  static const _keyAccessExpiry = 'sc_access_expiry';
  static const _keyRefreshExpiry = 'sc_refresh_expiry';

  static const _secure = FlutterSecureStorage(
    aOptions: AndroidOptions(encryptedSharedPreferences: true),
    iOptions: IOSOptions(accessibility: KeychainAccessibility.first_unlock),
  );

  Future<void> saveTokens({
    required String accessToken,
    required String refreshToken,
    required String accessExpiresAt,
    required String refreshExpiresAt,
  }) =>
      Future.wait([
        _secure.write(key: _keyAccess, value: accessToken),
        _secure.write(key: _keyRefresh, value: refreshToken),
        _secure.write(key: _keyAccessExpiry, value: accessExpiresAt),
        _secure.write(key: _keyRefreshExpiry, value: refreshExpiresAt),
      ]);

  Future<String?> getAccessToken() => _secure.read(key: _keyAccess);

  Future<String?> getRefreshToken() => _secure.read(key: _keyRefresh);

  Future<bool> isAccessExpired() async {
    final expiry = await _secure.read(key: _keyAccessExpiry);
    if (expiry == null) return true;
    try {
      final dt = DateTime.parse(expiry);
      return DateTime.now().isAfter(dt.subtract(const Duration(seconds: 30)));
    } catch (_) {
      return true;
    }
  }

  Future<bool> isRefreshExpired() async {
    final expiry = await _secure.read(key: _keyRefreshExpiry);
    if (expiry == null) return true;
    try {
      final dt = DateTime.parse(expiry);
      return DateTime.now().isAfter(dt);
    } catch (_) {
      return true;
    }
  }

  Future<void> clear() => Future.wait([
        _secure.delete(key: _keyAccess),
        _secure.delete(key: _keyRefresh),
        _secure.delete(key: _keyAccessExpiry),
        _secure.delete(key: _keyRefreshExpiry),
      ]);
}
