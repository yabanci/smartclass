import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:hive_ce_flutter/hive_flutter.dart';

/// A single cached entry returned from [OfflineCache.get].
class CachedEntry<T> {
  final T data;
  final DateTime cachedAt;
  final bool isStale;

  const CachedEntry({
    required this.data,
    required this.cachedAt,
    required this.isStale,
  });
}

/// Singleton read-through cache backed by Hive.
///
/// Box format per key:
///   { "cachedAt": "<ISO-8601>", "data": <json-encodable> }
///
/// Usage:
///   await OfflineCache.instance.init();
///   await OfflineCache.instance.put('classrooms', 'all', jsonList);
///   final entry = OfflineCache.instance.get<List>('classrooms', 'all',
///       parser: (m) => ...,
///   );
class OfflineCache {
  OfflineCache._();

  static final OfflineCache instance = OfflineCache._();

  static const Duration defaultTtl = Duration(hours: 1);

  static const String boxClassrooms = 'classrooms';
  static const String boxSchedules = 'schedules';
  static const String boxDevices = 'devices';
  static const String boxScenes = 'scenes';

  static const List<String> _allBoxNames = [
    boxClassrooms,
    boxSchedules,
    boxDevices,
    boxScenes,
  ];

  bool _initialized = false;

  /// For tests only: marks the instance as initialized so that tests can open
  /// boxes directly without going through [Hive.initFlutter].
  @visibleForTesting
  void markInitialized() => _initialized = true;

  /// Opens all Hive boxes. Idempotent — safe to call multiple times.
  Future<void> init() async {
    if (_initialized) return;
    await Hive.initFlutter();
    for (final name in _allBoxNames) {
      if (!Hive.isBoxOpen(name)) {
        await Hive.openBox<String>(name);
      }
    }
    _initialized = true;
  }

  /// Persists [jsonData] under [key] in the named [boxName].
  ///
  /// [jsonData] must be JSON-encodable (List/Map of primitives).
  Future<void> put(
    String boxName,
    String key,
    dynamic jsonData, {
    Duration ttl = defaultTtl,
  }) async {
    final box = Hive.box<String>(boxName);
    final wrapper = jsonEncode({
      'cachedAt': DateTime.now().toIso8601String(),
      'ttlMs': ttl.inMilliseconds,
      'data': jsonData,
    });
    await box.put(key, wrapper);
  }

  /// Reads the entry stored under [key] in [boxName].
  ///
  /// Returns `null` if key doesn't exist or the box value is corrupt.
  /// [parser] is called with the raw `data` value from the wrapper; if omitted
  /// the raw value is cast to [T] directly.
  CachedEntry<T>? get<T>(
    String boxName,
    String key, {
    T Function(dynamic raw)? parser,
  }) {
    final box = Hive.box<String>(boxName);
    final raw = box.get(key);
    if (raw == null) return null;

    try {
      final wrapper = jsonDecode(raw) as Map<String, dynamic>;
      final cachedAt = DateTime.parse(wrapper['cachedAt'] as String);
      final ttlMs = wrapper['ttlMs'] as int? ?? defaultTtl.inMilliseconds;
      final isStale =
          DateTime.now().difference(cachedAt).inMilliseconds >= ttlMs;
      final data =
          parser != null ? parser(wrapper['data']) : wrapper['data'] as T;
      return CachedEntry<T>(data: data, cachedAt: cachedAt, isStale: isStale);
    } catch (_) {
      return null;
    }
  }

  /// Removes all entries from all boxes (e.g. on logout).
  Future<void> clearAll() async {
    for (final name in _allBoxNames) {
      if (Hive.isBoxOpen(name)) {
        await Hive.box<String>(name).clear();
      }
    }
  }
}
