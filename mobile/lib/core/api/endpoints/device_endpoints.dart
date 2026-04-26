import '../client.dart';
import '../../../shared/models/device.dart';

class DeviceEndpoints {
  final ApiClient _client;
  DeviceEndpoints(this._client);

  Future<List<Device>> listByClassroom(String classroomId) => _client.unwrap(
        _client.get('/classrooms/$classroomId/devices'),
        (d) => (d as List).map((e) => Device.fromJson(e as Map<String, dynamic>)).toList(),
      );

  Future<Device> create({
    required String classroomId,
    required String name,
    required String type,
    required String brand,
    required String driver,
    Map<String, dynamic>? config,
  }) =>
      _client.unwrap(
        _client.post('/devices', data: {
          'classroomId': classroomId,
          'name': name,
          'type': type,
          'brand': brand,
          'driver': driver,
          if (config != null) 'config': config,
        }),
        (d) => Device.fromJson(d as Map<String, dynamic>),
      );

  Future<Device> get(String id) => _client.unwrap(
        _client.get('/devices/$id'),
        (d) => Device.fromJson(d as Map<String, dynamic>),
      );

  Future<Device> update(String id, Map<String, dynamic> data) => _client.unwrap(
        _client.patch('/devices/$id', data: data),
        (d) => Device.fromJson(d as Map<String, dynamic>),
      );

  Future<void> delete(String id) => _client.unwrap(
        _client.delete('/devices/$id'),
        (_) {},
      );

  Future<Device> sendCommand(
    String id,
    String type, {
    dynamic value,
  }) =>
      _client.unwrap(
        _client.post('/devices/$id/commands', data: {
          'type': type,
          if (value != null) 'value': value,
        }),
        (d) => Device.fromJson(d as Map<String, dynamic>),
      );
}
