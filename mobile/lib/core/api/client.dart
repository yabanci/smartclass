import 'dart:async';

import 'package:dio/dio.dart';

import '../connection/resolver.dart';
import '../storage/token_storage.dart';
import 'envelope.dart';

typedef LogoutCallback = Future<void> Function();

class ApiClient {
  late Dio _dio;
  final TokenStorage _tokenStorage;
  LogoutCallback? _onLogout;

  bool _isRefreshing = false;
  final List<Completer<bool>> _refreshWaiters = [];

  ApiClient({TokenStorage? tokenStorage})
      : _tokenStorage = tokenStorage ?? TokenStorage() {
    _dio = _buildDio();
    _addInterceptors();
  }

  void setLogoutCallback(LogoutCallback cb) => _onLogout = cb;

  Dio _buildDio() => Dio(BaseOptions(
        baseUrl: ConnectionResolver.instance.current.baseUrl,
        connectTimeout: const Duration(seconds: 10),
        receiveTimeout: const Duration(seconds: 15),
        headers: {'Content-Type': 'application/json'},
      ));

  void updateBaseUrl() {
    _dio.options.baseUrl = ConnectionResolver.instance.current.baseUrl;
  }

  void _addInterceptors() {
    _dio.interceptors.add(InterceptorsWrapper(
      onRequest: (options, handler) async {
        // Skip auth injection for refresh endpoint to avoid infinite loop
        if (options.extra['skipAuth'] == true) {
          return handler.next(options);
        }
        final token = await _tokenStorage.getAccessToken();
        if (token != null) {
          options.headers['Authorization'] = 'Bearer $token';
        }
        handler.next(options);
      },
      onError: (DioException error, handler) async {
        // Only intercept 401 on non-refresh requests
        if (error.response?.statusCode == 401 &&
            error.requestOptions.extra['skipAuth'] != true) {
          final retried = await _handleUnauthorized(error);
          if (retried != null) return handler.resolve(retried);
        }
        handler.next(error);
      },
    ));
  }

  Future<Response?> _handleUnauthorized(DioException original) async {
    // If refresh already in progress, wait for it
    if (_isRefreshing) {
      final c = Completer<bool>();
      _refreshWaiters.add(c);
      final ok = await c.future;
      if (!ok) return null;
      return _retry(original.requestOptions);
    }

    _isRefreshing = true;
    try {
      final refreshToken = await _tokenStorage.getRefreshToken();
      if (refreshToken == null) {
        _notifyWaiters(false);
        await _logout();
        return null;
      }

      final resp = await _dio.post(
        '/auth/refresh',
        data: {'refreshToken': refreshToken},
        options: Options(extra: {'skipAuth': true}),
      );

      final body = resp.data as Map<String, dynamic>;
      // Backend wraps in { data: { tokens: {...} } }
      final tokensMap = (body['data'] as Map<String, dynamic>?)?['tokens']
          as Map<String, dynamic>?;

      if (tokensMap == null) {
        _notifyWaiters(false);
        await _logout();
        return null;
      }

      await _tokenStorage.saveTokens(
        accessToken: tokensMap['accessToken'] as String,
        refreshToken: tokensMap['refreshToken'] as String,
        accessExpiresAt: tokensMap['accessExpiresAt'] as String,
        refreshExpiresAt: tokensMap['refreshExpiresAt'] as String,
      );
      _notifyWaiters(true);
      return _retry(original.requestOptions);
    } catch (_) {
      _notifyWaiters(false);
      await _logout();
      return null;
    } finally {
      _isRefreshing = false;
    }
  }

  void _notifyWaiters(bool success) {
    for (final c in _refreshWaiters) {
      c.complete(success);
    }
    _refreshWaiters.clear();
  }

  Future<Response> _retry(RequestOptions req) async {
    final token = await _tokenStorage.getAccessToken();
    return _dio.request(
      req.path,
      data: req.data,
      queryParameters: req.queryParameters,
      options: Options(
        method: req.method,
        headers: {...req.headers, if (token != null) 'Authorization': 'Bearer $token'},
      ),
    );
  }

  Future<void> _logout() async {
    await _tokenStorage.clear();
    await _onLogout?.call();
  }

  Future<T> unwrap<T>(
    Future<Response> call,
    T Function(dynamic) fromData,
  ) async {
    final response = await call;
    final json = response.data as Map<String, dynamic>;
    final envelope = ApiEnvelope.fromJson(json, fromData);
    if (!envelope.ok) {
      throw ApiException(envelope.error ?? 'Unknown error',
          statusCode: response.statusCode);
    }
    return envelope.data as T;
  }

  Future<Response> get(String path, {Map<String, dynamic>? queryParameters, Options? options}) =>
      _dio.get(path, queryParameters: queryParameters, options: options);

  Future<Response> post(String path, {dynamic data, Map<String, dynamic>? queryParameters, Options? options}) =>
      _dio.post(path, data: data, queryParameters: queryParameters, options: options);

  Future<Response> patch(String path, {dynamic data, Options? options}) =>
      _dio.patch(path, data: data, options: options);

  Future<Response> delete(String path, {Options? options}) =>
      _dio.delete(path, options: options);
}
