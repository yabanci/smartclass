class Tokens {
  final String accessToken;
  final String refreshToken;
  final String accessExpiresAt;
  final String refreshExpiresAt;
  final String tokenType;

  const Tokens({
    required this.accessToken,
    required this.refreshToken,
    required this.accessExpiresAt,
    required this.refreshExpiresAt,
    required this.tokenType,
  });

  factory Tokens.fromJson(Map<String, dynamic> json) => Tokens(
        accessToken: json['accessToken'] as String,
        refreshToken: json['refreshToken'] as String,
        accessExpiresAt: json['accessExpiresAt'] as String,
        refreshExpiresAt: json['refreshExpiresAt'] as String,
        tokenType: (json['tokenType'] as String?) ?? 'Bearer',
      );

  Map<String, dynamic> toJson() => {
        'accessToken': accessToken,
        'refreshToken': refreshToken,
        'accessExpiresAt': accessExpiresAt,
        'refreshExpiresAt': refreshExpiresAt,
        'tokenType': tokenType,
      };
}
