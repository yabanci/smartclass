import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/connection/resolver.dart';
import '../../core/storage/token_storage.dart';
import '../../core/ws/ws_client.dart';
import '../../core/ws/ws_event.dart';
import 'auth_provider.dart';

final wsProvider = Provider<WsClient>((ref) => WsClient.instance);

final wsEventsProvider = StreamProvider<WsEvent>((ref) {
  return ref.watch(wsProvider).events;
});

class WsConnectionNotifier extends StateNotifier<bool> {
  final WsClient _ws;
  final TokenStorage _storage;
  final ConnectionResolver _resolver;

  WsConnectionNotifier(this._ws, this._storage, this._resolver) : super(false);

  Future<void> connectToClassroom(String classroomId) async {
    final token = await _storage.getAccessToken();
    if (token == null) return;

    final baseWs = _resolver.wsBaseUrl;
    final url =
        '$baseWs?topic=classroom:$classroomId:devices&topic=classroom:$classroomId:sensors&access_token=$token';
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
    ref.read(tokenStorageProvider),
    ConnectionResolver.instance,
  );
});
