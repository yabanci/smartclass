import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../shared/models/lesson.dart';
import '../../shared/providers/classroom_provider.dart';
import '../../shared/providers/schedule_provider.dart';
import '../../shared/widgets/error_view.dart';
import '../../shared/widgets/loading_indicator.dart';
import 'add_lesson_sheet.dart';

class SchedulePage extends ConsumerWidget {
  const SchedulePage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final classroom = ref.watch(activeClassroomProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Schedule')),
      body: classroom == null
          ? const Center(child: Text('Select a classroom first'))
          : _WeekView(classroomId: classroom.id),
      floatingActionButton: classroom == null
          ? null
          : FloatingActionButton(
              onPressed: () => showModalBottomSheet(
                context: context,
                isScrollControlled: true,
                builder: (_) => AddLessonSheet(classroomId: classroom.id),
              ),
              child: const Icon(Icons.add),
            ),
    );
  }
}

const _dayNames = {
  1: 'Monday',
  2: 'Tuesday',
  3: 'Wednesday',
  4: 'Thursday',
  5: 'Friday',
  6: 'Saturday',
  7: 'Sunday',
};

class _WeekView extends ConsumerWidget {
  final String classroomId;

  const _WeekView({required this.classroomId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final scheduleAsync = ref.watch(scheduleProvider(classroomId));

    return scheduleAsync.when(
      loading: () => const LoadingIndicator(),
      error: (e, _) => ErrorView(
        message: e.toString(),
        onRetry: () =>
            ref.read(scheduleProvider(classroomId).notifier).load(),
      ),
      data: (schedule) {
        final now = DateTime.now();
        final todayKey = now.weekday.toString();

        return RefreshIndicator(
          onRefresh: () =>
              ref.read(scheduleProvider(classroomId).notifier).load(),
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              for (int day = 1; day <= 7; day++)
                if (schedule.containsKey(day.toString()) &&
                    schedule[day.toString()]!.isNotEmpty)
                  _DaySection(
                    day: day,
                    lessons: schedule[day.toString()] ?? [],
                    isToday: day.toString() == todayKey,
                    classroomId: classroomId,
                  ),
              if (schedule.values.every((l) => l.isEmpty))
                const Center(
                  child: Padding(
                    padding: EdgeInsets.all(32),
                    child: Text('No lessons scheduled',
                        style: TextStyle(color: Colors.grey)),
                  ),
                ),
            ],
          ),
        );
      },
    );
  }
}

class _DaySection extends ConsumerWidget {
  final int day;
  final List<Lesson> lessons;
  final bool isToday;
  final String classroomId;

  const _DaySection({
    required this.day,
    required this.lessons,
    required this.isToday,
    required this.classroomId,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.symmetric(vertical: 8),
          child: Row(
            children: [
              Text(
                _dayNames[day] ?? 'Day $day',
                style: theme.textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: isToday ? theme.colorScheme.primary : null,
                ),
              ),
              if (isToday) ...[
                const SizedBox(width: 8),
                Container(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                  decoration: BoxDecoration(
                    color: theme.colorScheme.primaryContainer,
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Text('Today',
                      style: TextStyle(
                          fontSize: 11,
                          color: theme.colorScheme.onPrimaryContainer)),
                ),
              ],
            ],
          ),
        ),
        ...lessons.map((lesson) => _LessonTile(
              lesson: lesson,
              classroomId: classroomId,
            )),
        const SizedBox(height: 8),
      ],
    );
  }
}

class _LessonTile extends ConsumerWidget {
  final Lesson lesson;
  final String classroomId;

  const _LessonTile({required this.lesson, required this.classroomId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Card(
      elevation: 0,
      color: Theme.of(context).colorScheme.surfaceContainerHighest,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      margin: const EdgeInsets.only(bottom: 6),
      child: ListTile(
        leading: const Icon(Icons.book_outlined),
        title: Text(lesson.subject),
        subtitle: Text('${lesson.startsAt} – ${lesson.endsAt}'),
        trailing: IconButton(
          icon: Icon(Icons.delete_outlined,
              color: Theme.of(context).colorScheme.error),
          onPressed: () => _confirmDelete(context, ref),
        ),
      ),
    );
  }

  Future<void> _confirmDelete(BuildContext context, WidgetRef ref) async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Delete lesson?'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Cancel')),
          FilledButton(
              style: FilledButton.styleFrom(
                  backgroundColor: Theme.of(ctx).colorScheme.error),
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('Delete')),
        ],
      ),
    );
    if (confirm == true) {
      await ref
          .read(scheduleProvider(classroomId).notifier)
          .deleteLesson(lesson.id);
    }
  }
}
