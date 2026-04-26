import '../client.dart';
import '../../../shared/models/lesson.dart';

class ScheduleEndpoints {
  final ApiClient _client;
  ScheduleEndpoints(this._client);

  Future<Map<String, List<Lesson>>> getWeek(String classroomId) =>
      _client.unwrap(
        _client.get('/classrooms/$classroomId/schedule'),
        (d) {
          final map = d as Map<String, dynamic>;
          return map.map((k, v) => MapEntry(
              k,
              (v as List)
                  .map((e) => Lesson.fromJson(e as Map<String, dynamic>))
                  .toList()));
        },
      );

  Future<List<Lesson>> getDay(String classroomId, int day) => _client.unwrap(
        _client.get('/classrooms/$classroomId/schedule/day/$day'),
        (d) => (d as List)
            .map((e) => Lesson.fromJson(e as Map<String, dynamic>))
            .toList(),
      );

  Future<Lesson?> getCurrent(String classroomId) => _client.unwrap(
        _client.get('/classrooms/$classroomId/schedule/current'),
        (d) => d == null ? null : Lesson.fromJson(d as Map<String, dynamic>),
      );

  Future<Lesson> create({
    required String classroomId,
    required String subject,
    required int dayOfWeek,
    required String startsAt,
    required String endsAt,
    String? notes,
  }) =>
      _client.unwrap(
        _client.post('/schedule', data: {
          'classroomId': classroomId,
          'subject': subject,
          'dayOfWeek': dayOfWeek,
          'startsAt': startsAt,
          'endsAt': endsAt,
          if (notes != null) 'notes': notes,
        }),
        (d) => Lesson.fromJson(d as Map<String, dynamic>),
      );

  Future<Lesson> update(String id, Map<String, dynamic> data) => _client.unwrap(
        _client.patch('/schedule/$id', data: data),
        (d) => Lesson.fromJson(d as Map<String, dynamic>),
      );

  Future<void> delete(String id) => _client.unwrap(
        _client.delete('/schedule/$id'),
        (_) {},
      );
}
