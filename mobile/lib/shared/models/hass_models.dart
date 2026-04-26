class HassStatus {
  final String? baseUrl;
  final bool configured;
  final bool onboarded;
  final String? reason;

  const HassStatus({
    this.baseUrl,
    required this.configured,
    required this.onboarded,
    this.reason,
  });

  factory HassStatus.fromJson(Map<String, dynamic> json) => HassStatus(
        baseUrl: json['baseUrl'] as String?,
        configured: json['configured'] as bool? ?? false,
        onboarded: json['onboarded'] as bool? ?? false,
        reason: json['reason'] as String?,
      );
}

class HassFlowHandler {
  final String domain;
  final String name;
  final String? integration;
  final String? iotClass;
  final bool configFlow;

  const HassFlowHandler({
    required this.domain,
    required this.name,
    this.integration,
    this.iotClass,
    required this.configFlow,
  });

  factory HassFlowHandler.fromJson(Map<String, dynamic> json) =>
      HassFlowHandler(
        domain: json['domain'] as String,
        name: json['name'] as String,
        integration: json['integration'] as String?,
        iotClass: json['iot_class'] as String?,
        configFlow: json['config_flow'] as bool? ?? false,
      );
}

class HassSchemaField {
  final String name;
  final String type;
  final bool required;
  final bool optional;
  final dynamic defaultValue;
  final dynamic options;
  final bool multiple;

  const HassSchemaField({
    required this.name,
    required this.type,
    this.required = false,
    this.optional = false,
    this.defaultValue,
    this.options,
    this.multiple = false,
  });

  factory HassSchemaField.fromJson(Map<String, dynamic> json) =>
      HassSchemaField(
        name: json['name'] as String,
        type: json['type'] as String? ?? 'string',
        required: json['required'] as bool? ?? false,
        optional: json['optional'] as bool? ?? false,
        defaultValue: json['default'],
        options: json['options'],
        multiple: json['multiple'] as bool? ?? false,
      );

  // Normalize options to List<(value, label)>
  List<(String, String)> get normalizedOptions {
    if (options == null) return [];
    if (options is Map) {
      return (options as Map).entries
          .map((e) => (e.key.toString(), e.value.toString()))
          .toList();
    }
    if (options is List) {
      final list = options as List;
      if (list.isEmpty) return [];
      final first = list.first;
      if (first is List) {
        return list.map((e) {
          final pair = e as List;
          return (pair[0].toString(), pair[1].toString());
        }).toList();
      }
      if (first is Map) {
        return list.map((e) {
          final m = e as Map;
          final v = m['value']?.toString() ?? m.keys.first.toString();
          final l = m['label']?.toString() ?? v;
          return (v, l);
        }).toList();
      }
      return list.map((e) => (e.toString(), e.toString())).toList();
    }
    return [];
  }
}

class HassFlowStep {
  final String? flowId;
  final String? handler;
  final String type;
  final String? stepId;
  final List<HassSchemaField>? dataSchema;
  final Map<String, dynamic>? errors;
  final String? description;
  final Map<String, String>? descriptionPlaceholders;
  final String? reason;
  final String? title;
  final String? url;

  const HassFlowStep({
    this.flowId,
    this.handler,
    required this.type,
    this.stepId,
    this.dataSchema,
    this.errors,
    this.description,
    this.descriptionPlaceholders,
    this.reason,
    this.title,
    this.url,
  });

  factory HassFlowStep.fromJson(Map<String, dynamic> json) => HassFlowStep(
        flowId: json['flow_id'] as String?,
        handler: json['handler'] as String?,
        type: json['type'] as String,
        stepId: json['step_id'] as String?,
        dataSchema: (json['data_schema'] as List<dynamic>?)
            ?.map((e) => HassSchemaField.fromJson(e as Map<String, dynamic>))
            .toList(),
        errors: json['errors'] as Map<String, dynamic>?,
        description: json['description'] as String?,
        descriptionPlaceholders:
            (json['description_placeholders'] as Map<String, dynamic>?)
                ?.map((k, v) => MapEntry(k, v?.toString() ?? '')),
        reason: json['reason'] as String?,
        title: json['title'] as String?,
        url: json['url'] as String?,
      );
}

class HassEntity {
  final String entityId;
  final String state;
  final String domain;
  final String? friendlyName;
  final Map<String, dynamic>? attributes;

  const HassEntity({
    required this.entityId,
    required this.state,
    required this.domain,
    this.friendlyName,
    this.attributes,
  });

  factory HassEntity.fromJson(Map<String, dynamic> json) => HassEntity(
        entityId: json['entity_id'] as String,
        state: json['state'] as String,
        domain: json['domain'] as String,
        friendlyName: json['friendly_name'] as String?,
        attributes: json['attributes'] as Map<String, dynamic>?,
      );

  String get displayName => friendlyName ?? entityId;
}
