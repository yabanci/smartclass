import 'package:flutter/material.dart';

import '../../app.dart';

class AppCard extends StatelessWidget {
  final Widget child;
  final EdgeInsetsGeometry? padding;
  final VoidCallback? onTap;
  final Color? color;
  final bool glass;

  const AppCard({
    super.key,
    required this.child,
    this.padding,
    this.onTap,
    this.color,
    this.glass = true,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    final bg = color ??
        (isDark
            ? kDarkCard.withValues(alpha: glass ? 0.85 : 1.0)
            : Colors.white.withValues(alpha: glass ? 0.75 : 1.0));

    final border = Border.all(
      color: isDark
          ? Colors.white.withValues(alpha: 0.08)
          : const Color(0xFF1e3a8a).withValues(alpha: 0.08),
      width: 1,
    );

    final shadow = [
      BoxShadow(
        color: kPrimary.withValues(alpha: isDark ? 0.0 : 0.07),
        blurRadius: 20,
        offset: const Offset(0, 4),
      ),
      if (isDark)
        BoxShadow(
          color: Colors.black.withValues(alpha: 0.3),
          blurRadius: 20,
          offset: const Offset(0, 4),
        ),
    ];

    Widget content = Container(
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(16),
        border: border,
        boxShadow: shadow,
      ),
      child: Padding(
        padding: padding ?? const EdgeInsets.all(16),
        child: child,
      ),
    );

    if (onTap != null) {
      content = GestureDetector(
        onTap: onTap,
        child: content,
      );
    }

    return content;
  }
}

// Tinted card — for device type backgrounds (climate/light/fan/cover)
class TintCard extends StatelessWidget {
  final Widget child;
  final Color tint;
  final EdgeInsetsGeometry? padding;

  const TintCard({
    super.key,
    required this.child,
    required this.tint,
    this.padding,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: tint.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(12),
      ),
      padding: padding ?? const EdgeInsets.all(8),
      child: child,
    );
  }
}
