import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class TokenStorage {
  static const _keyAccess = 'access_token';
  static const _keyRefresh = 'refresh_token';
  static const _keyAccessExpiry = 'access_expiry';
  static const _keyRefreshExpiry = 'refresh_expiry';

  final FlutterSecureStorage _storage;

  TokenStorage({FlutterSecureStorage? storage})
      : _storage = storage ?? const FlutterSecureStorage();

  Future<void> saveTokens({
    required String accessToken,
    required String refreshToken,
    required String accessExpiresAt,
    required String refreshExpiresAt,
  }) async {
    await Future.wait([
      _storage.write(key: _keyAccess, value: accessToken),
      _storage.write(key: _keyRefresh, value: refreshToken),
      _storage.write(key: _keyAccessExpiry, value: accessExpiresAt),
      _storage.write(key: _keyRefreshExpiry, value: refreshExpiresAt),
    ]);
  }

  Future<String?> getAccessToken() => _storage.read(key: _keyAccess);
  Future<String?> getRefreshToken() => _storage.read(key: _keyRefresh);

  Future<bool> isAccessExpired() async {
    final expiry = await _storage.read(key: _keyAccessExpiry);
    if (expiry == null) return true;
    try {
      final dt = DateTime.parse(expiry);
      return DateTime.now().isAfter(dt.subtract(const Duration(seconds: 30)));
    } catch (_) {
      return true;
    }
  }

  Future<bool> isRefreshExpired() async {
    final expiry = await _storage.read(key: _keyRefreshExpiry);
    if (expiry == null) return true;
    try {
      final dt = DateTime.parse(expiry);
      return DateTime.now().isAfter(dt);
    } catch (_) {
      return true;
    }
  }

  Future<void> clear() async {
    await Future.wait([
      _storage.delete(key: _keyAccess),
      _storage.delete(key: _keyRefresh),
      _storage.delete(key: _keyAccessExpiry),
      _storage.delete(key: _keyRefreshExpiry),
    ]);
  }
}
