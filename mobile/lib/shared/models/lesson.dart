class Lesson {
  final String id;
  final String classroomId;
  final String subject;
  final String? teacherId;
  final int dayOfWeek;
  final String startsAt;
  final String endsAt;
  final String? notes;
  final String createdAt;
  final String updatedAt;

  const Lesson({
    required this.id,
    required this.classroomId,
    required this.subject,
    this.teacherId,
    required this.dayOfWeek,
    required this.startsAt,
    required this.endsAt,
    this.notes,
    required this.createdAt,
    required this.updatedAt,
  });

  factory Lesson.fromJson(Map<String, dynamic> json) => Lesson(
        id: json['id'] as String,
        classroomId: json['classroomId'] as String,
        subject: json['subject'] as String,
        teacherId: json['teacherId'] as String?,
        dayOfWeek: json['dayOfWeek'] as int,
        startsAt: json['startsAt'] as String,
        endsAt: json['endsAt'] as String,
        notes: json['notes'] as String?,
        createdAt: json['createdAt'] as String,
        updatedAt: json['updatedAt'] as String,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'classroomId': classroomId,
        'subject': subject,
        if (teacherId != null) 'teacherId': teacherId,
        'dayOfWeek': dayOfWeek,
        'startsAt': startsAt,
        'endsAt': endsAt,
        if (notes != null) 'notes': notes,
        'createdAt': createdAt,
        'updatedAt': updatedAt,
      };
}
