import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/endpoints/sensor_endpoints.dart';
import '../models/sensor_reading.dart';
import 'auth_provider.dart';

final sensorEndpointsProvider = Provider<SensorEndpoints>(
  (ref) => SensorEndpoints(ref.watch(apiClientProvider)),
);

final latestSensorsProvider =
    FutureProvider.family<List<SensorReading>, String>((ref, classroomId) {
  return ref.watch(sensorEndpointsProvider).latestByClassroom(classroomId);
});

class SensorState {
  final List<SensorReading> readings;

  const SensorState({this.readings = const []});
}

class SensorNotifier extends StateNotifier<SensorState> {
  final SensorEndpoints _endpoints;
  final String classroomId;

  SensorNotifier(this._endpoints, this.classroomId)
      : super(const SensorState()) {
    load();
  }

  Future<void> load() async {
    try {
      final readings = await _endpoints.latestByClassroom(classroomId);
      state = SensorState(readings: readings);
    } catch (_) {}
  }

  void addReading(SensorReading reading) {
    final list = [...state.readings];
    final idx = list.indexWhere(
        (r) => r.deviceId == reading.deviceId && r.metric == reading.metric);
    if (idx >= 0) {
      list[idx] = reading;
    } else {
      list.add(reading);
    }
    state = SensorState(readings: list);
  }

  double? getMetric(String metric) {
    try {
      return state.readings.firstWhere((r) => r.metric == metric).value;
    } catch (_) {
      return null;
    }
  }
}

final sensorNotifierProvider = StateNotifierProvider.family<SensorNotifier,
    SensorState, String>((ref, classroomId) {
  return SensorNotifier(ref.watch(sensorEndpointsProvider), classroomId);
});
