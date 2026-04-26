import '../client.dart';
import '../../../shared/models/hass_models.dart';
import '../../../shared/models/device.dart';

class HassEndpoints {
  final ApiClient _client;
  HassEndpoints(this._client);

  Future<HassStatus> status() => _client.unwrap(
        _client.get('/hass/status'),
        (d) => HassStatus.fromJson(d as Map<String, dynamic>),
      );

  Future<HassStatus> saveToken(String token) => _client.unwrap(
        _client.post('/hass/token', data: {'token': token}),
        (d) => HassStatus.fromJson(d as Map<String, dynamic>),
      );

  Future<List<HassFlowHandler>> integrations() => _client.unwrap(
        _client.get('/hass/integrations'),
        (d) => (d as List)
            .map((e) => HassFlowHandler.fromJson(e as Map<String, dynamic>))
            .toList(),
      );

  Future<HassFlowStep> startFlow(String handler) => _client.unwrap(
        _client.post('/hass/flows', data: {'handler': handler}),
        (d) => HassFlowStep.fromJson(d as Map<String, dynamic>),
      );

  Future<HassFlowStep> submitStep(String flowId, Map<String, dynamic> data) =>
      _client.unwrap(
        _client.post('/hass/flows/$flowId/step', data: {'data': data}),
        (d) => HassFlowStep.fromJson(d as Map<String, dynamic>),
      );

  Future<void> deleteFlow(String flowId) => _client.unwrap(
        _client.delete('/hass/flows/$flowId'),
        (_) {},
      );

  Future<List<HassEntity>> entities() => _client.unwrap(
        _client.get('/hass/entities'),
        (d) => (d as List)
            .map((e) => HassEntity.fromJson(e as Map<String, dynamic>))
            .toList(),
      );

  Future<Device> adopt({
    required String entityId,
    required String classroomId,
    String? name,
    String? brand,
  }) =>
      _client.unwrap(
        _client.post('/hass/adopt', data: {
          'entityId': entityId,
          'classroomId': classroomId,
          if (name != null) 'name': name,
          if (brand != null) 'brand': brand,
        }),
        (d) => Device.fromJson(d as Map<String, dynamic>),
      );
}
