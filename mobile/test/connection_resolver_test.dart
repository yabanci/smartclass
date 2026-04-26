import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:smartclass/core/connection/connection_mode.dart';
import 'package:smartclass/core/connection/resolver.dart';

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  group('ConnectionResolver', () {
    test('remote mode when no localUrl', () async {
      final state = await ConnectionResolver.instance.resolve();
      expect(state.mode, ConnectionMode.remote);
      expect(state.isLocal, isFalse);
    });

    test('remote mode when localUrl is unreachable', () async {
      SharedPreferences.setMockInitialValues({
        'local_server_url': 'http://192.168.99.99:9999',
      });
      final state = await ConnectionResolver.instance.resolve();
      expect(state.mode, ConnectionMode.remote);
    });

    test('setLocalUrl persists to SharedPreferences', () async {
      await ConnectionResolver.instance
          .setLocalUrl('http://192.168.1.100:8080');
      final stored = await ConnectionResolver.instance.getLocalUrl();
      expect(stored, 'http://192.168.1.100:8080');
    });

    test('ConnectionState.isLocal true for local mode', () {
      const s = ConnectionState(
          mode: ConnectionMode.local,
          baseUrl: 'http://192.168.1.100:8080/api/v1');
      expect(s.isLocal, isTrue);
    });

    test('ConnectionState.isLocal false for remote mode', () {
      const s = ConnectionState(
          mode: ConnectionMode.remote,
          baseUrl: 'http://localhost:8080/api/v1');
      expect(s.isLocal, isFalse);
    });

    test('wsBaseUrl converts http to ws', () {
      SharedPreferences.setMockInitialValues({});
      final resolver = ConnectionResolver.instance;
      // default remote URL
      expect(resolver.wsBaseUrl, contains('ws://'));
      expect(resolver.wsBaseUrl, isNot(contains('http://')));
    });
  });
}
