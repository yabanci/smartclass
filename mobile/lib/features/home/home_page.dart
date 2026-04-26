import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/connection/connection_mode.dart';
import '../../core/connection/resolver.dart';
import '../../shared/models/classroom.dart';
import '../../shared/providers/auth_provider.dart';
import '../../shared/providers/classroom_provider.dart';
import '../../shared/providers/device_provider.dart';
import '../../shared/providers/schedule_provider.dart';
import '../../shared/providers/sensor_provider.dart';
import '../../shared/providers/ws_provider.dart';
import '../../shared/widgets/app_card.dart';
import '../../shared/widgets/classroom_picker.dart';
import '../../shared/widgets/loading_indicator.dart';

class HomePage extends ConsumerStatefulWidget {
  const HomePage({super.key});

  @override
  ConsumerState<HomePage> createState() => _HomePageState();
}

class _HomePageState extends ConsumerState<HomePage> {
  @override
  void initState() {
    super.initState();
    // Load classrooms and auto-select first one
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      final classroomsAsync = ref.read(classroomListProvider);
      classroomsAsync.whenData((classrooms) {
        if (classrooms.isNotEmpty && ref.read(activeClassroomProvider) == null) {
          ref.read(activeClassroomProvider.notifier).select(classrooms.first);
        }
      });
    });
  }

  @override
  Widget build(BuildContext context) {
    final classroom = ref.watch(activeClassroomProvider);
    final connectionMode = ConnectionResolver.instance.current.mode;

    // Connect WebSocket when classroom is selected
    if (classroom != null) {
      ref.read(wsConnectionProvider.notifier).connectToClassroom(classroom.id);

      // Listen to WS events
      ref.listen(wsEventsProvider, (_, next) {
        next.whenData((event) {
          if (event.type.startsWith('device.')) {
            ref.read(deviceListProvider(classroom.id).notifier).load();
          } else if (event.type == 'sensor.reading') {
            ref.read(sensorNotifierProvider(classroom.id).notifier).load();
          }
        });
      });
    }

    return Scaffold(
      appBar: AppBar(
        title: const Text('Smart Classroom'),
        actions: [
          // Connection mode chip
          Padding(
            padding: const EdgeInsets.only(right: 8),
            child: Chip(
              label: Text(
                connectionMode == ConnectionMode.local ? 'Local' : 'Remote',
                style: const TextStyle(fontSize: 11),
              ),
              avatar: Icon(
                connectionMode == ConnectionMode.local
                    ? Icons.home_outlined
                    : Icons.cloud_outlined,
                size: 14,
              ),
              padding: EdgeInsets.zero,
              visualDensity: VisualDensity.compact,
            ),
          ),
          IconButton(
            icon: const Icon(Icons.notifications_outlined),
            onPressed: () => context.push('/notifications'),
          ),
        ],
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(48),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
            child: Row(
              children: [
                Expanded(child: ClassroomPicker()),
                IconButton(
                  icon: const Icon(Icons.add),
                  tooltip: 'New classroom',
                  onPressed: () => _showCreateDialog(context),
                ),
              ],
            ),
          ),
        ),
      ),
      body: classroom == null
          ? _EmptyState(onCreateTap: () => _showCreateDialog(context))
          : _ClassroomBody(classroom: classroom),
    );
  }

  Future<void> _showCreateDialog(BuildContext context) async {
    final ctrl = TextEditingController();
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('New classroom'),
        content: TextField(
          controller: ctrl,
          decoration: const InputDecoration(
            labelText: 'Classroom name',
            border: OutlineInputBorder(),
          ),
          autofocus: true,
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Create'),
          ),
        ],
      ),
    );
    if (confirmed == true && ctrl.text.isNotEmpty) {
      final classroom =
          await ref.read(classroomListProvider.notifier).create(ctrl.text);
      if (classroom != null) {
        ref.read(activeClassroomProvider.notifier).select(classroom);
      }
    }
  }
}

class _EmptyState extends StatelessWidget {
  final VoidCallback onCreateTap;

  const _EmptyState({required this.onCreateTap});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.meeting_room_outlined, size: 64, color: Colors.grey),
          const SizedBox(height: 16),
          const Text('Create your first classroom',
              style: TextStyle(fontSize: 16)),
          const SizedBox(height: 16),
          FilledButton.icon(
            onPressed: onCreateTap,
            icon: const Icon(Icons.add),
            label: const Text('New classroom'),
          ),
        ],
      ),
    );
  }
}

class _ClassroomBody extends ConsumerWidget {
  final Classroom classroom;

  const _ClassroomBody({required this.classroom});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final devicesAsync = ref.watch(deviceListProvider(classroom.id));
    final currentLessonAsync = ref.watch(currentLessonProvider(classroom.id));
    final sensorState = ref.watch(sensorNotifierProvider(classroom.id));

    return RefreshIndicator(
      onRefresh: () async {
        ref.invalidate(deviceListProvider(classroom.id));
        ref.invalidate(currentLessonProvider(classroom.id));
        ref.read(sensorNotifierProvider(classroom.id).notifier).load();
      },
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          // Stats row
          Row(
            children: [
              Expanded(
                child: AppCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Text('Active devices',
                          style: TextStyle(fontSize: 12, color: Colors.grey)),
                      const SizedBox(height: 4),
                      devicesAsync.when(
                        data: (devices) => Text(
                          '${devices.where((d) => d.online).length}/${devices.length}',
                          style: const TextStyle(
                              fontSize: 24, fontWeight: FontWeight.bold),
                        ),
                        loading: () =>
                            const CircularProgressIndicator(strokeWidth: 2),
                        error: (_, __) => const Text('—'),
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: AppCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Text('Devices ON',
                          style: TextStyle(fontSize: 12, color: Colors.grey)),
                      const SizedBox(height: 4),
                      devicesAsync.when(
                        data: (devices) => Text(
                          '${devices.where((d) => d.isOn).length}',
                          style: const TextStyle(
                              fontSize: 24, fontWeight: FontWeight.bold),
                        ),
                        loading: () =>
                            const CircularProgressIndicator(strokeWidth: 2),
                        error: (_, __) => const Text('—'),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),

          // Sensors row
          Row(
            children: [
              Expanded(
                child: _SensorCard(
                  icon: Icons.thermostat,
                  label: 'Temperature',
                  value: sensorState.readings
                      .where((r) => r.metric == 'temperature')
                      .fold<double?>(
                        null,
                        (_, r) => r.value,
                      ),
                  unit: '°C',
                  color: Colors.orange,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _SensorCard(
                  icon: Icons.water_drop_outlined,
                  label: 'Humidity',
                  value: sensorState.readings
                      .where((r) => r.metric == 'humidity')
                      .fold<double?>(
                        null,
                        (_, r) => r.value,
                      ),
                  unit: '%',
                  color: Colors.blue,
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),

          // Current lesson
          AppCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text('Current lesson',
                    style: TextStyle(fontSize: 12, color: Colors.grey)),
                const SizedBox(height: 8),
                currentLessonAsync.when(
                  data: (lesson) => lesson != null
                      ? Row(
                          children: [
                            const Icon(Icons.book_outlined, size: 20),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(lesson.subject,
                                      style: const TextStyle(
                                          fontWeight: FontWeight.bold)),
                                  Text(
                                      '${lesson.startsAt} – ${lesson.endsAt}',
                                      style: const TextStyle(
                                          fontSize: 12,
                                          color: Colors.grey)),
                                ],
                              ),
                            ),
                          ],
                        )
                      : const Text('No lesson in progress',
                          style: TextStyle(color: Colors.grey)),
                  loading: () =>
                      const CircularProgressIndicator(strokeWidth: 2),
                  error: (_, __) =>
                      const Text('No lesson in progress',
                          style: TextStyle(color: Colors.grey)),
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),

          // Quick controls
          AppCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text('Quick controls',
                    style: TextStyle(fontSize: 12, color: Colors.grey)),
                const SizedBox(height: 12),
                Row(
                  children: [
                    Expanded(
                      child: _QuickControlButton(
                        label: 'All ON',
                        icon: Icons.power,
                        color: Colors.green,
                        classroomId: classroom.id,
                        command: 'ON',
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: _QuickControlButton(
                        label: 'All OFF',
                        icon: Icons.power_off,
                        color: Colors.red,
                        classroomId: classroom.id,
                        command: 'OFF',
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _SensorCard extends StatelessWidget {
  final IconData icon;
  final String label;
  final double? value;
  final String unit;
  final Color color;

  const _SensorCard({
    required this.icon,
    required this.label,
    required this.value,
    required this.unit,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return AppCard(
      child: Row(
        children: [
          Icon(icon, color: color, size: 28),
          const SizedBox(width: 8),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(label,
                  style: const TextStyle(fontSize: 12, color: Colors.grey)),
              Text(
                value != null
                    ? '${value!.toStringAsFixed(1)}$unit'
                    : '—',
                style: const TextStyle(
                    fontSize: 18, fontWeight: FontWeight.bold),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _QuickControlButton extends ConsumerWidget {
  final String label;
  final IconData icon;
  final Color color;
  final String classroomId;
  final String command;

  const _QuickControlButton({
    required this.label,
    required this.icon,
    required this.color,
    required this.classroomId,
    required this.command,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return ElevatedButton.icon(
      style: ElevatedButton.styleFrom(
        backgroundColor: color.withOpacity(0.1),
        foregroundColor: color,
      ),
      icon: Icon(icon, size: 18),
      label: Text(label),
      onPressed: () async {
        final devicesAsync = ref.read(deviceListProvider(classroomId));
        devicesAsync.whenData((devices) {
          for (final device in devices) {
            ref
                .read(deviceListProvider(classroomId).notifier)
                .sendCommand(device.id, command);
          }
        });
      },
    );
  }
}
