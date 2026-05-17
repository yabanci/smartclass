import '../client.dart';
import '../../../shared/models/user.dart';

class UserEndpoints {
  final ApiClient _client;
  UserEndpoints(this._client);

  Future<User> getMe() => _client.unwrap(
        _client.get('/users/me'),
        (d) => User.fromJson(d as Map<String, dynamic>),
      );

  Future<User> updateMe({
    String? fullName,
    String? language,
    String? avatarUrl,
    String? phone,
  }) =>
      _client.unwrap(
        _client.patch('/users/me', data: {
          if (fullName != null) 'fullName': fullName,
          if (language != null) 'language': language,
          if (avatarUrl != null) 'avatarUrl': avatarUrl,
          if (phone != null) 'phone': phone,
        }),
        (d) => User.fromJson(d as Map<String, dynamic>),
      );

  Future<void> changePassword({
    required String currentPassword,
    required String newPassword,
  }) =>
      _client.unwrap(
        _client.post('/users/me/password', data: {
          'currentPassword': currentPassword,
          'newPassword': newPassword,
        }),
        (_) {},
      );

  Future<void> registerDeviceToken({
    required String token,
    required String platform,
  }) =>
      _client.unwrap(
        _client.post(
          '/me/device-tokens',
          data: {'token': token, 'platform': platform},
        ),
        (_) {},
      );

  Future<void> unregisterDeviceToken(String token) => _client.unwrap(
        _client.delete('/me/device-tokens/$token'),
        (_) {},
      );
}
