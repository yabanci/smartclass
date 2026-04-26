import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:smartclass/core/connection/connection_mode.dart';
import 'package:smartclass/core/connection/resolver.dart';

void main() {
  setUp(() {
    // Reset shared preferences for each test
    SharedPreferences.setMockInitialValues({});
  });

  group('ConnectionResolver', () {
    test('returns remote mode when no localUrl is set', () async {
      SharedPreferences.setMockInitialValues({});

      final resolver = ConnectionResolver.instance;
      final state = await resolver.resolve();

      // Without a local URL, should fall back to remote
      expect(state.mode, ConnectionMode.remote);
    });

    test('returns remote mode when localUrl is set but unreachable', () async {
      // Set a URL that definitely won't respond within 600ms
      SharedPreferences.setMockInitialValues({
        'local_server_url': 'http://192.168.99.99:9999',
      });

      final resolver = ConnectionResolver.instance;
      final state = await resolver.resolve();

      // The ping will fail (no server at that address), so remote mode
      expect(state.mode, ConnectionMode.remote);
    });

    test('connection state isLocal is false for remote mode', () {
      const state = ConnectionState(
        mode: ConnectionMode.remote,
        baseUrl: 'http://localhost:8080/api/v1',
      );
      expect(state.isLocal, isFalse);
    });

    test('connection state isLocal is true for local mode', () {
      const state = ConnectionState(
        mode: ConnectionMode.local,
        baseUrl: 'http://192.168.1.100:8080/api/v1',
      );
      expect(state.isLocal, isTrue);
    });

    test('setLocalUrl stores the URL in SharedPreferences', () async {
      SharedPreferences.setMockInitialValues({});

      final resolver = ConnectionResolver.instance;
      // Setting to a known-unreachable URL to avoid real network calls
      await resolver.setLocalUrl('http://192.168.99.99:9999');

      final stored = await resolver.getLocalUrl();
      expect(stored, 'http://192.168.99.99:9999');
    });
  });
}
