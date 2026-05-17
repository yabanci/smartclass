class TimePoint {
  final String bucket;
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
        bucket: json['bucket'] as String,
        avg: (json['avg'] as num).toDouble(),
        min: (json['min'] as num).toDouble(),
        max: (json['max'] as num).toDouble(),
        count: (json['count'] as num).toInt(),
      );
}
