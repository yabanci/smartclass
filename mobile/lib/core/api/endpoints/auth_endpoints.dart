import '../client.dart';
import '../../../shared/models/auth_response.dart';

class AuthEndpoints {
  final ApiClient _client;
  AuthEndpoints(this._client);

  Future<AuthResponse> login(String email, String password) =>
      _client.unwrap(
        _client.post('/auth/login', data: {'email': email, 'password': password}),
        (d) => AuthResponse.fromJson(d as Map<String, dynamic>),
      );

  Future<AuthResponse> register({
    required String email,
    required String password,
    required String fullName,
    required String role,
    String? language,
    String? phone,
  }) =>
      _client.unwrap(
        _client.post('/auth/register', data: {
          'email': email,
          'password': password,
          'fullName': fullName,
          'role': role,
          if (language != null) 'language': language,
          if (phone != null) 'phone': phone,
        }),
        (d) => AuthResponse.fromJson(d as Map<String, dynamic>),
      );
}
