import '../../core/api/envelope.dart';
import 'tokens.dart';
import 'user.dart';

class AuthResponse {
  final User user;
  final Tokens tokens;

  const AuthResponse({required this.user, required this.tokens});

  factory AuthResponse.fromJson(Map<String, dynamic> json) {
    final rawUser = json['user'];
    if (rawUser == null) {
      throw const ApiException('Invalid auth response: missing user');
    }
    final rawTokens = json['tokens'];
    if (rawTokens == null) {
      throw const ApiException('Invalid auth response: missing tokens');
    }
    return AuthResponse(
      user: User.fromJson(rawUser as Map<String, dynamic>),
      tokens: Tokens.fromJson(rawTokens as Map<String, dynamic>),
    );
  }
}
