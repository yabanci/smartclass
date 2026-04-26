import 'package:flutter/material.dart';

class StatusBadge extends StatelessWidget {
  final bool online;
  final String? label;

  const StatusBadge({super.key, required this.online, this.label});

  @override
  Widget build(BuildContext context) {
    final color = online ? Colors.green : Colors.grey;
    final text = label ?? (online ? 'Online' : 'Offline');
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          width: 8,
          height: 8,
          decoration: BoxDecoration(color: color, shape: BoxShape.circle),
        ),
        const SizedBox(width: 4),
        Text(
          text,
          style: TextStyle(fontSize: 12, color: color),
        ),
      ],
    );
  }
}
