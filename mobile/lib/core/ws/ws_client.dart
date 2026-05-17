import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import 'ws_event.dart';

typedef TicketFactory = Future<String> Function();

enum WsState { connected, reconnecting, failed }

class WsClient {
  WsClient._();
  static final WsClient instance = WsClient._();

  WebSocketChannel? _channel;
  StreamSubscription? _sub;
  Timer? _reconnectTimer;

  // FA-2: use a broadcast StreamController that is recreated on each connect()
  // so that calling close() then connect() does not add events to a closed
  // stream. The public `events` getter always returns the current controller's
  // stream; listeners that survive a reconnect will re-subscribe automatically
  // via wsEventsProvider (a StreamProvider that re-reads this getter).
  StreamController<WsEvent> _controller = StreamController<WsEvent>.broadcast();
  Stream<WsEvent> get events => _controller.stream;

  StreamController<WsState> _stateController =
      StreamController<WsState>.broadcast();
  Stream<WsState> get connectionState => _stateController.stream;

  String? _wsBaseUrl;
  String? _classroomId;
  TicketFactory? _ticketFactory;

  bool _disposed = false;

  int _reconnectAttempt = 0;
  static const int _maxReconnectAttempts = 20;

  // C-005: serialize concurrent connect() calls — if one is in flight, await it.
  Future<void>? _connecting;

  Future<void> connect({
    required String wsBaseUrl,
    required String classroomId,
    required TicketFactory ticketFactory,
  }) async {
    _disposed = false;
    // C-005: if already connecting, wait for the in-flight connect to finish.
    // V-4: propagate errors — if the first caller's connect threw, the awaited
    // future completes with an error and this caller rethrows it too.
    if (_connecting != null) {
      await _connecting;
      return;
    }
    // No-op if already connected to the same room with the same base.
    if (_wsBaseUrl == wsBaseUrl && _classroomId == classroomId && _channel != null) {
      return;
    }

    final completer = Completer<void>();
    _connecting = completer.future;
    try {
      _wsBaseUrl = wsBaseUrl;
      _classroomId = classroomId;
      _ticketFactory = ticketFactory;
      _reconnectAttempt = 0;
      _dispose();
      // FA-2: if the controller was closed by a previous close() call, create a
      // fresh one so _connectUrl can add events without hitting a closed sink.
      if (_controller.isClosed) {
        _controller = StreamController<WsEvent>.broadcast();
      }
      if (_stateController.isClosed) {
        _stateController = StreamController<WsState>.broadcast();
      }
      // C-006: always mint a fresh ticket on (re)connect via the factory.
      final ticket = await ticketFactory();
      final url = _buildUrl(wsBaseUrl, classroomId, ticket);
      _connectUrl(url);
      completer.complete();
    } catch (e, st) {
      // V-4: complete with error so concurrent waiters also receive the failure.
      completer.completeError(e, st);
      rethrow;
    } finally {
      _connecting = null;
    }
  }

  static String _buildUrl(
      String wsBaseUrl, String classroomId, String ticket) {
    // V-6: user:<id>:notifications topic is omitted — the backend's
    // authorizeTopics() auto-adds it from the ticket's UserID claim, so
    // sending it explicitly from the client is redundant and leaks the userId
    // into the URL (logged by reverse proxies).
    return '$wsBaseUrl/ws'
        '?topic=classroom:$classroomId:devices'
        '&topic=classroom:$classroomId:sensors'
        '&topic=classroom:$classroomId:scenes'
        '&ticket=$ticket';
  }

  void disconnect() {
    _wsBaseUrl = null;
    _classroomId = null;
    _ticketFactory = null;
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

  void _connectUrl(String url) {
    if (_disposed) return;
    bool connectedOnce = false;
    try {
      _channel = WebSocketChannel.connect(Uri.parse(url));
      _sub = _channel!.stream.listen(
        (data) {
          // Reset backoff counter on first successful frame received.
          if (!connectedOnce) {
            connectedOnce = true;
            _reconnectAttempt = 0;
            if (!_stateController.isClosed) {
              _stateController.add(WsState.connected);
            }
          }
          // B-007: guard against binary frames — only handle String messages
          if (data is! String) return;
          try {
            final decoded = jsonDecode(data) as Map<String, dynamic>;
            _controller.add(WsEvent.fromJson(decoded));
          } catch (_) {
            // ignore malformed messages
          }
        },
        onError: (_) => _scheduleReconnect(),
        onDone: () => _scheduleReconnect(),
      );
    } catch (_) {
      _scheduleReconnect();
    }
  }

  // C-006: on reconnect, call ticketFactory() to mint a fresh ticket.
  // M-001: exponential backoff — 1s, 2s, 4s … capped at 60s; stops after 20 attempts.
  void _scheduleReconnect() {
    _sub?.cancel();
    _sub = null;
    _channel?.sink.close();
    _channel = null;
    _reconnectTimer?.cancel();

    if (_reconnectAttempt >= _maxReconnectAttempts) {
      // Give up — emit failed so UI can react via connectionState stream.
      if (!_stateController.isClosed) {
        _stateController.add(WsState.failed);
      }
      return;
    }
    if (!_stateController.isClosed) {
      _stateController.add(WsState.reconnecting);
    }

    final delaySecs = (1 << _reconnectAttempt).clamp(1, 60);
    _reconnectAttempt++;

    _reconnectTimer = Timer(Duration(seconds: delaySecs), () async {
      final factory = _ticketFactory;
      final base = _wsBaseUrl;
      final room = _classroomId;
      if (factory == null || base == null || room == null) return;
      try {
        final ticket = await factory();
        final url = _buildUrl(base, room, ticket);
        if (_wsBaseUrl == base && _classroomId == room) {
          _connectUrl(url);
        }
      } catch (_) {
        // Factory failed (e.g. offline); retry next cycle.
        _scheduleReconnect();
      }
    });
  }

  /// Stops the socket and reconnect timers without closing the StreamController.
  /// After close(), connect() can be called again safely (FA-2).
  void close() {
    _disposed = true;
    _dispose();
    // Do NOT close _controller here — it would permanently break the stream.
    // Use dispose() for final app teardown.
  }

  /// Resets ALL mutable fields to their initial state.
  /// Must only be called from tests — never from production code.
  @visibleForTesting
  void resetForTest() {
    _disposed = false;
    _wsBaseUrl = null;
    _classroomId = null;
    _ticketFactory = null;
    _reconnectAttempt = 0;
    _connecting = null;
    _reconnectTimer?.cancel();
    _reconnectTimer = null;
    _sub?.cancel();
    _sub = null;
    _channel?.sink.close();
    _channel = null;
    if (!_controller.isClosed) {
      _controller.close();
    }
    _controller = StreamController<WsEvent>.broadcast();
    if (!_stateController.isClosed) {
      _stateController.close();
    }
    _stateController = StreamController<WsState>.broadcast();
  }

  /// Final teardown — closes the StreamController permanently.
  /// Should only be called when the app is shutting down.
  void dispose() {
    _disposed = true;
    _dispose();
    if (!_controller.isClosed) {
      _controller.close();
    }
    if (!_stateController.isClosed) {
      _stateController.close();
    }
  }
}
