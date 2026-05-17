import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/endpoints/classroom_endpoints.dart';
import '../../core/cache/offline_cache.dart';
import '../models/classroom.dart';
import 'auth_provider.dart';

final classroomEndpointsProvider = Provider<ClassroomEndpoints>(
  (ref) => ClassroomEndpoints(ref.watch(apiClientProvider)),
);

/// `true` when the list currently displayed was loaded from cache.
final classroomFromCacheProvider = StateProvider<bool>((ref) => false);

final classroomListProvider =
    StateNotifierProvider<ClassroomListNotifier, AsyncValue<List<Classroom>>>(
        (ref) {
  return ClassroomListNotifier(
    ref.watch(classroomEndpointsProvider),
    ref,
  );
});

class ClassroomListNotifier
    extends StateNotifier<AsyncValue<List<Classroom>>> {
  final ClassroomEndpoints _endpoints;
  final Ref _ref;

  ClassroomListNotifier(this._endpoints, this._ref)
      : super(const AsyncValue.loading()) {
    load();
  }

  Future<void> load() async {
    state = const AsyncValue.loading();
    try {
      final list = await _endpoints.list();
      // Persist fresh data to cache
      await OfflineCache.instance.put(
        OfflineCache.boxClassrooms,
        'all',
        list.map((c) => c.toJson()).toList(),
      );
      _ref.read(classroomFromCacheProvider.notifier).state = false;
      state = AsyncValue.data(list);
    } catch (e, st) {
      // Fall back to cache on any error
      final entry = OfflineCache.instance.get<List<Classroom>>(
        OfflineCache.boxClassrooms,
        'all',
        parser: (raw) => (raw as List<dynamic>)
            .map((e) => Classroom.fromJson(e as Map<String, dynamic>))
            .toList(),
      );
      if (entry != null) {
        _ref.read(classroomFromCacheProvider.notifier).state = true;
        state = AsyncValue.data(entry.data);
      } else {
        _ref.read(classroomFromCacheProvider.notifier).state = false;
        state = AsyncValue.error(e, st);
      }
    }
  }

  // Throws on error — callers must handle
  Future<Classroom> create(String name, {String? description}) async {
    final classroom =
        await _endpoints.create(name: name, description: description);
    await load();
    return classroom;
  }

  Future<void> delete(String id) async {
    await _endpoints.delete(id);
    await load();
  }
}

final activeClassroomProvider =
    StateNotifierProvider<ActiveClassroomNotifier, Classroom?>((ref) {
  return ActiveClassroomNotifier();
});

class ActiveClassroomNotifier extends StateNotifier<Classroom?> {
  ActiveClassroomNotifier() : super(null);
  void select(Classroom classroom) => state = classroom;
  void clear() => state = null;
}
