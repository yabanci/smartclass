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

  // Prevents multiple concurrent token refreshes
  bool _isRefreshing = false;
  final _refreshCompleter = <Completer<bool>>[];

  ApiClient({TokenStorage? tokenStorage})
      : _tokenStorage = tokenStorage ?? TokenStorage() {
    _dio = _buildDio();
    _addInterceptors();
  }

  void setLogoutCallback(LogoutCallback cb) => _onLogout = cb;

  Dio _buildDio() {
    return Dio(BaseOptions(
      baseUrl: ConnectionResolver.instance.current.baseUrl,
      connectTimeout: const Duration(seconds: 10),
      receiveTimeout: const Duration(seconds: 15),
      headers: {'Content-Type': 'application/json'},
    ));
  }

  void updateBaseUrl() {
    _dio.options.baseUrl = ConnectionResolver.instance.current.baseUrl;
  }

  void _addInterceptors() {
    _dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) async {
          final token = await _tokenStorage.getAccessToken();
          if (token != null) {
            options.headers['Authorization'] = 'Bearer $token';
          }
          return handler.next(options);
        },
        onError: (DioException error, handler) async {
          if (error.response?.statusCode == 401) {
            final retried = await _handleUnauthorized(error);
            if (retried != null) return handler.resolve(retried);
          }
          return handler.next(error);
        },
      ),
    );
  }

  Future<Response?> _handleUnauthorized(DioException error) async {
    if (_isRefreshing) {
      // Wait for the in-progress refresh
      final completer = Completer<bool>();
      _refreshCompleter.add(completer);
      final success = await completer.future;
      if (!success) return null;
      return _retry(error.requestOptions);
    }

    _isRefreshing = true;
    try {
      final refreshToken = await _tokenStorage.getRefreshToken();
      if (refreshToken == null) {
        await _logout();
        return null;
      }

      final response = await _dio.post(
        '/auth/refresh',
        data: {'refreshToken': refreshToken},
        options: Options(
          // Skip the interceptor for this call to avoid infinite loop
          extra: {'skipAuth': true},
        ),
      );

      final data = response.data as Map<String, dynamic>;
      if (data['ok'] == true) {
        final tokens = data['data']['tokens'] as Map<String, dynamic>;
        await _tokenStorage.saveTokens(
          accessToken: tokens['accessToken'] as String,
          refreshToken: tokens['refreshToken'] as String,
          accessExpiresAt: tokens['accessExpiresAt'] as String,
          refreshExpiresAt: tokens['refreshExpiresAt'] as String,
        );
        for (final c in _refreshCompleter) {
          c.complete(true);
        }
        _refreshCompleter.clear();
        return _retry(error.requestOptions);
      } else {
        await _logout();
        return null;
      }
    } catch (_) {
      for (final c in _refreshCompleter) {
        c.complete(false);
      }
      _refreshCompleter.clear();
      await _logout();
      return null;
    } finally {
      _isRefreshing = false;
    }
  }

  Future<Response> _retry(RequestOptions requestOptions) async {
    final token = await _tokenStorage.getAccessToken();
    return _dio.request(
      requestOptions.path,
      data: requestOptions.data,
      queryParameters: requestOptions.queryParameters,
      options: Options(
        method: requestOptions.method,
        headers: {
          ...requestOptions.headers,
          'Authorization': 'Bearer $token',
        },
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

  Future<Response> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
  }) =>
      _dio.get(path, queryParameters: queryParameters, options: options);

  Future<Response> post(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
  }) =>
      _dio.post(path, data: data, queryParameters: queryParameters, options: options);

  Future<Response> patch(
    String path, {
    dynamic data,
    Options? options,
  }) =>
      _dio.patch(path, data: data, options: options);

  Future<Response> delete(
    String path, {
    Options? options,
  }) =>
      _dio.delete(path, options: options);
}
