enum AppFlavor { dev, prod }

class AppConfig {
  final AppFlavor flavor;
  final String apiBaseUrl;
  final String wsBaseUrl;
  final String appName;
  final bool debugBanner;

  const AppConfig({
    required this.flavor,
    required this.apiBaseUrl,
    required this.wsBaseUrl,
    required this.appName,
    this.debugBanner = false,
  });

  static const dev = AppConfig(
    flavor: AppFlavor.dev,
    apiBaseUrl: 'http://localhost:8080/api/v1',
    wsBaseUrl: 'ws://localhost:8080/api/v1',
    appName: 'SmartClass Dev',
    debugBanner: true,
  );

  static const prod = AppConfig(
    flavor: AppFlavor.prod,
    apiBaseUrl: 'https://api.smartclass.kz/api/v1',
    wsBaseUrl: 'wss://api.smartclass.kz/api/v1',
    appName: 'Smart Classroom',
    debugBanner: false,
  );
}

/// Global singleton set once at startup by mainWithConfig().
/// Falls back to [AppConfig.dev] when not explicitly set (e.g. in tests).
AppConfig get appConfig => _appConfig ?? AppConfig.dev;
AppConfig? _appConfig;

void setAppConfig(AppConfig config) {
  _appConfig ??= config;
}
