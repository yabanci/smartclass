import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/endpoints/device_endpoints.dart';
import '../../core/cache/offline_cache.dart';
import '../models/device.dart';
import 'auth_provider.dart';

final deviceEndpointsProvider = Provider<DeviceEndpoints>(
  (ref) => DeviceEndpoints(ref.watch(apiClientProvider)),
);

/// `true` when the device list currently displayed was loaded from cache.
/// Keyed by classroomId.
final deviceFromCacheProvider =
    StateProvider.family<bool, String>((ref, classroomId) => false);

final deviceListProvider = StateNotifierProvider.family<DeviceListNotifier,
    AsyncValue<List<Device>>, String>((ref, classroomId) {
  return DeviceListNotifier(
    ref.watch(deviceEndpointsProvider),
    classroomId,
    ref,
  );
});

class DeviceListNotifier extends StateNotifier<AsyncValue<List<Device>>> {
  final DeviceEndpoints _endpoints;
  final String classroomId;
  final Ref _ref;

  DeviceListNotifier(this._endpoints, this.classroomId, this._ref)
      : super(const AsyncValue.loading()) {
    load();
  }

  String get _cacheKey => 'devices:$classroomId';

  Future<void> load() async {
    state = const AsyncValue.loading();
    try {
      final list = await _endpoints.listByClassroom(classroomId);
      await OfflineCache.instance.put(
        OfflineCache.boxDevices,
        _cacheKey,
        list.map((d) => d.toJson()).toList(),
      );
      _ref.read(deviceFromCacheProvider(classroomId).notifier).state = false;
      state = AsyncValue.data(list);
    } catch (e, st) {
      final entry = OfflineCache.instance.get<List<Device>>(
        OfflineCache.boxDevices,
        _cacheKey,
        parser: (raw) => (raw as List<dynamic>)
            .map((e) => Device.fromJson(e as Map<String, dynamic>))
            .toList(),
      );
      if (entry != null) {
        _ref.read(deviceFromCacheProvider(classroomId).notifier).state = true;
        state = AsyncValue.data(entry.data);
      } else {
        _ref.read(deviceFromCacheProvider(classroomId).notifier).state = false;
        state = AsyncValue.error(e, st);
      }
    }
  }

  Future<void> sendCommand(String deviceId, String type, {dynamic value}) async {
    final updated = await _endpoints.sendCommand(deviceId, type, value: value);
    // B-106: update optimistic state first, then reload unconditionally
    state.whenData((devices) {
      state = AsyncValue.data([
        for (final d in devices) if (d.id == deviceId) updated else d
      ]);
    });
    await load();
  }

  Future<Device> create({
    required String name,
    required String type,
    required String brand,
    required String driver,
    Map<String, dynamic>? config,
  }) async {
    final device = await _endpoints.create(
      classroomId: classroomId,
      name: name,
      type: type,
      brand: brand,
      driver: driver,
      config: config,
    );
    await load();
    return device;
  }

  Future<void> delete(String id) async {
    await _endpoints.delete(id);
    // B-107: await load() so callers know when the list is refreshed
    await load();
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
