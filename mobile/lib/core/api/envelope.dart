class ApiEnvelope<T> {
  final bool ok;
  final T? data;
  final String? error;

  const ApiEnvelope({required this.ok, this.data, this.error});

  factory ApiEnvelope.fromJson(
    Map<String, dynamic> json,
    T Function(dynamic) fromData,
  ) {
    final errorObj = json['error'];
    if (errorObj != null) {
      String? message;
      if (errorObj is Map<String, dynamic>) {
        message = errorObj['message'] as String?;
      } else if (errorObj is String) {
        message = errorObj;
      }
      return ApiEnvelope(ok: false, error: message ?? 'Unknown error');
    }
    return ApiEnvelope(ok: true, data: fromData(json['data']));
  }
}

class ApiException implements Exception {
  final String message;
  final int? statusCode;

  const ApiException(this.message, {this.statusCode});

  @override
  String toString() => 'ApiException($statusCode): $message';
}
