class AppNotification {
  final String id;
  final String userId;
  final String? classroomId;
  final String type;
  final String title;
  final String message;
  final Map<String, dynamic>? metadata;
  final String? readAt;
  final String createdAt;

  const AppNotification({
    required this.id,
    required this.userId,
    this.classroomId,
    required this.type,
    required this.title,
    required this.message,
    this.metadata,
    this.readAt,
    required this.createdAt,
  });

  bool get isRead => readAt != null;

  factory AppNotification.fromJson(Map<String, dynamic> json) =>
      AppNotification(
        id: json['id'] as String,
        userId: json['userId'] as String,
        classroomId: json['classroomId'] as String?,
        type: json['type'] as String,
        title: json['title'] as String,
        message: json['message'] as String,
        metadata: json['metadata'] as Map<String, dynamic>?,
        readAt: json['readAt'] as String?,
        createdAt: json['createdAt'] as String,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'userId': userId,
        if (classroomId != null) 'classroomId': classroomId,
        'type': type,
        'title': title,
        'message': message,
        if (metadata != null) 'metadata': metadata,
        if (readAt != null) 'readAt': readAt,
        'createdAt': createdAt,
      };
}
