// IMPORTANT: Run `flutter pub get` before running this app.
// The app uses flutter_gen for localizations (run: flutter gen-l10n or flutter pub get).

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app.dart';
import 'core/connection/resolver.dart';
import 'shared/providers/auth_provider.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Resolve connection mode before starting
  await ConnectionResolver.instance.resolve();

  runApp(
    ProviderScope(
      child: _AppInitializer(),
    ),
  );
}

class _AppInitializer extends ConsumerStatefulWidget {
  @override
  ConsumerState<_AppInitializer> createState() => _AppInitializerState();
}

class _AppInitializerState extends ConsumerState<_AppInitializer> {
  bool _initialized = false;

  @override
  void initState() {
    super.initState();
    _init();
  }

  Future<void> _init() async {
    // Try to restore session from stored tokens
    await ref.read(authProvider.notifier).init();
    if (mounted) setState(() => _initialized = true);
  }

  @override
  Widget build(BuildContext context) {
    if (!_initialized) {
      return const MaterialApp(
        debugShowCheckedModeBanner: false,
        home: Scaffold(
          body: Center(child: CircularProgressIndicator()),
        ),
      );
    }
    return const SmartClassApp();
  }
}
