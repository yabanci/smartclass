class Device {
  final String id;
  final String classroomId;
  final String name;
  final String type;
  final String brand;
  final String driver;
  final Map<String, dynamic> config;
  final String status;
  final bool online;
  final String? lastSeenAt;
  final String createdAt;
  final String updatedAt;

  const Device({
    required this.id,
    required this.classroomId,
    required this.name,
    required this.type,
    required this.brand,
    required this.driver,
    this.config = const {},
    required this.status,
    required this.online,
    this.lastSeenAt,
    required this.createdAt,
    required this.updatedAt,
  });

  factory Device.fromJson(Map<String, dynamic> json) => Device(
        id: json['id'] as String,
        classroomId: json['classroomId'] as String,
        name: json['name'] as String,
        type: json['type'] as String,
        brand: json['brand'] as String,
        driver: json['driver'] as String,
        config: (json['config'] as Map<String, dynamic>?) ?? {},
        status: json['status'] as String? ?? 'unknown',
        online: json['online'] as bool? ?? false,
        lastSeenAt: json['lastSeenAt'] as String?,
        createdAt: json['createdAt'] as String,
        updatedAt: json['updatedAt'] as String,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'classroomId': classroomId,
        'name': name,
        'type': type,
        'brand': brand,
        'driver': driver,
        'config': config,
        'status': status,
        'online': online,
        if (lastSeenAt != null) 'lastSeenAt': lastSeenAt,
        'createdAt': createdAt,
        'updatedAt': updatedAt,
      };

  Device copyWith({
    String? id,
    String? classroomId,
    String? name,
    String? type,
    String? brand,
    String? driver,
    Map<String, dynamic>? config,
    String? status,
    bool? online,
    String? lastSeenAt,
    String? createdAt,
    String? updatedAt,
  }) =>
      Device(
        id: id ?? this.id,
        classroomId: classroomId ?? this.classroomId,
        name: name ?? this.name,
        type: type ?? this.type,
        brand: brand ?? this.brand,
        driver: driver ?? this.driver,
        config: config ?? this.config,
        status: status ?? this.status,
        online: online ?? this.online,
        lastSeenAt: lastSeenAt ?? this.lastSeenAt,
        createdAt: createdAt ?? this.createdAt,
        updatedAt: updatedAt ?? this.updatedAt,
      );

  bool get isOn => status == 'on' || status == 'open';
}
