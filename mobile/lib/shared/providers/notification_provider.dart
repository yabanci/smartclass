import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/endpoints/notification_endpoints.dart';
import '../models/notification.dart';
import 'auth_provider.dart';

final notificationEndpointsProvider = Provider<NotificationEndpoints>(
  (ref) => NotificationEndpoints(ref.watch(apiClientProvider)),
);

class NotificationListNotifier
    extends StateNotifier<AsyncValue<List<AppNotification>>> {
  final NotificationEndpoints _endpoints;

  NotificationListNotifier(this._endpoints)
      : super(const AsyncValue.loading()) {
    load();
  }

  Future<void> load() async {
    state = const AsyncValue.loading();
    try {
      final list = await _endpoints.list(limit: 50);
      state = AsyncValue.data(list);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
    }
  }

  Future<void> markRead(String id) async {
    await _endpoints.markRead(id);
    state.whenData((notifications) {
      state = AsyncValue.data([
        for (final n in notifications)
          if (n.id == id)
            AppNotification(
              id: n.id,
              userId: n.userId,
              classroomId: n.classroomId,
              type: n.type,
              title: n.title,
              message: n.message,
              metadata: n.metadata,
              readAt: DateTime.now().toIso8601String(),
              createdAt: n.createdAt,
            )
          else
            n
      ]);
    });
  }

  Future<void> markAllRead() async {
    await _endpoints.markAllRead();
    await load(); // B-301: must await so callers can chain on completion
  }
}

final notificationListProvider = StateNotifierProvider<
    NotificationListNotifier, AsyncValue<List<AppNotification>>>((ref) {
  return NotificationListNotifier(ref.watch(notificationEndpointsProvider));
});

// B-112: derive unread count from the already-loaded list instead of a separate
// network call on every rebuild, so no extra request is made.
final unreadCountProvider = Provider<int>((ref) {
  final listState = ref.watch(notificationListProvider);
  return listState.whenOrNull(
        data: (notifications) =>
            notifications.where((n) => n.readAt == null).length,
      ) ??
      0;
});
