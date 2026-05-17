import 'dart:async';
import 'dart:convert';

import 'package:web_socket_channel/web_socket_channel.dart';

import 'ws_event.dart';

class WsClient {
  WsClient._();
  static final WsClient instance = WsClient._();

  WebSocketChannel? _channel;
  StreamSubscription? _sub;
  Timer? _reconnectTimer;

  final _controller = StreamController<WsEvent>.broadcast();
  Stream<WsEvent> get events => _controller.stream;

  String? _currentUrl;
  bool _disposed = false;

  void connect(String wsUrl) {
    _disposed = false;
    if (_currentUrl == wsUrl) return;
    _currentUrl = wsUrl;
    _dispose();
    _connect(wsUrl);
  }

  void disconnect() {
    _currentUrl = null;
    _dispose();
  }

  void _dispose() {
    _reconnectTimer?.cancel();
    _reconnectTimer = null;
    _sub?.cancel();
    _sub = null;
    _channel?.sink.close();
    _channel = null;
  }

  void _connect(String url) {
    if (_disposed) return;
    try {
      _channel = WebSocketChannel.connect(Uri.parse(url));
      _sub = _channel!.stream.listen(
        (data) {
          // B-007: guard against binary frames — only handle String messages
          if (data is! String) return;
          try {
            final decoded = jsonDecode(data) as Map<String, dynamic>;
            _controller.add(WsEvent.fromJson(decoded));
          } catch (_) {
            // ignore malformed messages
          }
        },
        onError: (_) => _scheduleReconnect(url),
        onDone: () => _scheduleReconnect(url),
      );
    } catch (_) {
      _scheduleReconnect(url);
    }
  }

  void _scheduleReconnect(String url) {
    _sub?.cancel();
    _sub = null;
    _channel?.sink.close();
    _channel = null;
    _reconnectTimer?.cancel();
    _reconnectTimer = Timer(const Duration(seconds: 3), () {
      if (_currentUrl == url) _connect(url);
    });
  }

  void close() {
    _disposed = true;
    _dispose();
    _controller.close();
  }
}
