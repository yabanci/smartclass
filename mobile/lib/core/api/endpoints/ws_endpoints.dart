import '../client.dart';

/// WsEndpoints exposes WebSocket-related HTTP endpoints. The single-use
/// ticket flow lives here: the caller fetches a ticket immediately before
/// each WS upgrade attempt (including reconnects), then includes it as a
/// `?ticket=` query param on the upgrade URL.
///
/// The ticket is single-use and 60s-lived; never cache it across upgrades.
class WsEndpoints {
  WsEndpoints(this._client);

  final ApiClient _client;

  /// Issues a fresh single-use ticket for the next WebSocket upgrade.
  /// Throws on failure — the caller surfaces it as a connection error.
  Future<String> createTicket() => _client.unwrap<String>(
        _client.post('/ws/ticket'),
        (d) => (d as Map<String, dynamic>)['ticket'] as String,
      );
}
