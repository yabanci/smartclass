import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/endpoints/device_endpoints.dart';
import '../models/device.dart';
import 'auth_provider.dart';

final deviceEndpointsProvider = Provider<DeviceEndpoints>(
  (ref) => DeviceEndpoints(ref.watch(apiClientProvider)),
);

final deviceListProvider = StateNotifierProvider.family<DeviceListNotifier,
    AsyncValue<List<Device>>, String>((ref, classroomId) {
  return DeviceListNotifier(
    ref.watch(deviceEndpointsProvider),
    classroomId,
  );
});

class DeviceListNotifier extends StateNotifier<AsyncValue<List<Device>>> {
  final DeviceEndpoints _endpoints;
  final String classroomId;

  DeviceListNotifier(this._endpoints, this.classroomId)
      : super(const AsyncValue.loading()) {
    load();
  }

  Future<void> load() async {
    state = const AsyncValue.loading();
    try {
      final list = await _endpoints.listByClassroom(classroomId);
      state = AsyncValue.data(list);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
    }
  }

  Future<void> sendCommand(String deviceId, String type, {dynamic value}) async {
    try {
      final updated =
          await _endpoints.sendCommand(deviceId, type, value: value);
      state.whenData((devices) {
        state = AsyncValue.data([
          for (final d in devices)
            if (d.id == deviceId) updated else d
        ]);
      });
    } catch (_) {
      await load();
    }
  }

  Future<Device?> create({
    required String name,
    required String type,
    required String brand,
    required String driver,
    Map<String, dynamic>? config,
  }) async {
    try {
      final device = await _endpoints.create(
        classroomId: classroomId,
        name: name,
        type: type,
        brand: brand,
        driver: driver,
        config: config,
      );
      load();
      return device;
    } catch (_) {
      return null;
    }
  }

  Future<void> delete(String id) async {
    await _endpoints.delete(id);
    load();
  }

  void updateDevice(Device updated) {
    state.whenData((devices) {
      final idx = devices.indexWhere((d) => d.id == updated.id);
      if (idx >= 0) {
        final newList = [...devices];
        newList[idx] = updated;
        state = AsyncValue.data(newList);
      } else {
        state = AsyncValue.data([...devices, updated]);
      }
    });
  }
}
