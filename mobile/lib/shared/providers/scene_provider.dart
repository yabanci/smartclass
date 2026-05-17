import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/endpoints/scene_endpoints.dart';
import '../../core/cache/offline_cache.dart';
import '../models/scene.dart';
import 'auth_provider.dart';

final sceneEndpointsProvider = Provider<SceneEndpoints>(
  (ref) => SceneEndpoints(ref.watch(apiClientProvider)),
);

/// `true` when the scene list currently displayed was loaded from cache.
/// Keyed by classroomId.
final sceneFromCacheProvider =
    StateProvider.family<bool, String>((ref, classroomId) => false);

final sceneListProvider = StateNotifierProvider.family<SceneListNotifier,
    AsyncValue<List<Scene>>, String>((ref, classroomId) {
  return SceneListNotifier(
    ref.watch(sceneEndpointsProvider),
    classroomId,
    ref,
  );
});

class SceneListNotifier extends StateNotifier<AsyncValue<List<Scene>>> {
  final SceneEndpoints _endpoints;
  final String classroomId;
  final Ref _ref;

  SceneListNotifier(this._endpoints, this.classroomId, this._ref)
      : super(const AsyncValue.loading()) {
    load();
  }

  String get _cacheKey => 'scenes:$classroomId';

  Future<void> load() async {
    state = const AsyncValue.loading();
    try {
      final list = await _endpoints.listByClassroom(classroomId);
      await OfflineCache.instance.put(
        OfflineCache.boxScenes,
        _cacheKey,
        list.map((s) => s.toJson()).toList(),
      );
      _ref.read(sceneFromCacheProvider(classroomId).notifier).state = false;
      state = AsyncValue.data(list);
    } catch (e, st) {
      final entry = OfflineCache.instance.get<List<Scene>>(
        OfflineCache.boxScenes,
        _cacheKey,
        parser: (raw) => (raw as List<dynamic>)
            .map((e) => Scene.fromJson(e as Map<String, dynamic>))
            .toList(),
      );
      if (entry != null) {
        _ref.read(sceneFromCacheProvider(classroomId).notifier).state = true;
        state = AsyncValue.data(entry.data);
      } else {
        _ref.read(sceneFromCacheProvider(classroomId).notifier).state = false;
        state = AsyncValue.error(e, st);
      }
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
