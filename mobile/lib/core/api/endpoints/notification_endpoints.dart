import '../client.dart';
import '../../../shared/models/notification.dart';

class NotificationEndpoints {
  final ApiClient _client;
  NotificationEndpoints(this._client);

  Future<List<AppNotification>> list({bool? unread, int? limit}) =>
      _client.unwrap(
        _client.get('/notifications', queryParameters: {
          if (unread != null) 'unread': unread.toString(),
          if (limit != null) 'limit': limit.toString(),
        }),
        (d) => (d as List)
            .map((e) => AppNotification.fromJson(e as Map<String, dynamic>))
            .toList(),
      );

  Future<int> unreadCount() => _client.unwrap(
        _client.get('/notifications/unread-count'),
        (d) => (d as Map<String, dynamic>)['count'] as int,
      );

  Future<void> markRead(String id) => _client.unwrap(
        _client.post('/notifications/$id/read'),
        (_) => null,
      );

  Future<void> markAllRead() => _client.unwrap(
        _client.post('/notifications/read-all'),
        (_) => null,
      );
}
