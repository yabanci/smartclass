import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../shared/models/device.dart';
import '../../shared/providers/device_provider.dart';
import '../../shared/widgets/status_badge.dart';

IconData deviceIcon(String type) {
  final t = type.toLowerCase();
  if (t.contains('climate') || t.contains('ac') || t.contains('thermo')) {
    return Icons.ac_unit;
  }
  if (t.contains('light')) return Icons.lightbulb_outlined;
  if (t.contains('fan')) return Icons.air;
  if (t.contains('cover') || t.contains('blind') || t.contains('curtain')) {
    return Icons.blinds;
  }
  if (t.contains('sensor')) return Icons.thermostat;
  return Icons.power;
}

class DeviceCard extends ConsumerStatefulWidget {
  final Device device;
  final String classroomId;
  final VoidCallback? onDelete;
  final VoidCallback? onEdit;

  const DeviceCard({
    super.key,
    required this.device,
    required this.classroomId,
    this.onDelete,
    this.onEdit,
  });

  @override
  ConsumerState<DeviceCard> createState() => _DeviceCardState();
}

class _DeviceCardState extends ConsumerState<DeviceCard> {
  bool _sending = false;

  Future<void> _sendCommand(String type, {dynamic value}) async {
    setState(() => _sending = true);
    try {
      await ref
          .read(deviceListProvider(widget.classroomId).notifier)
          .sendCommand(widget.device.id, type, value: value);
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final device = widget.device;
    final theme = Theme.of(context);
    final isOn = device.isOn;

    return Card(
      elevation: 0,
      color: theme.colorScheme.surfaceContainerHighest,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    color: isOn
                        ? theme.colorScheme.primaryContainer
                        : theme.colorScheme.surfaceContainer,
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Icon(
                    deviceIcon(device.type),
                    color: isOn
                        ? theme.colorScheme.primary
                        : theme.colorScheme.onSurfaceVariant,
                    size: 20,
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(device.name,
                          style: const TextStyle(fontWeight: FontWeight.w600),
                          overflow: TextOverflow.ellipsis),
                      Text(device.brand,
                          style: const TextStyle(
                              fontSize: 12, color: Colors.grey)),
                    ],
                  ),
                ),
                StatusBadge(online: device.online),
                const SizedBox(width: 8),
                if (_sending)
                  const SizedBox(
                    width: 24,
                    height: 24,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                else
                  Switch(
                    value: isOn,
                    onChanged: (val) =>
                        _sendCommand(val ? 'ON' : 'OFF'),
                  ),
              ],
            ),

            // Controls visible when ON
            if (isOn) _buildControls(device, theme),

            // Actions row
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                if (widget.onEdit != null)
                  IconButton(
                    icon: const Icon(Icons.edit_outlined, size: 18),
                    onPressed: widget.onEdit,
                    tooltip: 'Edit',
                    visualDensity: VisualDensity.compact,
                  ),
                if (widget.onDelete != null)
                  IconButton(
                    icon: Icon(Icons.delete_outlined,
                        size: 18,
                        color: theme.colorScheme.error),
                    onPressed: widget.onDelete,
                    tooltip: 'Delete',
                    visualDensity: VisualDensity.compact,
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildControls(Device device, ThemeData theme) {
    final t = device.type.toLowerCase();

    if (t.contains('light')) {
      return _SliderControl(
        label: 'Brightness',
        value: (device.config['brightness'] as num?)?.toDouble() ?? 100.0,
        min: 0,
        max: 100,
        divisions: 10,
        unit: '%',
        onChanged: (v) => _sendCommand('SET_VALUE', value: v.round()),
      );
    }

    if (t.contains('climate') || t.contains('ac') || t.contains('thermo')) {
      return _SliderControl(
        label: 'Temperature',
        value: (device.config['temperature'] as num?)?.toDouble() ?? 22.0,
        min: 16,
        max: 30,
        divisions: 14,
        unit: '°C',
        onChanged: (v) => _sendCommand('SET_VALUE', value: v.round()),
      );
    }

    if (t.contains('fan')) {
      return _FanControl(
        value: (device.config['level'] as num?)?.toInt() ?? 33,
        onChanged: (v) => _sendCommand('SET_VALUE', value: v),
      );
    }

    if (t.contains('cover') || t.contains('blind') || t.contains('curtain')) {
      return _SliderControl(
        label: 'Level',
        value: (device.config['level'] as num?)?.toDouble() ?? 50.0,
        min: 0,
        max: 100,
        divisions: 10,
        unit: '%',
        onChanged: (v) => _sendCommand('SET_VALUE', value: v.round()),
      );
    }

    return const SizedBox.shrink();
  }
}

class _SliderControl extends StatefulWidget {
  final String label;
  final double value;
  final double min;
  final double max;
  final int divisions;
  final String unit;
  final ValueChanged<double> onChanged;

  const _SliderControl({
    required this.label,
    required this.value,
    required this.min,
    required this.max,
    required this.divisions,
    required this.unit,
    required this.onChanged,
  });

  @override
  State<_SliderControl> createState() => _SliderControlState();
}

class _SliderControlState extends State<_SliderControl> {
  late double _localValue;

  @override
  void initState() {
    super.initState();
    _localValue = widget.value;
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 8),
      child: Row(
        children: [
          Text('${widget.label}: ${_localValue.round()}${widget.unit}',
              style: const TextStyle(fontSize: 12)),
          Expanded(
            child: Slider(
              value: _localValue.clamp(widget.min, widget.max),
              min: widget.min,
              max: widget.max,
              divisions: widget.divisions,
              onChanged: (v) => setState(() => _localValue = v),
              onChangeEnd: widget.onChanged,
            ),
          ),
        ],
      ),
    );
  }
}

class _FanControl extends StatelessWidget {
  final int value;
  final ValueChanged<int> onChanged;

  const _FanControl({required this.value, required this.onChanged});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 8),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceEvenly,
        children: [
          _FanButton(label: 'Low', level: 33, current: value, onTap: onChanged),
          _FanButton(
              label: 'Medium', level: 66, current: value, onTap: onChanged),
          _FanButton(
              label: 'High', level: 100, current: value, onTap: onChanged),
        ],
      ),
    );
  }
}

class _FanButton extends StatelessWidget {
  final String label;
  final int level;
  final int current;
  final ValueChanged<int> onTap;

  const _FanButton({
    required this.label,
    required this.level,
    required this.current,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final active = current == level;
    return OutlinedButton(
      style: OutlinedButton.styleFrom(
        backgroundColor: active
            ? Theme.of(context).colorScheme.primaryContainer
            : null,
        side: active
            ? BorderSide(
                color: Theme.of(context).colorScheme.primary, width: 2)
            : null,
      ),
      onPressed: () => onTap(level),
      child: Text(label, style: const TextStyle(fontSize: 12)),
    );
  }
}
