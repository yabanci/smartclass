import 'package:dio/dio.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../config/app_config.dart';
import 'connection_mode.dart';

const _kLocalUrlKey = 'local_server_url';

class ConnectionResolver {
  ConnectionResolver._();

  static final ConnectionResolver instance = ConnectionResolver._();

  ConnectionState? _current;

  ConnectionState get current =>
      _current ??
      ConnectionState(
        mode: ConnectionMode.remote,
        baseUrl: appConfig.apiBaseUrl,
      );

  Future<ConnectionState> resolve() async {
    final prefs = await SharedPreferences.getInstance();
    final localUrl = prefs.getString(_kLocalUrlKey);

    if (localUrl != null && localUrl.isNotEmpty) {
      final reachable = await _ping(localUrl);
      if (reachable) {
        _current = ConnectionState(
          mode: ConnectionMode.local,
          baseUrl: '$localUrl/api/v1',
        );
        return _current!;
      }
    }

    _current = ConnectionState(
      mode: ConnectionMode.remote,
      baseUrl: appConfig.apiBaseUrl,
    );
    return _current!;
  }

  Future<bool> _ping(String baseUrl) async {
    try {
      final dio = Dio(BaseOptions(
        connectTimeout: const Duration(milliseconds: 600),
        receiveTimeout: const Duration(milliseconds: 600),
      ));
      final response = await dio.get('$baseUrl/healthz');
      return response.statusCode != null && response.statusCode! < 500;
    } catch (_) {
      return false;
    }
  }

  Future<void> setLocalUrl(String url) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_kLocalUrlKey, url);
    await resolve();
  }

  Future<String?> getLocalUrl() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_kLocalUrlKey);
  }

  String get wsBaseUrl {
    final base = current.baseUrl
        .replaceFirst('https://', 'wss://')
        .replaceFirst('http://', 'ws://');
    // Strip /api/v1 suffix for WS construction
    return base;
  }
}
