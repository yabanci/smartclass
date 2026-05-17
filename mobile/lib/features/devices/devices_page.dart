import '../../core/utils/error_utils.dart';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/i18n/app_localizations.dart';
import '../../shared/models/device.dart';
import '../../shared/providers/classroom_provider.dart';
import '../../shared/providers/device_provider.dart';
import '../../shared/widgets/cached_banner.dart';
import '../../shared/widgets/error_view.dart';
import '../../shared/widgets/loading_indicator.dart';
import 'device_card.dart';
import 'device_form.dart';

class DevicesPage extends ConsumerWidget {
  const DevicesPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final classroom = ref.watch(activeClassroomProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text(classroom != null ? classroom.name : l.devicesTitle),
        actions: [
          TextButton.icon(
            icon: const Icon(Icons.wifi_find),
            label: Text(l.devicesFindIot),
            onPressed: () => context.push('/devices/iot-wizard'),
          ),
        ],
      ),
      body: classroom == null
          ? Center(child: Text(l.homeNoClassroom))
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
    final l = AppLocalizations.of(context);
    final devicesAsync = ref.watch(deviceListProvider(classroomId));
    final isFromCache = ref.watch(deviceFromCacheProvider(classroomId));

    return Column(
      children: [
        if (isFromCache) const CachedBanner(),
        Expanded(
          child: devicesAsync.when(
      loading: () => const LoadingIndicator(),
      error: (e, _) => ErrorView(
        message: friendlyError(e),
        onRetry: () =>
            ref.read(deviceListProvider(classroomId).notifier).load(),
        retryLabel: l.commonRetry,
      ),
      data: (devices) {
        if (devices.isEmpty) {
          return Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.devices_other, size: 64, color: Colors.grey),
                const SizedBox(height: 16),
                Text(l.devicesEmpty,
                    style: const TextStyle(fontSize: 16, color: Colors.grey)),
                const SizedBox(height: 16),
                OutlinedButton.icon(
                  icon: const Icon(Icons.wifi_find),
                  label: Text(l.devicesFindIot),
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
                onEdit: () => _showEditForm(context, ref, classroomId, device),
              );
            },
          ),
        );
      },
          ),
        ),
      ],
    );
  }

  Future<void> _showEditForm(
    BuildContext context,
    WidgetRef ref,
    String classroomId,
    Device device,
  ) async {
    await showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (_) => _EditDeviceSheet(
        classroomId: classroomId,
        device: device,
      ),
    );
  }

  Future<void> _confirmDelete(
    BuildContext context,
    WidgetRef ref,
    String deviceId,
  ) async {
    final l = AppLocalizations.of(context);
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l.commonDelete),
        // B-203: use l10n key instead of hardcoded English
        content: Text(l.commonCannotUndo),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: Text(l.commonCancel),
          ),
          FilledButton(
            style: FilledButton.styleFrom(
                backgroundColor: Theme.of(ctx).colorScheme.error),
            onPressed: () => Navigator.pop(ctx, true),
            child: Text(l.commonDelete),
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

class _EditDeviceSheet extends ConsumerStatefulWidget {
  final String classroomId;
  final Device device;

  const _EditDeviceSheet({
    required this.classroomId,
    required this.device,
  });

  @override
  ConsumerState<_EditDeviceSheet> createState() => _EditDeviceSheetState();
}

class _EditDeviceSheetState extends ConsumerState<_EditDeviceSheet> {
  final _formKey = GlobalKey<FormState>();
  late TextEditingController _nameCtrl;
  late TextEditingController _brandCtrl;
  late TextEditingController _driverCtrl;
  late TextEditingController _configCtrl;
  late String _type;
  bool _saving = false;

  static const _types = [
    'switch', 'light', 'climate', 'fan', 'cover', 'sensor'
  ];

  @override
  void initState() {
    super.initState();
    _nameCtrl = TextEditingController(text: widget.device.name);
    _brandCtrl = TextEditingController(text: widget.device.brand);
    _driverCtrl = TextEditingController(text: widget.device.driver);
    _configCtrl = TextEditingController(
        text: jsonEncode(widget.device.config));
    _type = widget.device.type;
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _brandCtrl.dispose();
    _driverCtrl.dispose();
    _configCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _saving = true);
    try {
      Map<String, dynamic> config = {};
      try {
        config = jsonDecode(_configCtrl.text) as Map<String, dynamic>;
      } catch (_) {}

      await ref.read(deviceEndpointsProvider).update(widget.device.id, {
        'name': _nameCtrl.text.trim(),
        'type': _type,
        'brand': _brandCtrl.text.trim(),
        'driver': _driverCtrl.text.trim(),
        'config': config,
      });
      ref.invalidate(deviceListProvider(widget.classroomId));
      if (mounted) Navigator.of(context).pop();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(friendlyError(e)), backgroundColor: Colors.red),
        );
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
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
              Text(l.commonEdit,
                  style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
              const SizedBox(height: 16),
              TextFormField(
                controller: _nameCtrl,
                decoration: InputDecoration(
                  labelText: l.devicesName,
                  border: const OutlineInputBorder(),
                ),
                validator: (v) =>
                    v == null || v.isEmpty ? '${l.devicesName} is required' : null,
              ),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                value: _type,
                decoration: InputDecoration(
                  labelText: l.devicesType,
                  border: const OutlineInputBorder(),
                ),
                items: _types
                    .map((t) => DropdownMenuItem(value: t, child: Text(t)))
                    .toList(),
                onChanged: (v) => setState(() => _type = v ?? 'switch'),
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _brandCtrl,
                decoration: InputDecoration(
                  labelText: l.devicesBrand,
                  border: const OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _driverCtrl,
                decoration: InputDecoration(
                  labelText: l.devicesDriver,
                  border: const OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _configCtrl,
                decoration: InputDecoration(
                  labelText: l.devicesConfig,
                  border: const OutlineInputBorder(),
                ),
                maxLines: 3,
                validator: (v) {
                  try {
                    if (v != null) jsonDecode(v);
                    return null;
                  } catch (_) {
                    return 'Invalid JSON';
                  }
                },
              ),
              const SizedBox(height: 16),
              FilledButton(
                onPressed: _saving ? null : _submit,
                child: _saving
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : Text(l.commonSave),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
