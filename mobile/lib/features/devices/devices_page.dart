import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../shared/providers/classroom_provider.dart';
import '../../shared/providers/device_provider.dart';
import '../../shared/widgets/error_view.dart';
import '../../shared/widgets/loading_indicator.dart';
import 'device_card.dart';
import 'device_form.dart';

class DevicesPage extends ConsumerWidget {
  const DevicesPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final classroom = ref.watch(activeClassroomProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text(classroom != null ? classroom.name : 'Devices'),
        actions: [
          TextButton.icon(
            icon: const Icon(Icons.wifi_find),
            label: const Text('Find IoT'),
            onPressed: () => context.push('/devices/iot-wizard'),
          ),
        ],
      ),
      body: classroom == null
          ? const Center(child: Text('Select a classroom first'))
          : _DeviceList(classroomId: classroom.id),
      floatingActionButton: classroom == null
          ? null
          : FloatingActionButton(
              onPressed: () => _showAddForm(context, ref, classroom.id),
              child: const Icon(Icons.add),
            ),
    );
  }

  Future<void> _showAddForm(
    BuildContext context,
    WidgetRef ref,
    String classroomId,
  ) async {
    await showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (_) => DeviceFormSheet(classroomId: classroomId),
    );
  }
}

class _DeviceList extends ConsumerWidget {
  final String classroomId;

  const _DeviceList({required this.classroomId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final devicesAsync = ref.watch(deviceListProvider(classroomId));

    return devicesAsync.when(
      loading: () => const LoadingIndicator(),
      error: (e, _) => ErrorView(
        message: e.toString(),
        onRetry: () =>
            ref.read(deviceListProvider(classroomId).notifier).load(),
        retryLabel: 'Retry',
      ),
      data: (devices) {
        if (devices.isEmpty) {
          return Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.devices_other, size: 64, color: Colors.grey),
                const SizedBox(height: 16),
                const Text('No devices yet',
                    style: TextStyle(fontSize: 16, color: Colors.grey)),
                const SizedBox(height: 16),
                OutlinedButton.icon(
                  icon: const Icon(Icons.wifi_find),
                  label: const Text('Find IoT devices'),
                  onPressed: () => context.push('/devices/iot-wizard'),
                ),
              ],
            ),
          );
        }
        return RefreshIndicator(
          onRefresh: () =>
              ref.read(deviceListProvider(classroomId).notifier).load(),
          child: ListView.separated(
            padding: const EdgeInsets.all(16),
            itemCount: devices.length,
            separatorBuilder: (_, __) => const SizedBox(height: 8),
            itemBuilder: (context, index) {
              final device = devices[index];
              return DeviceCard(
                device: device,
                classroomId: classroomId,
                onDelete: () => _confirmDelete(context, ref, device.id),
              );
            },
          ),
        );
      },
    );
  }

  Future<void> _confirmDelete(
    BuildContext context,
    WidgetRef ref,
    String deviceId,
  ) async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Delete device?'),
        content: const Text('This action cannot be undone.'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            style: FilledButton.styleFrom(
                backgroundColor: Theme.of(ctx).colorScheme.error),
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
    if (confirm == true) {
      await ref
          .read(deviceListProvider(classroomId).notifier)
          .delete(deviceId);
    }
  }
}
