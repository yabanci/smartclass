class ApiEnvelope<T> {
  final bool ok;
  final T? data;
  final String? error;

  const ApiEnvelope({required this.ok, this.data, this.error});

  factory ApiEnvelope.fromJson(
    Map<String, dynamic> json,
    T Function(dynamic) fromData,
  ) {
    final ok = json['ok'] as bool? ?? false;
    if (!ok) {
      return ApiEnvelope(ok: false, error: json['error'] as String?);
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
