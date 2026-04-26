class Classroom {
  final String id;
  final String name;
  final String? description;
  final String createdBy;
  final String createdAt;
  final String updatedAt;

  const Classroom({
    required this.id,
    required this.name,
    this.description,
    required this.createdBy,
    required this.createdAt,
    required this.updatedAt,
  });

  factory Classroom.fromJson(Map<String, dynamic> json) => Classroom(
        id: json['id'] as String,
        name: json['name'] as String,
        description: json['description'] as String?,
        createdBy: json['createdBy'] as String,
        createdAt: json['createdAt'] as String,
        updatedAt: json['updatedAt'] as String,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'name': name,
        if (description != null) 'description': description,
        'createdBy': createdBy,
        'createdAt': createdAt,
        'updatedAt': updatedAt,
      };

  Classroom copyWith({
    String? id,
    String? name,
    String? description,
    String? createdBy,
    String? createdAt,
    String? updatedAt,
  }) =>
      Classroom(
        id: id ?? this.id,
        name: name ?? this.name,
        description: description ?? this.description,
        createdBy: createdBy ?? this.createdBy,
        createdAt: createdAt ?? this.createdAt,
        updatedAt: updatedAt ?? this.updatedAt,
      );
}
