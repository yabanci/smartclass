// IMPORTANT: Run `flutter pub get` before running this app.
// The app uses flutter_gen for localizations (run: flutter gen-l10n or flutter pub get).
//
// Entry points:
//   flutter run -t lib/main_dev.dart   → dev flavor (localhost)
//   flutter run -t lib/main_prod.dart  → prod flavor (api.smartclass.kz)
//   flutter run                        → defaults to dev

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app.dart';
import 'config/app_config.dart';
import 'core/connection/resolver.dart';
import 'shared/providers/auth_provider.dart';

/// Default entry point — dev flavor.
void main() => mainWithConfig(AppConfig.dev);

/// Shared startup logic used by all flavor entry points.
Future<void> mainWithConfig(AppConfig config) async {
  WidgetsFlutterBinding.ensureInitialized();
  setAppConfig(config);

  // Resolve connection mode before starting (uses AppConfig fallback URL)
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
