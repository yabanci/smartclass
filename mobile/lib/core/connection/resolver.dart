import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
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

    // C-016: check whether the remote is reachable. If not, expose Unreachable
    // state so callers (e.g. offline_banner) can show a distinct message.
    final remoteBase = appConfig.apiBaseUrl;
    // Strip /api/v1 suffix to ping the bare health endpoint.
    final remoteRoot = remoteBase.endsWith('/api/v1')
        ? remoteBase.substring(0, remoteBase.length - '/api/v1'.length)
        : remoteBase;
    final remoteReachable = await _ping(remoteRoot);

    if (!remoteReachable) {
      _current = ConnectionState(
        mode: ConnectionMode.unreachable,
        baseUrl: remoteBase,
      );
      return _current!;
    }

    _current = ConnectionState(
      mode: ConnectionMode.remote,
      baseUrl: remoteBase,
    );
    return _current!;
  }

  Future<bool> _ping(String baseUrl) async {
    try {
      final dio = Dio(BaseOptions(
        connectTimeout: const Duration(milliseconds: 600),
        receiveTimeout: const Duration(milliseconds: 600),
      ));
      // B-206: only 2xx counts as reachable; 404 is not a healthy endpoint
      final response = await dio.get('$baseUrl/healthz');
      return response.statusCode != null &&
          response.statusCode! >= 200 &&
          response.statusCode! < 300;
    } catch (_) {
      return false;
    }
  }

  Future<void> setLocalUrl(String url) async {
    if (kReleaseMode && url.startsWith('http://')) {
      throw ArgumentError(
        'Local server URL must use https:// in release builds.',
      );
    }
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_kLocalUrlKey, url);
    await resolve();
  }

  Future<String?> getLocalUrl() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_kLocalUrlKey);
  }

  // B-109: use Uri.parse to safely convert http→ws and preserve the /api/v1 path.
  // B-302: keep path intact so callers can append /ws directly.
  String get wsBaseUrl {
    final uri = Uri.parse(current.baseUrl);
    final wsScheme = uri.scheme == 'https' ? 'wss' : 'ws';
    return uri.replace(scheme: wsScheme).toString();
  }
}
