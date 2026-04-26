import '../../core/utils/error_utils.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/i18n/app_localizations.dart';
import '../../shared/providers/notification_provider.dart';
import '../../shared/widgets/error_view.dart';
import '../../shared/widgets/loading_indicator.dart';

class NotificationsPage extends ConsumerWidget {
  const NotificationsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final notificationsAsync = ref.watch(notificationListProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text(l.notificationsTitle),
        actions: [
          TextButton(
            onPressed: () => ref
                .read(notificationListProvider.notifier)
                .markAllRead(),
            child: Text(l.notificationsMarkAllRead),
          ),
        ],
      ),
      body: notificationsAsync.when(
        loading: () => const LoadingIndicator(),
        error: (e, _) => ErrorView(
          message: friendlyError(e),
          onRetry: () =>
              ref.read(notificationListProvider.notifier).load(),
        ),
        data: (notifications) {
          if (notifications.isEmpty) {
            return Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(Icons.notifications_off_outlined,
                      size: 64, color: Colors.grey),
                  const SizedBox(height: 16),
                  Text(l.notificationsEmpty,
                      style: const TextStyle(color: Colors.grey)),
                ],
              ),
            );
          }
          return RefreshIndicator(
            onRefresh: () =>
                ref.read(notificationListProvider.notifier).load(),
            child: ListView.separated(
              padding: const EdgeInsets.symmetric(vertical: 8),
              itemCount: notifications.length,
              separatorBuilder: (_, __) => const Divider(height: 1),
              itemBuilder: (context, i) {
                final n = notifications[i];
                return ListTile(
                  leading: _typeIcon(n.type),
                  title: Text(n.title,
                      style: TextStyle(
                          fontWeight: n.isRead
                              ? FontWeight.normal
                              : FontWeight.bold)),
                  subtitle: Text(n.message,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis),
                  trailing: n.isRead
                      ? null
                      : IconButton(
                          icon: const Icon(Icons.check, size: 18),
                          onPressed: () => ref
                              .read(notificationListProvider.notifier)
                              .markRead(n.id),
                        ),
                  tileColor: n.isRead
                      ? null
                      : Theme.of(context)
                          .colorScheme
                          .primaryContainer
                          .withValues(alpha: 0.2),
                );
              },
            ),
          );
        },
      ),
    );
  }

  Widget _typeIcon(String type) {
    switch (type) {
      case 'error':
        return const CircleAvatar(
          backgroundColor: Colors.red,
          radius: 16,
          child: Icon(Icons.error_outline, color: Colors.white, size: 16),
        );
      case 'warning':
        return const CircleAvatar(
          backgroundColor: Colors.orange,
          radius: 16,
          child: Icon(Icons.warning_amber_outlined,
              color: Colors.white, size: 16),
        );
      default:
        return const CircleAvatar(
          backgroundColor: Colors.blue,
          radius: 16,
          child:
              Icon(Icons.info_outline, color: Colors.white, size: 16),
        );
    }
  }
}
