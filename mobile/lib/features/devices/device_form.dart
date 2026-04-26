import '../../core/utils/error_utils.dart';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/i18n/app_localizations.dart';
import '../../shared/providers/device_provider.dart';

class DeviceFormSheet extends ConsumerStatefulWidget {
  final String classroomId;

  const DeviceFormSheet({super.key, required this.classroomId});

  @override
  ConsumerState<DeviceFormSheet> createState() => _DeviceFormSheetState();
}

class _DeviceFormSheetState extends ConsumerState<DeviceFormSheet> {
  final _formKey = GlobalKey<FormState>();
  final _nameCtrl = TextEditingController();
  final _brandCtrl = TextEditingController(text: 'Generic');
  final _driverCtrl = TextEditingController(text: 'generic');
  final _configCtrl = TextEditingController(text: '{}');
  String _type = 'switch';
  bool _saving = false;

  static const _types = [
    'switch', 'light', 'climate', 'fan', 'cover', 'sensor'
  ];

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

      final device = await ref
          .read(deviceListProvider(widget.classroomId).notifier)
          .create(
            name: _nameCtrl.text.trim(),
            type: _type,
            brand: _brandCtrl.text.trim(),
            driver: _driverCtrl.text.trim(),
            config: config,
          );
      if (mounted) Navigator.of(context).pop(device);
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
    final l = AppLocalizations.of(context)!;
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
              Text(l.devicesAdd,
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
                    : Text(l.devicesAdd),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
