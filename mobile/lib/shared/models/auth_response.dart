import 'tokens.dart';
import 'user.dart';

class AuthResponse {
  final User user;
  final Tokens tokens;

  const AuthResponse({required this.user, required this.tokens});

  factory AuthResponse.fromJson(Map<String, dynamic> json) => AuthResponse(
        user: User.fromJson(json['user'] as Map<String, dynamic>),
        tokens: Tokens.fromJson(json['tokens'] as Map<String, dynamic>),
      );
}
