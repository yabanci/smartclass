import '../../core/utils/error_utils.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app.dart';
import '../../core/i18n/app_localizations.dart';
import '../../shared/models/classroom.dart';
import '../../shared/providers/classroom_provider.dart';
import '../../shared/providers/device_provider.dart';
import '../../shared/providers/schedule_provider.dart';
import '../../shared/providers/sensor_provider.dart';
import '../../shared/providers/ws_provider.dart';
import '../../shared/widgets/app_card.dart';
import '../../shared/widgets/classroom_picker.dart';

class HomePage extends ConsumerWidget {
  const HomePage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context)!;
    final classroomsAsync = ref.watch(classroomListProvider);
    final classroom = ref.watch(activeClassroomProvider);

    // Auto-select first classroom as soon as list loads
    ref.listen(classroomListProvider, (_, next) {
      next.whenData((classrooms) {
        if (classrooms.isNotEmpty && ref.read(activeClassroomProvider) == null) {
          ref.read(activeClassroomProvider.notifier).select(classrooms.first);
        }
      });
    });

    // Connect WebSocket when classroom CHANGES (not on every rebuild)
    ref.listen(activeClassroomProvider, (prev, next) {
      if (next != null && next.id != prev?.id) {
        ref.read(wsConnectionProvider.notifier).connectToClassroom(next.id);
      } else if (next == null) {
        ref.read(wsConnectionProvider.notifier).disconnect();
      }
    });

    // React to real-time events
    if (classroom != null) {
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
        title: Text(l.homeTitle,
            style: const TextStyle(fontWeight: FontWeight.w800)),
        actions: [
          // Connection mode chip
          Padding(
            padding: const EdgeInsets.only(right: 4),
            child: ActionChip(
              label: Text(l.commonOnline, style: const TextStyle(fontSize: 11)),
              avatar: const Icon(Icons.cloud_outlined, size: 14),
              padding: EdgeInsets.zero,
              visualDensity: VisualDensity.compact,
              onPressed: () {},
            ),
          ),
          IconButton(
            icon: const Icon(Icons.notifications_outlined),
            onPressed: () => context.push('/notifications'),
          ),
        ],
      ),
      body: Column(
        children: [
          // Classroom selector bar
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
            decoration: BoxDecoration(
              color: Theme.of(context).scaffoldBackgroundColor,
              border: Border(
                bottom: BorderSide(
                  color: kPrimary.withOpacity(0.08),
                ),
              ),
            ),
            child: Row(
              children: [
                Expanded(
                  child: classroomsAsync.when(
                    loading: () => const LinearProgressIndicator(),
                    error: (e, _) => Text('Error: $e',
                        style: const TextStyle(color: kDanger, fontSize: 12)),
                    data: (_) => const ClassroomPicker(),
                  ),
                ),
                IconButton(
                  icon: const Icon(Icons.add_circle_outline, color: kPrimary),
                  tooltip: l.homeCreateClassroom,
                  onPressed: () => _showCreateDialog(context, ref),
                ),
              ],
            ),
          ),

          Expanded(
            child: classroom == null
                ? _EmptyState(onCreateTap: () => _showCreateDialog(context, ref))
                : _ClassroomBody(classroom: classroom),
          ),
        ],
      ),
    );
  }

  Future<void> _showCreateDialog(BuildContext context, WidgetRef ref) async {
    final l = AppLocalizations.of(context)!;
    final ctrl = TextEditingController();
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l.homeCreateClassroom),
        content: TextField(
          controller: ctrl,
          decoration: InputDecoration(
            labelText: l.homeClassroomName,
          ),
          autofocus: true,
          onSubmitted: (_) => Navigator.pop(ctx, true),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: Text(l.commonCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: Text(l.commonCreate),
          ),
        ],
      ),
    );
    if (confirmed == true && ctrl.text.trim().isNotEmpty && context.mounted) {
      try {
        final classroom = await ref
            .read(classroomListProvider.notifier)
            .create(ctrl.text.trim());
        ref.read(activeClassroomProvider.notifier).select(classroom);
      } catch (e) {
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text(friendlyError(e)),
              backgroundColor: kDanger,
            ),
          );
        }
      }
    }
  }
}

class _EmptyState extends StatelessWidget {
  final VoidCallback onCreateTap;
  const _EmptyState({required this.onCreateTap});

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context)!;
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.meeting_room_outlined, size: 72,
              color: kPrimary.withOpacity(0.3)),
          const SizedBox(height: 16),
          Text(l.homeNoClassroom,
              style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600)),
          const SizedBox(height: 8),
          Text(l.homeCreateClassroom,
              style: TextStyle(fontSize: 13, color: Colors.grey.shade500)),
          const SizedBox(height: 24),
          FilledButton.icon(
            onPressed: onCreateTap,
            icon: const Icon(Icons.add),
            label: Text(l.homeCreateClassroom),
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
    final l = AppLocalizations.of(context)!;
    final devicesAsync = ref.watch(deviceListProvider(classroom.id));
    final currentLessonAsync = ref.watch(currentLessonProvider(classroom.id));
    final sensorState = ref.watch(sensorNotifierProvider(classroom.id));

    return RefreshIndicator(
      onRefresh: () async {
        ref.read(deviceListProvider(classroom.id).notifier).load();
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
                      Row(
                        children: [
                          Container(
                            width: 32,
                            height: 32,
                            decoration: BoxDecoration(
                              color: kPrimary.withOpacity(0.1),
                              borderRadius: BorderRadius.circular(8),
                            ),
                            child: const Icon(Icons.devices,
                                color: kPrimary, size: 18),
                          ),
                          const SizedBox(width: 8),
                          Text(l.homeActiveDevices,
                              style: const TextStyle(
                                  fontSize: 12, color: Colors.grey)),
                        ],
                      ),
                      const SizedBox(height: 8),
                      devicesAsync.when(
                        data: (d) => Text(
                          '${d.where((x) => x.online).length}/${d.length}',
                          style: const TextStyle(
                              fontSize: 24, fontWeight: FontWeight.w800),
                        ),
                        loading: () => const SizedBox(
                            width: 20, height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2)),
                        error: (_, __) => const Text('—'),
                      ),
                      if (devicesAsync.valueOrNull != null)
                        Text(
                          '● ${devicesAsync.valueOrNull!.where((x) => x.isOn).length} on',
                          style: const TextStyle(fontSize: 12, color: kAccent),
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
                      Row(
                        children: [
                          Container(
                            width: 32,
                            height: 32,
                            decoration: BoxDecoration(
                              color: kAccent.withOpacity(0.1),
                              borderRadius: BorderRadius.circular(8),
                            ),
                            child: const Icon(Icons.bolt,
                                color: kAccent, size: 18),
                          ),
                          const SizedBox(width: 8),
                          Text(l.analyticsEnergy,
                              style: const TextStyle(
                                  fontSize: 12, color: Colors.grey)),
                        ],
                      ),
                      const SizedBox(height: 8),
                      devicesAsync.when(
                        data: (d) {
                          final on = d.where((x) => x.isOn).length;
                          return Text(
                            '${(on * 0.2).toStringAsFixed(1)} kW',
                            style: const TextStyle(
                                fontSize: 24, fontWeight: FontWeight.w800),
                          );
                        },
                        loading: () => const SizedBox(
                            width: 20, height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2)),
                        error: (_, __) => const Text('—'),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),

          // Sensor row
          Row(
            children: [
              Expanded(
                child: _SensorCard(
                  icon: Icons.thermostat,
                  label: l.homeTemperature,
                  value: sensorState.readings
                      .where((r) => r.metric == 'temperature')
                      .fold<double?>(null, (_, r) => r.value),
                  unit: '°C',
                  color: Colors.orange,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _SensorCard(
                  icon: Icons.water_drop_outlined,
                  label: l.homeHumidity,
                  value: sensorState.readings
                      .where((r) => r.metric == 'humidity')
                      .fold<double?>(null, (_, r) => r.value),
                  unit: '%',
                  color: Colors.blue,
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),

          // Quick controls
          devicesAsync.when(
            data: (devices) => devices.isEmpty
                ? const SizedBox.shrink()
                : AppCard(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(l.devicesQuickControls,
                            style: const TextStyle(
                                fontSize: 12, color: Colors.grey)),
                        const SizedBox(height: 12),
                        Row(
                          children: [
                            Expanded(
                              child: _QuickBtn(
                                label: l.devicesAllOn,
                                color: kAccent,
                                classroomId: classroom.id,
                                command: 'ON',
                                devices: devices,
                              ),
                            ),
                            const SizedBox(width: 8),
                            Expanded(
                              child: _QuickBtn(
                                label: l.devicesAllOff,
                                color: Colors.grey,
                                classroomId: classroom.id,
                                command: 'OFF',
                                devices: devices,
                              ),
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
            loading: () => const SizedBox.shrink(),
            error: (_, __) => const SizedBox.shrink(),
          ),
          const SizedBox(height: 12),

          // Current lesson
          AppCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Icon(Icons.calendar_today,
                        color: kPrimary, size: 16),
                    const SizedBox(width: 8),
                    Text(l.homeCurrentLesson,
                        style: const TextStyle(
                            fontWeight: FontWeight.w700)),
                  ],
                ),
                const SizedBox(height: 12),
                currentLessonAsync.when(
                  data: (lesson) => lesson != null
                      ? Container(
                          padding: const EdgeInsets.all(12),
                          decoration: BoxDecoration(
                            color: kAccent.withOpacity(0.08),
                            borderRadius: BorderRadius.circular(12),
                            border: Border(
                              left: BorderSide(
                                  color: kAccent, width: 4),
                            ),
                          ),
                          child: Column(
                            crossAxisAlignment:
                                CrossAxisAlignment.start,
                            children: [
                              Text(
                                lesson.subject,
                                style: const TextStyle(
                                    fontWeight: FontWeight.w700,
                                    fontSize: 15),
                              ),
                              const SizedBox(height: 4),
                              Text(
                                '${lesson.startsAt} – ${lesson.endsAt}',
                                style: const TextStyle(
                                    fontSize: 12,
                                    color: Colors.grey),
                              ),
                            ],
                          ),
                        )
                      : Text(
                          l.homeNoLesson,
                          style: TextStyle(color: Colors.grey.shade500),
                        ),
                  loading: () =>
                      const CircularProgressIndicator(strokeWidth: 2),
                  error: (_, __) => Text(l.homeNoLesson,
                      style: TextStyle(color: Colors.grey.shade500)),
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
          const SizedBox(width: 10),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(label,
                  style: const TextStyle(
                      fontSize: 11, color: Colors.grey)),
              Text(
                value != null
                    ? '${value!.toStringAsFixed(1)}$unit'
                    : '—',
                style: const TextStyle(
                    fontSize: 20, fontWeight: FontWeight.w800),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _QuickBtn extends ConsumerWidget {
  final String label;
  final Color color;
  final String classroomId;
  final String command;
  final List devices;

  const _QuickBtn({
    required this.label,
    required this.color,
    required this.classroomId,
    required this.command,
    required this.devices,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return GestureDetector(
      onTap: () {
        for (final d in devices) {
          ref
              .read(deviceListProvider(classroomId).notifier)
              .sendCommand(d.id, command);
        }
      },
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: 12),
        decoration: BoxDecoration(
          color: color.withOpacity(0.12),
          borderRadius: BorderRadius.circular(12),
        ),
        child: Center(
          child: Text(
            label,
            style: TextStyle(
              color: color == Colors.grey ? Colors.grey.shade700 : color,
              fontWeight: FontWeight.w700,
              fontSize: 13,
            ),
          ),
        ),
      ),
    );
  }
}
