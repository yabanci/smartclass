import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/endpoints/schedule_endpoints.dart';
import '../models/lesson.dart';
import 'auth_provider.dart';

final scheduleEndpointsProvider = Provider<ScheduleEndpoints>(
  (ref) => ScheduleEndpoints(ref.watch(apiClientProvider)),
);

final scheduleProvider = StateNotifierProvider.family<ScheduleNotifier,
    AsyncValue<Map<String, List<Lesson>>>, String>((ref, classroomId) {
  return ScheduleNotifier(ref.watch(scheduleEndpointsProvider), classroomId);
});

class ScheduleNotifier
    extends StateNotifier<AsyncValue<Map<String, List<Lesson>>>> {
  final ScheduleEndpoints _endpoints;
  final String classroomId;

  ScheduleNotifier(this._endpoints, this.classroomId)
      : super(const AsyncValue.loading()) {
    load();
  }

  Future<void> load() async {
    state = const AsyncValue.loading();
    try {
      final week = await _endpoints.getWeek(classroomId);
      state = AsyncValue.data(week);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
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
