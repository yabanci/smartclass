import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/classroom.dart';
import '../providers/classroom_provider.dart';

class ClassroomPicker extends ConsumerWidget {
  const ClassroomPicker({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final classroomsAsync = ref.watch(classroomListProvider);
    final active = ref.watch(activeClassroomProvider);

    return classroomsAsync.when(
      loading: () => const SizedBox(
        width: 24,
        height: 24,
        child: CircularProgressIndicator(strokeWidth: 2),
      ),
      error: (e, _) => const Icon(Icons.error),
      data: (classrooms) {
        if (classrooms.isEmpty) return const SizedBox.shrink();
        return DropdownButtonHideUnderline(
          child: DropdownButton<String>(
            value: active?.id,
            hint: const Text('Select classroom'),
            items: classrooms
                .map((c) => DropdownMenuItem(
                      value: c.id,
                      child: Text(c.name),
                    ))
                .toList(),
            onChanged: (id) {
              if (id == null) return;
              final classroom = classrooms.firstWhere((c) => c.id == id);
              ref.read(activeClassroomProvider.notifier).select(classroom);
            },
          ),
        );
      },
    );
  }
}
