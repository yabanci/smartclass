import 'package:flutter_test/flutter_test.dart';
import 'package:hive_ce/hive.dart';
import 'package:smartclass/core/cache/offline_cache.dart';

void main() {
  /// In tests, Hive is initialised in a temp directory via [setUp].
  /// We open boxes manually because [OfflineCache.init] calls
  /// [Hive.initFlutter] which requires Flutter binding.

  setUp(() async {
    Hive.init('test_hive_${DateTime.now().microsecondsSinceEpoch}');
    // Open the boxes the tests will use
    for (final name in [
      OfflineCache.boxClassrooms,
      OfflineCache.boxSchedules,
      OfflineCache.boxDevices,
      OfflineCache.boxScenes,
    ]) {
      if (!Hive.isBoxOpen(name)) {
        await Hive.openBox<String>(name);
      }
    }
    // Bypass the Flutter-specific init
    // ignore: invalid_use_of_visible_for_testing_member
    OfflineCache.instance.markInitialized();
  });

  tearDown(() async {
    await Hive.deleteFromDisk();
  });

  group('OfflineCache.put / get round-trip', () {
    test('stores and retrieves a list', () async {
      final data = [
        {'id': '1', 'name': 'Room A'},
        {'id': '2', 'name': 'Room B'},
      ];

      await OfflineCache.instance.put(OfflineCache.boxClassrooms, 'all', data);

      final entry = OfflineCache.instance.get<List<dynamic>>(
        OfflineCache.boxClassrooms,
        'all',
      );

      expect(entry, isNotNull);
      expect(entry!.data, hasLength(2));
      expect((entry.data[0] as Map)['name'], 'Room A');
    });

    test('missing key returns null', () {
      final entry = OfflineCache.instance.get<List<dynamic>>(
        OfflineCache.boxClassrooms,
        'nonexistent',
      );
      expect(entry, isNull);
    });

    test('parser is applied when provided', () async {
      final data = [
        {'id': '1', 'name': 'Room A'},
      ];

      await OfflineCache.instance.put(OfflineCache.boxClassrooms, 'all', data);

      final entry = OfflineCache.instance.get<List<String>>(
        OfflineCache.boxClassrooms,
        'all',
        parser: (raw) =>
            (raw as List<dynamic>).map((e) => e['name'] as String).toList(),
      );

      expect(entry, isNotNull);
      expect(entry!.data, ['Room A']);
    });

    test('isStale is false just after writing with 1h TTL', () async {
      await OfflineCache.instance.put(
        OfflineCache.boxClassrooms,
        'all',
        [1, 2, 3],
        ttl: const Duration(hours: 1),
      );

      final entry = OfflineCache.instance.get<List<dynamic>>(
        OfflineCache.boxClassrooms,
        'all',
      );

      expect(entry, isNotNull);
      expect(entry!.isStale, isFalse);
    });

    test('isStale is true when TTL is zero', () async {
      await OfflineCache.instance.put(
        OfflineCache.boxClassrooms,
        'all',
        [1, 2, 3],
        // TTL of zero means immediately stale
        ttl: Duration.zero,
      );

      final entry = OfflineCache.instance.get<List<dynamic>>(
        OfflineCache.boxClassrooms,
        'all',
      );

      expect(entry, isNotNull);
      expect(entry!.isStale, isTrue);
    });

    test('corrupt entry returns null', () async {
      final box = Hive.box<String>(OfflineCache.boxClassrooms);
      await box.put('bad', 'not-valid-json{{{{');

      final entry = OfflineCache.instance.get<List<dynamic>>(
        OfflineCache.boxClassrooms,
        'bad',
      );

      expect(entry, isNull);
    });
  });
}
