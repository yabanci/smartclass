import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/endpoints/ws_endpoints.dart';
import '../../core/connection/resolver.dart';
import '../../core/ws/ws_client.dart';
import '../../core/ws/ws_event.dart';
import 'auth_provider.dart';

final wsProvider = Provider<WsClient>((ref) => WsClient.instance);

final wsEventsProvider = StreamProvider<WsEvent>((ref) {
  return ref.watch(wsProvider).events;
});

class WsConnectionNotifier extends StateNotifier<bool> {
  final WsClient _ws;
  final WsEndpoints _wsEndpoints;
  final ConnectionResolver _resolver;

  WsConnectionNotifier(this._ws, this._wsEndpoints, this._resolver)
      : super(false);

  /// Connects the WebSocket. The auth flow is:
  /// 1) POST /ws/ticket with the access JWT (added by ApiClient's
  ///    interceptor) → backend returns a 60-second single-use ticket.
  /// 2) Build the WS URL with `?ticket=<x>` and connect.
  ///
  /// This avoids putting the long-lived JWT into the URL — query strings get
  /// logged by reverse proxies; tickets are one-shot and short-lived.
  ///
  /// C-006: ticketFactory is passed so reconnects always mint a fresh ticket.
  /// C-007: classroom:<id>:scenes topic added.
  /// C-019: state is only set to true after the connect Future resolves without
  ///         throwing, so a failed socket doesn't falsely advertise "connected".
  Future<void> connectToClassroom(String classroomId) async {
    final baseWs = _resolver.wsBaseUrl;

    // Factory captures classroomId only for ticket scoping; the ticket itself
    // is not classroom-scoped on the backend — this is a fresh single-use ticket.
    Future<String> ticketFactory() => _wsEndpoints.createTicket();

    try {
      // B-302: baseWs already contains /api/v1 (e.g. ws://host:8080/api/v1);
      // WsClient.connect appends /ws to reach the backend route at /api/v1/ws.
      await _ws.connect(
        wsBaseUrl: baseWs,
        classroomId: classroomId,
        ticketFactory: ticketFactory,
      );
      // C-019: only set true after connect succeeds.
      state = true;
    } catch (_) {
      // Connect failed (ticket fetch failed, socket error, etc.) — stay false.
      state = false;
      rethrow;
    }
  }

  void disconnect() {
    _ws.disconnect();
    state = false;
  }
}

final wsConnectionProvider =
    StateNotifierProvider<WsConnectionNotifier, bool>((ref) {
  // B-005: watch apiClientProvider so WsEndpoints is rebuilt when client changes
  final apiClient = ref.watch(apiClientProvider);
  return WsConnectionNotifier(
    ref.watch(wsProvider),
    WsEndpoints(apiClient),
    ConnectionResolver.instance,
  );
});
