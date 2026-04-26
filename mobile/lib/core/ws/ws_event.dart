class WsEvent {
  final String topic;
  final String type;
  final Map<String, dynamic> payload;

  const WsEvent({
    required this.topic,
    required this.type,
    required this.payload,
  });

  factory WsEvent.fromJson(Map<String, dynamic> json) => WsEvent(
        topic: json['topic'] as String? ?? '',
        type: json['type'] as String? ?? '',
        payload: json['payload'] as Map<String, dynamic>? ?? {},
      );
}
