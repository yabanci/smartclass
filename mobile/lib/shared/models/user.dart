class User {
  final String id;
  final String email;
  final String fullName;
  final String role;
  final String language;
  final String? avatarUrl;
  final String? phone;
  final String? birthDate;
  final String createdAt;
  final String updatedAt;

  const User({
    required this.id,
    required this.email,
    required this.fullName,
    required this.role,
    required this.language,
    this.avatarUrl,
    this.phone,
    this.birthDate,
    required this.createdAt,
    required this.updatedAt,
  });

  factory User.fromJson(Map<String, dynamic> json) => User(
        id: json['id'] as String,
        email: json['email'] as String,
        fullName: json['fullName'] as String,
        role: json['role'] as String,
        language: (json['language'] as String?) ?? 'en',
        avatarUrl: json['avatarUrl'] as String?,
        phone: json['phone'] as String?,
        birthDate: json['birthDate'] as String?,
        createdAt: json['createdAt'] as String,
        updatedAt: json['updatedAt'] as String,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'email': email,
        'fullName': fullName,
        'role': role,
        'language': language,
        if (avatarUrl != null) 'avatarUrl': avatarUrl,
        if (phone != null) 'phone': phone,
        if (birthDate != null) 'birthDate': birthDate,
        'createdAt': createdAt,
        'updatedAt': updatedAt,
      };

  User copyWith({
    String? id,
    String? email,
    String? fullName,
    String? role,
    String? language,
    String? avatarUrl,
    String? phone,
    String? birthDate,
    String? createdAt,
    String? updatedAt,
  }) =>
      User(
        id: id ?? this.id,
        email: email ?? this.email,
        fullName: fullName ?? this.fullName,
        role: role ?? this.role,
        language: language ?? this.language,
        avatarUrl: avatarUrl ?? this.avatarUrl,
        phone: phone ?? this.phone,
        birthDate: birthDate ?? this.birthDate,
        createdAt: createdAt ?? this.createdAt,
        updatedAt: updatedAt ?? this.updatedAt,
      );
}
