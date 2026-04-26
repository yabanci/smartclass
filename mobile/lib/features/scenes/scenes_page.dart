import '../../core/utils/error_utils.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/i18n/app_localizations.dart';
import '../../shared/models/device.dart';
import '../../shared/models/scene.dart';
import '../../shared/providers/classroom_provider.dart';
import '../../shared/providers/device_provider.dart';
import '../../shared/providers/scene_provider.dart';
import '../../shared/widgets/error_view.dart';
import '../../shared/widgets/loading_indicator.dart';

class ScenesPage extends ConsumerWidget {
  const ScenesPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context)!;
    final classroom = ref.watch(activeClassroomProvider);

    return Scaffold(
      appBar: AppBar(title: Text(l.scenesTitle)),
      body: classroom == null
          ? Center(child: Text(l.homeNoClassroom))
          : _SceneList(classroomId: classroom.id),
      floatingActionButton: classroom == null
          ? null
          : FloatingActionButton(
              onPressed: () => _showAddScene(context, ref, classroom.id),
              child: const Icon(Icons.add),
            ),
    );
  }

  Future<void> _showAddScene(
      BuildContext context, WidgetRef ref, String classroomId) async {
    await showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (_) => _AddSceneSheet(classroomId: classroomId),
    );
  }
}

class _SceneList extends ConsumerWidget {
  final String classroomId;

  const _SceneList({required this.classroomId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context)!;
    final scenesAsync = ref.watch(sceneListProvider(classroomId));

    return scenesAsync.when(
      loading: () => const LoadingIndicator(),
      error: (e, _) => ErrorView(
        message: friendlyError(e),
        onRetry: () => ref.read(sceneListProvider(classroomId).notifier).load(),
      ),
      data: (scenes) {
        if (scenes.isEmpty) {
          return Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.auto_awesome, size: 64, color: Colors.grey),
                const SizedBox(height: 16),
                Text(l.scenesEmpty,
                    style: const TextStyle(fontSize: 16, color: Colors.grey)),
              ],
            ),
          );
        }
        return ListView.separated(
          padding: const EdgeInsets.all(16),
          itemCount: scenes.length,
          separatorBuilder: (_, __) => const SizedBox(height: 8),
          itemBuilder: (context, i) => _SceneCard(
            scene: scenes[i],
            classroomId: classroomId,
          ),
        );
      },
    );
  }
}

class _SceneCard extends ConsumerWidget {
  final Scene scene;
  final String classroomId;

  const _SceneCard({required this.scene, required this.classroomId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context)!;
    return Card(
      elevation: 0,
      color: Theme.of(context).colorScheme.surfaceContainerHighest,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Row(
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color:
                    Theme.of(context).colorScheme.primaryContainer,
                borderRadius: BorderRadius.circular(8),
              ),
              child: Icon(Icons.auto_awesome,
                  color: Theme.of(context).colorScheme.primary),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(scene.name,
                      style:
                          const TextStyle(fontWeight: FontWeight.w600)),
                  Text('${scene.steps.length} steps',
                      style: const TextStyle(
                          fontSize: 12, color: Colors.grey)),
                  if (scene.description != null)
                    Text(scene.description!,
                        style: const TextStyle(
                            fontSize: 12, color: Colors.grey),
                        overflow: TextOverflow.ellipsis),
                ],
              ),
            ),
            IconButton(
              icon: const Icon(Icons.play_arrow),
              onPressed: () => _runScene(context, ref),
              tooltip: l.scenesRun,
            ),
            IconButton(
              icon: Icon(Icons.delete_outlined,
                  color: Theme.of(context).colorScheme.error),
              onPressed: () => _confirmDelete(context, ref),
              tooltip: l.commonDelete,
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _runScene(BuildContext context, WidgetRef ref) async {
    final result = await ref
        .read(sceneListProvider(classroomId).notifier)
        .run(scene.id);
    if (context.mounted) {
      if (result == null) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Failed to run scene')),
        );
      } else {
        final msg = result.successCount == result.total
            ? '${result.total} steps completed'
            : '${result.successCount}/${result.total} steps OK';
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(msg),
            backgroundColor: result.successCount == result.total
                ? Colors.green
                : Colors.orange,
          ),
        );
      }
    }
  }

  Future<void> _confirmDelete(BuildContext context, WidgetRef ref) async {
    final l = AppLocalizations.of(context)!;
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l.commonDelete),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: Text(l.commonCancel)),
          FilledButton(
              style: FilledButton.styleFrom(
                  backgroundColor: Theme.of(ctx).colorScheme.error),
              onPressed: () => Navigator.pop(ctx, true),
              child: Text(l.commonDelete)),
        ],
      ),
    );
    if (confirm == true) {
      await ref.read(sceneListProvider(classroomId).notifier).delete(scene.id);
    }
  }
}

class _AddSceneSheet extends ConsumerStatefulWidget {
  final String classroomId;

  const _AddSceneSheet({required this.classroomId});

  @override
  ConsumerState<_AddSceneSheet> createState() => _AddSceneSheetState();
}

class _AddSceneSheetState extends ConsumerState<_AddSceneSheet> {
  final _formKey = GlobalKey<FormState>();
  final _nameCtrl = TextEditingController();
  final _descCtrl = TextEditingController();
  final List<SceneStep> _steps = [];
  bool _saving = false;

  static const _commands = ['ON', 'OFF', 'OPEN', 'CLOSE', 'SET_VALUE'];

  @override
  void dispose() {
    _nameCtrl.dispose();
    _descCtrl.dispose();
    super.dispose();
  }

  void _addStep(List<Device> devices) {
    if (devices.isEmpty) return;
    setState(() {
      _steps.add(SceneStep(
        deviceId: devices.first.id,
        command: 'ON',
      ));
    });
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    if (_steps.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Add at least one step')),
      );
      return;
    }
    setState(() => _saving = true);
    try {
      await ref.read(sceneListProvider(widget.classroomId).notifier).create(
            name: _nameCtrl.text.trim(),
            description: _descCtrl.text.isNotEmpty
                ? _descCtrl.text.trim()
                : null,
            steps: _steps,
          );
      if (mounted) Navigator.of(context).pop();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(friendlyError(e))));
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context)!;
    final devicesAsync =
        ref.watch(deviceListProvider(widget.classroomId));
    final devices = devicesAsync.valueOrNull ?? [];

    return Padding(
      padding: EdgeInsets.only(
        left: 16,
        right: 16,
        top: 16,
        bottom: MediaQuery.of(context).viewInsets.bottom + 16,
      ),
      child: Form(
        key: _formKey,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(l.scenesAdd,
                  style:
                      const TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
              const SizedBox(height: 16),
              TextFormField(
                controller: _nameCtrl,
                decoration: InputDecoration(
                  labelText: l.scenesName,
                  border: const OutlineInputBorder(),
                ),
                validator: (v) =>
                    v == null || v.isEmpty ? '${l.scenesName} is required' : null,
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _descCtrl,
                decoration: InputDecoration(
                  labelText: '${l.scenesDescription} (optional)',
                  border: const OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 16),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text('Steps',
                      style: const TextStyle(fontWeight: FontWeight.bold)),
                  TextButton.icon(
                    icon: const Icon(Icons.add, size: 16),
                    label: Text(l.scenesAddStep),
                    onPressed: () => _addStep(devices),
                  ),
                ],
              ),
              ..._steps.asMap().entries.map((entry) {
                final idx = entry.key;
                final step = entry.value;
                return Card(
                  margin: const EdgeInsets.only(bottom: 8),
                  child: Padding(
                    padding: const EdgeInsets.all(8),
                    child: Row(
                      children: [
                        Expanded(
                          child: DropdownButton<String>(
                            value: devices
                                    .any((d) => d.id == step.deviceId)
                                ? step.deviceId
                                : (devices.isNotEmpty
                                    ? devices.first.id
                                    : null),
                            isExpanded: true,
                            underline: const SizedBox(),
                            items: devices
                                .map((d) => DropdownMenuItem(
                                    value: d.id, child: Text(d.name)))
                                .toList(),
                            onChanged: (v) {
                              if (v != null) {
                                setState(() {
                                  _steps[idx] = SceneStep(
                                      deviceId: v,
                                      command: step.command);
                                });
                              }
                            },
                          ),
                        ),
                        const SizedBox(width: 8),
                        DropdownButton<String>(
                          value: step.command,
                          underline: const SizedBox(),
                          items: _commands
                              .map((c) => DropdownMenuItem(
                                  value: c, child: Text(c)))
                              .toList(),
                          onChanged: (v) {
                            if (v != null) {
                              setState(() {
                                _steps[idx] = SceneStep(
                                    deviceId: step.deviceId,
                                    command: v);
                              });
                            }
                          },
                        ),
                        IconButton(
                          icon: const Icon(Icons.close, size: 16),
                          onPressed: () =>
                              setState(() => _steps.removeAt(idx)),
                        ),
                      ],
                    ),
                  ),
                );
              }),
              const SizedBox(height: 16),
              FilledButton(
                onPressed: _saving ? null : _submit,
                child: _saving
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : Text(l.commonCreate),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
