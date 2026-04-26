import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/endpoints/classroom_endpoints.dart';
import '../../core/i18n/app_localizations.dart';
import '../../core/utils/error_utils.dart';
import '../providers/auth_provider.dart';
import '../providers/classroom_provider.dart';

class ClassroomPicker extends ConsumerWidget {
  const ClassroomPicker({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final classroomsAsync = ref.watch(classroomListProvider);
    final active = ref.watch(activeClassroomProvider);
    final l = AppLocalizations.of(context)!;

    return classroomsAsync.when(
      loading: () => const SizedBox(
        width: 20,
        height: 20,
        child: CircularProgressIndicator(strokeWidth: 2),
      ),
      error: (e, _) => TextButton.icon(
        icon: const Icon(Icons.refresh, size: 16),
        label: const Text('Retry'),
        onPressed: () => ref.read(classroomListProvider.notifier).load(),
      ),
      data: (classrooms) {
        if (classrooms.isEmpty) return const SizedBox.shrink();
        return Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Flexible(
              child: DropdownButtonHideUnderline(
                child: DropdownButton<String>(
                  value: active?.id,
                  hint: Text(l.homeCreateClassroom),
                  isExpanded: false,
                  items: classrooms
                      .map((c) => DropdownMenuItem(
                            value: c.id,
                            child: Text(
                              c.name,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ))
                      .toList(),
                  onChanged: (id) {
                    if (id == null) return;
                    final classroom =
                        classrooms.firstWhere((c) => c.id == id);
                    ref
                        .read(activeClassroomProvider.notifier)
                        .select(classroom);
                  },
                ),
              ),
            ),
            // Classroom actions menu
            if (active != null)
              PopupMenuButton<_ClassroomAction>(
                icon: const Icon(Icons.more_vert, size: 18),
                tooltip: 'Classroom options',
                onSelected: (action) async {
                  switch (action) {
                    case _ClassroomAction.rename:
                      await _renameClassroom(context, ref, active.id, active.name);
                    case _ClassroomAction.delete:
                      await _deleteClassroom(context, ref, active.id);
                  }
                },
                itemBuilder: (_) => [
                  const PopupMenuItem(
                    value: _ClassroomAction.rename,
                    child: ListTile(
                      dense: true,
                      leading: Icon(Icons.edit_outlined, size: 18),
                      title: Text('Rename'),
                      contentPadding: EdgeInsets.zero,
                    ),
                  ),
                  const PopupMenuItem(
                    value: _ClassroomAction.delete,
                    child: ListTile(
                      dense: true,
                      leading: Icon(Icons.delete_outlined,
                          size: 18, color: Colors.red),
                      title: Text('Delete',
                          style: TextStyle(color: Colors.red)),
                      contentPadding: EdgeInsets.zero,
                    ),
                  ),
                ],
              ),
          ],
        );
      },
    );
  }

  Future<void> _renameClassroom(
    BuildContext context,
    WidgetRef ref,
    String id,
    String currentName,
  ) async {
    final ctrl = TextEditingController(text: currentName);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Rename classroom'),
        content: TextField(
          controller: ctrl,
          decoration: const InputDecoration(labelText: 'Name'),
          autofocus: true,
          onSubmitted: (_) => Navigator.pop(ctx, true),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Save'),
          ),
        ],
      ),
    );
    if (confirmed == true &&
        ctrl.text.trim().isNotEmpty &&
        ctrl.text.trim() != currentName &&
        context.mounted) {
      try {
        final updated = await ref
            .read(classroomEndpointsProvider)
            .update(id, name: ctrl.text.trim());
        await ref.read(classroomListProvider.notifier).load();
        ref.read(activeClassroomProvider.notifier).select(updated);
      } catch (e) {
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
                content: Text(friendlyError(e)),
                backgroundColor: Colors.red),
          );
        }
      }
    }
  }

  Future<void> _deleteClassroom(
    BuildContext context,
    WidgetRef ref,
    String id,
  ) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Delete classroom?'),
        content: const Text(
            'This will remove the classroom and all its data. This cannot be undone.'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: Colors.red),
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
    if (confirmed == true && context.mounted) {
      try {
        await ref.read(classroomListProvider.notifier).delete(id);
        ref.read(activeClassroomProvider.notifier).clear();
      } catch (e) {
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
                content: Text(friendlyError(e)),
                backgroundColor: Colors.red),
          );
        }
      }
    }
  }
}

enum _ClassroomAction { rename, delete }
