import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/endpoints/scene_endpoints.dart';
import '../models/scene.dart';
import 'auth_provider.dart';

final sceneEndpointsProvider = Provider<SceneEndpoints>(
  (ref) => SceneEndpoints(ref.watch(apiClientProvider)),
);

final sceneListProvider = StateNotifierProvider.family<SceneListNotifier,
    AsyncValue<List<Scene>>, String>((ref, classroomId) {
  return SceneListNotifier(ref.watch(sceneEndpointsProvider), classroomId);
});

class SceneListNotifier extends StateNotifier<AsyncValue<List<Scene>>> {
  final SceneEndpoints _endpoints;
  final String classroomId;

  SceneListNotifier(this._endpoints, this.classroomId)
      : super(const AsyncValue.loading()) {
    load();
  }

  Future<void> load() async {
    state = const AsyncValue.loading();
    try {
      final list = await _endpoints.listByClassroom(classroomId);
      state = AsyncValue.data(list);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
    }
  }

  // Throws on error — caller shows message to user
  Future<SceneRunResult> run(String sceneId) =>
      _endpoints.run(sceneId);

  Future<void> create({
    required String name,
    String? description,
    required List<SceneStep> steps,
  }) async {
    await _endpoints.create(
      classroomId: classroomId,
      name: name,
      description: description,
      steps: steps,
    );
    await load();
  }

  Future<void> delete(String id) async {
    await _endpoints.delete(id);
    await load();
  }
}
