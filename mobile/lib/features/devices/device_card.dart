import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app.dart';
import '../../core/i18n/app_localizations.dart';
import '../../core/utils/error_utils.dart';
import '../../shared/models/device.dart';
import '../../shared/providers/device_provider.dart';
import '../../shared/widgets/app_card.dart';

// Device type → emoji + tint color (mirrors React frontend)
typedef _DeviceStyle = ({String emoji, Color tint});

_DeviceStyle _styleFor(String type) {
  final t = type.toLowerCase();
  if (t.contains('climat') || t.contains('ac') || t.contains('thermo')) {
    return (emoji: '❄️', tint: Colors.blue);
  }
  if (t.contains('light')) return (emoji: '💡', tint: Colors.amber);
  if (t.contains('fan') || t.contains('fresh')) {
    return (emoji: '🌬️', tint: Colors.green);
  }
  if (t.contains('cover') || t.contains('blind') || t.contains('curtain')) {
    return (emoji: '🪟', tint: Colors.purple);
  }
  if (t.contains('sensor')) return (emoji: '🌡️', tint: Colors.red);
  return (emoji: '🔌', tint: Colors.grey);
}

IconData deviceIcon(String type) {
  final t = type.toLowerCase();
  if (t.contains('climat') || t.contains('ac') || t.contains('thermo')) return Icons.ac_unit;
  if (t.contains('light')) return Icons.lightbulb_outlined;
  if (t.contains('fan')) return Icons.air;
  if (t.contains('cover') || t.contains('blind') || t.contains('curtain')) return Icons.blinds;
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
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(friendlyError(e)),
            backgroundColor: kDanger,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context)!;
    final device = widget.device;
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final isOn = device.isOn;
    final style = _styleFor(device.type);

    return AppCard(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              // Type icon with tinted background
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: style.tint.withOpacity(isDark ? 0.2 : 0.1),
                  borderRadius: BorderRadius.circular(12),
                ),
                child: Center(
                  child: Text(style.emoji, style: const TextStyle(fontSize: 22)),
                ),
              ),
              const SizedBox(width: 12),

              // Name + brand
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Flexible(
                          child: Text(
                            device.name,
                            style: const TextStyle(
                              fontWeight: FontWeight.w700,
                              fontSize: 14,
                            ),
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                        const SizedBox(width: 6),
                        // Online dot
                        Container(
                          width: 8,
                          height: 8,
                          decoration: BoxDecoration(
                            color: device.online ? kAccent : Colors.grey,
                            shape: BoxShape.circle,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 2),
                    Text(
                      '${device.brand} · ${device.type}',
                      style: TextStyle(
                        fontSize: 12,
                        color: isDark ? Colors.white54 : const Color(0xFF64748b),
                      ),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                ),
              ),

              // Toggle or spinner
              if (_sending)
                const SizedBox(
                  width: 32,
                  height: 32,
                  child: CircularProgressIndicator(strokeWidth: 2, color: kPrimary),
                )
              else if (device.type.toLowerCase() != 'sensor')
                _Toggle(isOn: isOn, onToggle: () => _sendCommand(isOn ? 'OFF' : 'ON')),

              // Edit / delete
              IconButton(
                icon: Icon(Icons.edit_outlined, size: 16,
                    color: kPrimary.withOpacity(0.8)),
                onPressed: widget.onEdit,
                visualDensity: VisualDensity.compact,
                padding: const EdgeInsets.all(6),
                tooltip: l.commonEdit,
              ),
              IconButton(
                icon: const Icon(Icons.delete_outlined, size: 16, color: kDanger),
                onPressed: widget.onDelete,
                visualDensity: VisualDensity.compact,
                padding: const EdgeInsets.all(6),
                tooltip: l.commonDelete,
              ),
            ],
          ),

          // Inline controls when ON
          if (isOn) _buildControls(device, l),
        ],
      ),
    );
  }

  Widget _buildControls(Device device, AppLocalizations l) {
    final t = device.type.toLowerCase();
    final configVal = (device.config['lastValue'] as num?)?.toDouble();

    if (t.contains('light')) {
      return _SliderControl(
        label: l.devicesBrightness, unit: '%', min: 0, max: 100,
        value: configVal ?? 75,
        color: kPrimary,
        onCommit: (v) => _sendCommand('SET_VALUE', value: v.round()),
      );
    }
    if (t.contains('climat') || t.contains('ac') || t.contains('thermo')) {
      return _SliderControl(
        label: l.devicesTemperature, unit: '°C', min: 16, max: 30,
        value: configVal ?? 22,
        color: Colors.blue,
        onCommit: (v) => _sendCommand('SET_VALUE', value: v.round()),
      );
    }
    if (t.contains('fan')) {
      return _FanControl(
        value: configVal?.toInt() ?? 33,
        onChanged: (v) => _sendCommand('SET_VALUE', value: v),
        labelLow: l.devicesLevelLow,
        labelMid: l.devicesLevelMid,
        labelHigh: l.devicesLevelHigh,
      );
    }
    if (t.contains('cover') || t.contains('blind') || t.contains('curtain')) {
      return _SliderControl(
        label: l.devicesLevel, unit: '%', min: 0, max: 100,
        value: configVal ?? 50,
        color: Colors.purple,
        onCommit: (v) => _sendCommand('SET_VALUE', value: v.round()),
      );
    }
    return const SizedBox.shrink();
  }
}

// Green/grey pill toggle — matches React .toggle CSS
class _Toggle extends StatelessWidget {
  final bool isOn;
  final VoidCallback onToggle;

  const _Toggle({required this.isOn, required this.onToggle});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onToggle,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 250),
        width: 48,
        height: 26,
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(13),
          color: isOn ? kAccent : const Color(0xFFCBD5E1),
        ),
        child: AnimatedAlign(
          duration: const Duration(milliseconds: 250),
          alignment: isOn ? Alignment.centerRight : Alignment.centerLeft,
          child: Container(
            width: 22,
            height: 22,
            margin: const EdgeInsets.all(2),
            decoration: const BoxDecoration(
              color: Colors.white,
              shape: BoxShape.circle,
            ),
          ),
        ),
      ),
    );
  }
}

class _SliderControl extends StatefulWidget {
  final String label;
  final String unit;
  final double value;
  final double min;
  final double max;
  final Color color;
  final ValueChanged<double> onCommit;

  const _SliderControl({
    required this.label,
    required this.unit,
    required this.value,
    required this.min,
    required this.max,
    required this.color,
    required this.onCommit,
  });

  @override
  State<_SliderControl> createState() => _SliderControlState();
}

class _SliderControlState extends State<_SliderControl> {
  late double _val;

  @override
  void initState() {
    super.initState();
    _val = widget.value.clamp(widget.min, widget.max);
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 12),
      child: Column(
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(widget.label,
                  style: const TextStyle(fontSize: 12, color: Colors.grey)),
              Text(
                '${_val.round()}${widget.unit}',
                style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w700,
                  color: widget.color,
                ),
              ),
            ],
          ),
          SliderTheme(
            data: SliderThemeData(
              trackHeight: 6,
              activeTrackColor: widget.color,
              inactiveTrackColor: widget.color.withOpacity(0.2),
              thumbColor: widget.color,
              overlayColor: widget.color.withOpacity(0.1),
              thumbShape: const RoundSliderThumbShape(enabledThumbRadius: 10),
            ),
            child: Slider(
              value: _val,
              min: widget.min,
              max: widget.max,
              onChanged: (v) => setState(() => _val = v),
              onChangeEnd: widget.onCommit,
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
  final String labelLow;
  final String labelMid;
  final String labelHigh;

  const _FanControl({
    required this.value,
    required this.onChanged,
    required this.labelLow,
    required this.labelMid,
    required this.labelHigh,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 12),
      child: Row(
        children: [
          _FanBtn(label: labelLow, level: 33, current: value, onTap: onChanged),
          const SizedBox(width: 6),
          _FanBtn(label: labelMid, level: 66, current: value, onTap: onChanged),
          const SizedBox(width: 6),
          _FanBtn(label: labelHigh, level: 100, current: value, onTap: onChanged),
        ],
      ),
    );
  }
}

class _FanBtn extends StatelessWidget {
  final String label;
  final int level;
  final int current;
  final ValueChanged<int> onTap;

  const _FanBtn({required this.label, required this.level,
      required this.current, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final active = current == level;
    return Expanded(
      child: GestureDetector(
        onTap: () => onTap(level),
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 200),
          padding: const EdgeInsets.symmetric(vertical: 8),
          decoration: BoxDecoration(
            color: active ? kPrimary : const Color(0xFFE2E8F0),
            borderRadius: BorderRadius.circular(8),
          ),
          child: Center(
            child: Text(
              label,
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w600,
                color: active ? Colors.white : const Color(0xFF475569),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
