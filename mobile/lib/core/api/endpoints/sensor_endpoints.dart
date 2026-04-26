import '../client.dart';
import '../../../shared/models/sensor_reading.dart';

class SensorEndpoints {
  final ApiClient _client;
  SensorEndpoints(this._client);

  Future<List<SensorReading>> latestByClassroom(String classroomId) =>
      _client.unwrap(
        _client.get('/classrooms/$classroomId/sensors/readings/latest'),
        (d) => (d as List)
            .map((e) => SensorReading.fromJson(e as Map<String, dynamic>))
            .toList(),
      );

  Future<List<SensorReading>> history({
    required String deviceId,
    String? metric,
    String? from,
    String? to,
    int? limit,
  }) =>
      _client.unwrap(
        _client.get('/devices/$deviceId/sensors/readings', queryParameters: {
          if (metric != null) 'metric': metric,
          if (from != null) 'from': from,
          if (to != null) 'to': to,
          if (limit != null) 'limit': limit.toString(),
        }),
        (d) => (d as List)
            .map((e) => SensorReading.fromJson(e as Map<String, dynamic>))
            .toList(),
      );
}
