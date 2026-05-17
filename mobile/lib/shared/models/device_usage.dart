class DeviceUsage {
  final String deviceId;
  final int commandCount;

  const DeviceUsage({
    required this.deviceId,
    required this.commandCount,
  });

  factory DeviceUsage.fromJson(Map<String, dynamic> json) => DeviceUsage(
        deviceId: json['deviceId'] as String,
        commandCount: (json['commandCount'] as num).toInt(),
      );
}
