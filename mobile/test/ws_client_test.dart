import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:smartclass/core/ws/ws_client.dart';

// A fresh WsClient instance per test — we cannot use the singleton because
// it carries state across tests.  We reach _connectUrl indirectly via connect().
// Since we're testing error propagation in the connect() Future, we only need
// a ticketFactory that throws; the socket upgrade itself won't run.

WsClient _freshClient() {
  // WsClient has a private constructor and exposes a singleton.  For isolation
  // we need to reset it.  The only safe public surface is connect/disconnect/close/dispose.
  // Reset state via close() then test.
  final c = WsClient.instance;
  c.close();
  return c;
}

void main() {
  // ─── V-4: concurrent connect() failure propagation ───────────────────────
  group('WsClient.connect concurrent error propagation (V-4)', () {
    late WsClient client;

    setUp(() {
      client = _freshClient();
    });

    tearDown(() {
      client.close();
    });

    test('second concurrent caller receives error when first connect throws', () async {
      // Arrange: a ticket factory that throws on first call.
      // Both callers issue connect() before the first has resolved; the second
      // awaits _connecting which now completes with an error.
      final barrier = Completer<void>(); // used to hold the factory open briefly
      int callCount = 0;

      Future<String> failingFactory() async {
        callCount++;
        // Wait until both connect() calls have been issued, then throw.
        await barrier.future;
        throw Exception('ticket-service-unavailable');
      }

      // Issue both connect calls "simultaneously" before the factory resolves.
      final f1 = client.connect(
        wsBaseUrl: 'ws://localhost:9999',
        classroomId: 'cls-1',
        ticketFactory: failingFactory,
      );
      // Give the first call time to set _connecting before the second runs.
      await Future.microtask(() {});

      final f2 = client.connect(
        wsBaseUrl: 'ws://localhost:9999',
        classroomId: 'cls-1',
        ticketFactory: failingFactory,
      );

      // Release the barrier so the factory throws.
      barrier.complete();

      // Both futures must complete with an error.
      await expectLater(f1, throwsA(isA<Exception>()));
      await expectLater(f2, throwsA(isA<Exception>()));

      // Factory should only have been called once (second caller awaited the
      // in-flight future rather than calling the factory again).
      expect(callCount, 1,
          reason: 'second caller must piggy-back the first future, not call factory twice');
    });

    test('successful connect does not affect subsequent independent connect', () async {
      // First connect succeeds (factory returns a ticket but socket will fail
      // to upgrade — that's OK, _connectUrl swallows it and schedules reconnect).
      // Key: connect() itself must complete without throwing.
      await client.connect(
        wsBaseUrl: 'ws://localhost:9999',
        classroomId: 'cls-1',
        ticketFactory: () async => 'fake-ticket',
      );
      // No exception thrown — state is as expected.
      // Disconnect to reset.
      client.disconnect();

      // Second independent connect (different classroom) also succeeds.
      await client.connect(
        wsBaseUrl: 'ws://localhost:9999',
        classroomId: 'cls-2',
        ticketFactory: () async => 'fake-ticket-2',
      );
      client.disconnect();
    });
  });
}
