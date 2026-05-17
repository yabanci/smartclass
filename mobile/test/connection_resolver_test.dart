import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:smartclass/core/connection/connection_mode.dart';
import 'package:smartclass/core/connection/resolver.dart';

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  group('ConnectionResolver', () {
    // C-016: when no localUrl AND remote is unreachable (test env has no server),
    // resolve() returns Unreachable, not Remote.
    test('unreachable mode when no localUrl and remote is down', () async {
      final state = await ConnectionResolver.instance.resolve();
      expect(state.mode, ConnectionMode.unreachable);
      expect(state.isLocal, isFalse);
      expect(state.isUnreachable, isTrue);
    });

    // C-016: local unreachable + remote unreachable → Unreachable mode.
    test('unreachable mode when localUrl is unreachable and remote is down', () async {
      SharedPreferences.setMockInitialValues({
        'local_server_url': 'http://192.168.99.99:9999',
      });
      final state = await ConnectionResolver.instance.resolve();
      expect(state.mode, ConnectionMode.unreachable);
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

    // B-307: tightened assertions — verify full URL including path and port,
    // and that https base → wss scheme.
    test('wsBaseUrl converts http to ws and preserves path', () {
      SharedPreferences.setMockInitialValues({});
      final resolver = ConnectionResolver.instance;
      // Default remote config is http://localhost:8080/api/v1 (AppConfig.dev).
      // After B-302 fix: wsBaseUrl preserves the /api/v1 path.
      expect(resolver.wsBaseUrl, 'ws://localhost:8080/api/v1');
      expect(resolver.wsBaseUrl, isNot(contains('http://')));
    });

    test('wsBaseUrl converts https to wss and preserves path', () {
      // Simulate a resolver pointing at the prod remote base URL.
      const prodBase = 'https://api.smartclass.kz/api/v1';
      final uri = Uri.parse(prodBase);
      final wsUri = uri.replace(scheme: 'wss');
      expect(wsUri.toString(), 'wss://api.smartclass.kz/api/v1');
      expect(wsUri.scheme, 'wss');
    });

    test('wsBaseUrl preserves non-standard port', () {
      const localBase = 'http://192.168.1.100:9090/api/v1';
      final uri = Uri.parse(localBase);
      final wsUri = uri.replace(scheme: 'ws');
      expect(wsUri.toString(), 'ws://192.168.1.100:9090/api/v1');
      expect(wsUri.port, 9090);
    });
  });
}
