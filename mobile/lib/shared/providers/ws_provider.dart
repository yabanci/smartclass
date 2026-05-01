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
  Future<void> connectToClassroom(String classroomId) async {
    final ticket = await _wsEndpoints.createTicket();
    final baseWs = _resolver.wsBaseUrl;
    final url = '$baseWs/ws'
        '?topic=classroom:$classroomId:devices'
        '&topic=classroom:$classroomId:sensors'
        '&ticket=$ticket';
    _ws.connect(url);
    state = true;
  }

  void disconnect() {
    _ws.disconnect();
    state = false;
  }
}

final wsConnectionProvider =
    StateNotifierProvider<WsConnectionNotifier, bool>((ref) {
  return WsConnectionNotifier(
    ref.watch(wsProvider),
    WsEndpoints(ref.watch(apiClientProvider)),
    ConnectionResolver.instance,
  );
});
