import 'package:flutter/material.dart';

import '../../core/i18n/app_localizations.dart';

/// Small pill shown at the top of a screen when data was loaded from cache.
class CachedBanner extends StatelessWidget {
  const CachedBanner({super.key});

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(vertical: 6, horizontal: 16),
      color: Colors.amber.shade100,
      child: Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.history, size: 14, color: Colors.amber.shade800),
          const SizedBox(width: 6),
          Text(
            l.cachedDataLabel,
            style: TextStyle(
              fontSize: 12,
              color: Colors.amber.shade900,
              fontWeight: FontWeight.w500,
            ),
          ),
        ],
      ),
    );
  }
}
