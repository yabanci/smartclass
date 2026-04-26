import 'package:flutter_test/flutter_test.dart';
import 'package:smartclass/shared/models/device.dart';
import 'package:smartclass/shared/models/notification.dart';
import 'package:smartclass/shared/models/hass_models.dart';

void main() {
  group('Device.fromJson', () {
    test('round-trips a full device', () {
      final json = {
        'id': 'dev-1',
        'classroomId': 'cls-1',
        'name': 'Main Light',
        'type': 'light',
        'brand': 'Shelly',
        'driver': 'shelly',
        'config': {'brightness': 80},
        'status': 'on',
        'online': true,
        'lastSeenAt': '2024-01-01T00:00:00Z',
        'createdAt': '2024-01-01T00:00:00Z',
        'updatedAt': '2024-01-01T00:00:00Z',
      };

      final device = Device.fromJson(json);

      expect(device.id, 'dev-1');
      expect(device.name, 'Main Light');
      expect(device.type, 'light');
      expect(device.online, isTrue);
      expect(device.isOn, isTrue);
      expect(device.config['brightness'], 80);

      final back = device.toJson();
      expect(back['id'], device.id);
      expect(back['status'], device.status);
    });

    test('handles missing optional fields gracefully', () {
      final json = {
        'id': 'dev-2',
        'classroomId': 'cls-1',
        'name': 'Sensor',
        'type': 'sensor',
        'brand': 'Generic',
        'driver': 'generic',
        'createdAt': '2024-01-01T00:00:00Z',
        'updatedAt': '2024-01-01T00:00:00Z',
      };

      final device = Device.fromJson(json);

      expect(device.status, 'unknown');
      expect(device.online, isFalse);
      expect(device.config, isEmpty);
      expect(device.lastSeenAt, isNull);
    });

    test('isOn returns true for "on" status', () {
      final device = Device.fromJson({
        'id': 'd',
        'classroomId': 'c',
        'name': 'n',
        'type': 'switch',
        'brand': 'b',
        'driver': 'd',
        'status': 'on',
        'online': false,
        'createdAt': '',
        'updatedAt': '',
      });
      expect(device.isOn, isTrue);
    });

    test('isOn returns true for "open" status', () {
      final device = Device.fromJson({
        'id': 'd',
        'classroomId': 'c',
        'name': 'n',
        'type': 'cover',
        'brand': 'b',
        'driver': 'd',
        'status': 'open',
        'online': true,
        'createdAt': '',
        'updatedAt': '',
      });
      expect(device.isOn, isTrue);
    });

    test('isOn returns false for offline device with unknown status', () {
      final device = Device.fromJson({
        'id': 'd',
        'classroomId': 'c',
        'name': 'n',
        'type': 'switch',
        'brand': 'b',
        'driver': 'd',
        'status': 'unknown',
        'online': false,
        'createdAt': '',
        'updatedAt': '',
      });
      expect(device.isOn, isFalse);
      expect(device.online, isFalse);
    });
  });

  group('AppNotification type parsing', () {
    test('parses info type', () {
      final n = AppNotification.fromJson({
        'id': 'n1',
        'userId': 'u1',
        'type': 'info',
        'title': 'Hello',
        'message': 'World',
        'createdAt': '2024-01-01T00:00:00Z',
      });
      expect(n.type, 'info');
      expect(n.isRead, isFalse);
    });

    test('parses warning type', () {
      final n = AppNotification.fromJson({
        'id': 'n2',
        'userId': 'u1',
        'type': 'warning',
        'title': 'Warning',
        'message': 'Check device',
        'createdAt': '2024-01-01T00:00:00Z',
      });
      expect(n.type, 'warning');
    });

    test('parses error type', () {
      final n = AppNotification.fromJson({
        'id': 'n3',
        'userId': 'u1',
        'type': 'error',
        'title': 'Error',
        'message': 'Device offline',
        'readAt': '2024-01-02T00:00:00Z',
        'createdAt': '2024-01-01T00:00:00Z',
      });
      expect(n.type, 'error');
      expect(n.isRead, isTrue);
    });
  });

  group('HassFlowStep type parsing', () {
    test('parses form step', () {
      final step = HassFlowStep.fromJson({
        'flow_id': 'flow-1',
        'type': 'form',
        'step_id': 'user',
        'data_schema': [
          {'name': 'username', 'type': 'string', 'required': true},
          {'name': 'password', 'type': 'string', 'required': true},
        ],
      });
      expect(step.type, 'form');
      expect(step.flowId, 'flow-1');
      expect(step.dataSchema?.length, 2);
    });

    test('parses create_entry step', () {
      final step = HassFlowStep.fromJson({
        'flow_id': 'flow-2',
        'type': 'create_entry',
        'title': 'Integration added',
      });
      expect(step.type, 'create_entry');
      expect(step.title, 'Integration added');
    });

    test('parses abort step', () {
      final step = HassFlowStep.fromJson({
        'flow_id': 'flow-3',
        'type': 'abort',
        'reason': 'already_configured',
      });
      expect(step.type, 'abort');
      expect(step.reason, 'already_configured');
    });

    test('parses external_step (oauth)', () {
      final step = HassFlowStep.fromJson({
        'flow_id': 'flow-4',
        'type': 'external_step',
        'url': 'https://auth.example.com/oauth',
      });
      expect(step.type, 'external_step');
      expect(step.url, contains('oauth'));
    });
  });

  group('HassSchemaField option normalization', () {
    test('normalizes flat string array', () {
      final field = HassSchemaField.fromJson({
        'name': 'region',
        'type': 'select',
        'options': ['cn', 'sg', 'us'],
      });
      final opts = field.normalizedOptions;
      expect(opts.length, 3);
      expect(opts.first.$1, 'cn');
      expect(opts.first.$2, 'cn');
    });

    test('normalizes dict options', () {
      final field = HassSchemaField.fromJson({
        'name': 'server',
        'type': 'select',
        'options': {'cn': 'China', 'sg': 'Singapore'},
      });
      final opts = field.normalizedOptions;
      expect(opts.any((o) => o.$1 == 'cn' && o.$2 == 'China'), isTrue);
    });

    test('normalizes array-of-pairs options', () {
      final field = HassSchemaField.fromJson({
        'name': 'zone',
        'type': 'select',
        'options': [
          ['cn', 'China'],
          ['sg', 'Singapore'],
        ],
      });
      final opts = field.normalizedOptions;
      expect(opts.length, 2);
      expect(opts.first.$1, 'cn');
      expect(opts.first.$2, 'China');
    });

    test('normalizes array-of-objects options', () {
      final field = HassSchemaField.fromJson({
        'name': 'area',
        'type': 'select',
        'options': [
          {'value': 'living_room', 'label': 'Living Room'},
          {'value': 'bedroom', 'label': 'Bedroom'},
        ],
      });
      final opts = field.normalizedOptions;
      expect(opts.first.$1, 'living_room');
      expect(opts.first.$2, 'Living Room');
    });
  });
}
