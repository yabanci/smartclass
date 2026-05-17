import 'package:dio/dio.dart';

import '../api/envelope.dart';

/// Extracts a short, user-readable message from any exception.
/// Strips DioException boilerplate and raw stack info.
String friendlyError(Object e) {
  if (e is PartialFailureException) {
    return '${e.success}/${e.total} steps succeeded';
  }
  if (e is DioException) {
    final code = e.response?.statusCode;
    final body = e.response?.data;
    // Backend returns { error: { code, message } }
    if (body is Map) {
      final err = body['error'];
      if (err is Map) {
        final msg = err['message'];
        if (msg is String && msg.isNotEmpty) return msg;
      }
    }
    if (code != null) return 'Server error ($code). Try again.';
    if (e.type == DioExceptionType.connectionTimeout ||
        e.type == DioExceptionType.receiveTimeout) {
      return 'Connection timed out. Check network.';
    }
    if (e.type == DioExceptionType.connectionError) {
      return 'Cannot connect to server. Check network.';
    }
    return 'Network error. Try again.';
  }
  // ApiException(200): message  →  strip prefix
  final s = e.toString();
  final match = RegExp(r'ApiException\(\d+\):\s*(.+)').firstMatch(s);
  if (match != null) return match.group(1) ?? s;
  return s;
}
