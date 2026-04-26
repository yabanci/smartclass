import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/endpoints/classroom_endpoints.dart';
import '../models/classroom.dart';
import 'auth_provider.dart';

final classroomEndpointsProvider = Provider<ClassroomEndpoints>(
  (ref) => ClassroomEndpoints(ref.watch(apiClientProvider)),
);

final classroomListProvider =
    StateNotifierProvider<ClassroomListNotifier, AsyncValue<List<Classroom>>>(
        (ref) {
  return ClassroomListNotifier(ref.watch(classroomEndpointsProvider));
});

class ClassroomListNotifier
    extends StateNotifier<AsyncValue<List<Classroom>>> {
  final ClassroomEndpoints _endpoints;

  ClassroomListNotifier(this._endpoints) : super(const AsyncValue.loading()) {
    load();
  }

  Future<void> load() async {
    state = const AsyncValue.loading();
    try {
      final list = await _endpoints.list();
      state = AsyncValue.data(list);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
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
