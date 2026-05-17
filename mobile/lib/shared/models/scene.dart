class SceneStep {
  final String deviceId;
  final String command;
  final dynamic value;

  const SceneStep({
    required this.deviceId,
    required this.command,
    this.value,
  });

  factory SceneStep.fromJson(Map<String, dynamic> json) => SceneStep(
        deviceId: json['deviceId'] as String,
        command: json['command'] as String,
        value: json['value'],
      );

  Map<String, dynamic> toJson() => {
        'deviceId': deviceId,
        'command': command,
        if (value != null) 'value': value,
      };
}

class Scene {
  final String id;
  final String classroomId;
  final String name;
  final String description;
  final List<SceneStep> steps;
  final String createdAt;
  final String updatedAt;

  const Scene({
    required this.id,
    required this.classroomId,
    required this.name,
    this.description = '',
    required this.steps,
    required this.createdAt,
    required this.updatedAt,
  });

  factory Scene.fromJson(Map<String, dynamic> json) => Scene(
        id: json['id'] as String,
        classroomId: json['classroomId'] as String,
        name: json['name'] as String,
        description: (json['description'] as String?) ?? '',
        steps: (json['steps'] as List<dynamic>? ?? [])
            .map((e) => SceneStep.fromJson(e as Map<String, dynamic>))
            .toList(),
        createdAt: json['createdAt'] as String,
        updatedAt: json['updatedAt'] as String,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'classroomId': classroomId,
        'name': name,
        'description': description,
        'steps': steps.map((s) => s.toJson()).toList(),
        'createdAt': createdAt,
        'updatedAt': updatedAt,
      };
}
