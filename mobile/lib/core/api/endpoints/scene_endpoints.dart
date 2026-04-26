import '../client.dart';
import '../../../shared/models/scene.dart';

class SceneRunResult {
  final String sceneId;
  final List<SceneStepResult> steps;

  const SceneRunResult({required this.sceneId, required this.steps});

  factory SceneRunResult.fromJson(Map<String, dynamic> json) => SceneRunResult(
        sceneId: json['sceneId'] as String,
        steps: (json['steps'] as List<dynamic>? ?? [])
            .map((e) => SceneStepResult.fromJson(e as Map<String, dynamic>))
            .toList(),
      );

  int get successCount => steps.where((s) => s.success).length;
  int get total => steps.length;
}

class SceneStepResult {
  final SceneStep step;
  final bool success;
  final String? error;

  const SceneStepResult({
    required this.step,
    required this.success,
    this.error,
  });

  factory SceneStepResult.fromJson(Map<String, dynamic> json) =>
      SceneStepResult(
        step: SceneStep.fromJson(json['step'] as Map<String, dynamic>),
        success: json['success'] as bool,
        error: json['error'] as String?,
      );
}

class SceneEndpoints {
  final ApiClient _client;
  SceneEndpoints(this._client);

  Future<List<Scene>> listByClassroom(String classroomId) => _client.unwrap(
        _client.get('/classrooms/$classroomId/scenes'),
        (d) => (d as List)
            .map((e) => Scene.fromJson(e as Map<String, dynamic>))
            .toList(),
      );

  Future<Scene> create({
    required String classroomId,
    required String name,
    String? description,
    required List<SceneStep> steps,
  }) =>
      _client.unwrap(
        _client.post('/scenes', data: {
          'classroomId': classroomId,
          'name': name,
          if (description != null) 'description': description,
          'steps': steps.map((s) => s.toJson()).toList(),
        }),
        (d) => Scene.fromJson(d as Map<String, dynamic>),
      );

  Future<Scene> get(String id) => _client.unwrap(
        _client.get('/scenes/$id'),
        (d) => Scene.fromJson(d as Map<String, dynamic>),
      );

  Future<Scene> update(String id, Map<String, dynamic> data) => _client.unwrap(
        _client.patch('/scenes/$id', data: data),
        (d) => Scene.fromJson(d as Map<String, dynamic>),
      );

  Future<void> delete(String id) => _client.unwrap(
        _client.delete('/scenes/$id'),
        (_) => null,
      );

  Future<SceneRunResult> run(String id) => _client.unwrap(
        _client.post('/scenes/$id/run'),
        (d) => SceneRunResult.fromJson(d as Map<String, dynamic>),
      );
}
