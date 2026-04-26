import '../client.dart';
import '../../../shared/models/time_point.dart';
import '../../../shared/models/device_usage.dart';

class AnalyticsEndpoints {
  final ApiClient _client;
  AnalyticsEndpoints(this._client);

  Future<List<TimePoint>> sensors({
    required String classroomId,
    required String metric,
    String bucket = 'hour',
    String? from,
    String? to,
  }) =>
      _client.unwrap(
        _client.get('/classrooms/$classroomId/analytics/sensors',
            queryParameters: {
              'metric': metric,
              'bucket': bucket,
              if (from != null) 'from': from,
              if (to != null) 'to': to,
            }),
        (d) => (d as List)
            .map((e) => TimePoint.fromJson(e as Map<String, dynamic>))
            .toList(),
      );

  Future<List<DeviceUsage>> usage({
    required String classroomId,
    String? from,
    String? to,
  }) =>
      _client.unwrap(
        _client.get('/classrooms/$classroomId/analytics/usage',
            queryParameters: {
              if (from != null) 'from': from,
              if (to != null) 'to': to,
            }),
        (d) => (d as List)
            .map((e) => DeviceUsage.fromJson(e as Map<String, dynamic>))
            .toList(),
      );

  Future<double> energy({
    required String classroomId,
    String? from,
    String? to,
  }) =>
      _client.unwrap(
        _client.get('/classrooms/$classroomId/analytics/energy',
            queryParameters: {
              if (from != null) 'from': from,
              if (to != null) 'to': to,
            }),
        (d) => ((d as Map<String, dynamic>)['total'] as num).toDouble(),
      );
}
