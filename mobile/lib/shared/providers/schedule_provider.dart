import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/endpoints/schedule_endpoints.dart';
import '../../core/cache/offline_cache.dart';
import '../models/lesson.dart';
import 'auth_provider.dart';

final scheduleEndpointsProvider = Provider<ScheduleEndpoints>(
  (ref) => ScheduleEndpoints(ref.watch(apiClientProvider)),
);

/// `true` when the week schedule displayed was loaded from cache.
/// Keyed by classroomId.
final scheduleFromCacheProvider =
    StateProvider.family<bool, String>((ref, classroomId) => false);

final scheduleProvider = StateNotifierProvider.family<ScheduleNotifier,
    AsyncValue<Map<String, List<Lesson>>>, String>((ref, classroomId) {
  return ScheduleNotifier(
    ref.watch(scheduleEndpointsProvider),
    classroomId,
    ref,
  );
});

class ScheduleNotifier
    extends StateNotifier<AsyncValue<Map<String, List<Lesson>>>> {
  final ScheduleEndpoints _endpoints;
  final String classroomId;
  final Ref _ref;

  ScheduleNotifier(this._endpoints, this.classroomId, this._ref)
      : super(const AsyncValue.loading()) {
    load();
  }

  String get _cacheKey => 'schedule:$classroomId';

  Future<void> load() async {
    state = const AsyncValue.loading();
    try {
      final week = await _endpoints.getWeek(classroomId);
      // Persist: serialise Map<String, List<Lesson>> as Map<String, List<Map>>
      final serialised = week.map(
        (day, lessons) =>
            MapEntry(day, lessons.map((l) => l.toJson()).toList()),
      );
      await OfflineCache.instance.put(
        OfflineCache.boxSchedules,
        _cacheKey,
        serialised,
      );
      _ref.read(scheduleFromCacheProvider(classroomId).notifier).state = false;
      state = AsyncValue.data(week);
    } catch (e, st) {
      final entry = OfflineCache.instance.get<Map<String, List<Lesson>>>(
        OfflineCache.boxSchedules,
        _cacheKey,
        parser: (raw) {
          final map = raw as Map<String, dynamic>;
          return map.map(
            (day, lessons) => MapEntry(
              day,
              (lessons as List<dynamic>)
                  .map((l) => Lesson.fromJson(l as Map<String, dynamic>))
                  .toList(),
            ),
          );
        },
      );
      if (entry != null) {
        _ref.read(scheduleFromCacheProvider(classroomId).notifier).state = true;
        state = AsyncValue.data(entry.data);
      } else {
        _ref
            .read(scheduleFromCacheProvider(classroomId).notifier)
            .state = false;
        state = AsyncValue.error(e, st);
      }
    }
  }

  Future<void> addLesson({
    required String subject,
    required int dayOfWeek,
    required String startsAt,
    required String endsAt,
    String? notes,
  }) async {
    await _endpoints.create(
      classroomId: classroomId,
      subject: subject,
      dayOfWeek: dayOfWeek,
      startsAt: startsAt,
      endsAt: endsAt,
      notes: notes,
    );
    await load();
  }

  Future<void> deleteLesson(String id) async {
    await _endpoints.delete(id);
    await load();
  }
}

final currentLessonProvider =
    FutureProvider.family<Lesson?, String>((ref, classroomId) {
  final endpoints = ref.watch(scheduleEndpointsProvider);
  return endpoints.getCurrent(classroomId);
});
