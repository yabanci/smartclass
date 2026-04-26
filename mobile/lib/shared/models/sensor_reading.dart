class SensorReading {
  final String? id;
  final String deviceId;
  final String metric;
  final double value;
  final String? unit;
  final String recordedAt;

  const SensorReading({
    this.id,
    required this.deviceId,
    required this.metric,
    required this.value,
    this.unit,
    required this.recordedAt,
  });

  factory SensorReading.fromJson(Map<String, dynamic> json) => SensorReading(
        id: json['id'] as String?,
        deviceId: json['deviceId'] as String,
        metric: json['metric'] as String,
        value: (json['value'] as num).toDouble(),
        unit: json['unit'] as String?,
        recordedAt: json['recordedAt'] as String,
      );

  Map<String, dynamic> toJson() => {
        if (id != null) 'id': id,
        'deviceId': deviceId,
        'metric': metric,
        'value': value,
        if (unit != null) 'unit': unit,
        'recordedAt': recordedAt,
      };
}
