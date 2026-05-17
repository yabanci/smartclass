class TimePoint {
  // FA-4: backend sends RFC3339 time.Time strings; parse to DateTime so
  // callers can format with intl.DateFormat rather than using raw strings.
  final DateTime bucket;
  final double avg;
  final double min;
  final double max;
  final int count;

  const TimePoint({
    required this.bucket,
    required this.avg,
    required this.min,
    required this.max,
    required this.count,
  });

  factory TimePoint.fromJson(Map<String, dynamic> json) => TimePoint(
        bucket: DateTime.parse(json['bucket'] as String),
        avg: (json['avg'] as num).toDouble(),
        min: (json['min'] as num).toDouble(),
        max: (json['max'] as num).toDouble(),
        count: (json['count'] as num).toInt(),
      );
}
