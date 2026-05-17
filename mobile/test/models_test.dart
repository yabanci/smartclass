import 'package:flutter_test/flutter_test.dart';
import 'package:smartclass/core/api/envelope.dart';
import 'package:smartclass/core/utils/error_utils.dart';
import 'package:smartclass/shared/models/classroom.dart';
import 'package:smartclass/shared/models/device.dart';
import 'package:smartclass/shared/models/hass_models.dart';
import 'package:smartclass/shared/models/lesson.dart';
import 'package:smartclass/shared/models/notification.dart';
import 'package:smartclass/shared/models/scene.dart';
import 'package:smartclass/shared/models/sensor_reading.dart';
import 'package:smartclass/shared/models/time_point.dart';

void main() {
  // ─── Device ───────────────────────────────────────────────────────────────
  group('Device.fromJson', () {
    test('round-trips full device', () {
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
      final d = Device.fromJson(json);
      expect(d.id, 'dev-1');
      expect(d.isOn, isTrue);
      expect(d.online, isTrue);
      expect(d.config['brightness'], 80);
      expect(d.toJson()['id'], d.id);
    });

    test('fills defaults for missing optional fields', () {
      final d = Device.fromJson({
        'id': 'd',
        'classroomId': 'c',
        'name': 'Sensor',
        'type': 'sensor',
        'brand': 'Generic',
        'driver': 'generic',
        'createdAt': '',
        'updatedAt': '',
      });
      expect(d.status, 'unknown');
      expect(d.online, isFalse);
      expect(d.config, isEmpty);
      expect(d.lastSeenAt, isNull);
    });

    test('isOn true for status=on', () {
      final d = Device.fromJson({
        'id': 'd', 'classroomId': 'c', 'name': 'n',
        'type': 'switch', 'brand': 'b', 'driver': 'd',
        'status': 'on', 'online': false, 'createdAt': '', 'updatedAt': '',
      });
      expect(d.isOn, isTrue);
    });

    test('isOn true for status=open', () {
      final d = Device.fromJson({
        'id': 'd', 'classroomId': 'c', 'name': 'n',
        'type': 'cover', 'brand': 'b', 'driver': 'd',
        'status': 'open', 'online': true, 'createdAt': '', 'updatedAt': '',
      });
      expect(d.isOn, isTrue);
    });

    test('isOn false for status=unknown', () {
      final d = Device.fromJson({
        'id': 'd', 'classroomId': 'c', 'name': 'n',
        'type': 'switch', 'brand': 'b', 'driver': 'd',
        'status': 'unknown', 'online': false, 'createdAt': '', 'updatedAt': '',
      });
      expect(d.isOn, isFalse);
    });

    test('copyWith preserves unchanged fields', () {
      final d = Device.fromJson({
        'id': 'd', 'classroomId': 'c', 'name': 'Old',
        'type': 'light', 'brand': 'b', 'driver': 'd',
        'status': 'off', 'online': false, 'createdAt': '', 'updatedAt': '',
      });
      final d2 = d.copyWith(name: 'New');
      expect(d2.name, 'New');
      expect(d2.type, d.type);
      expect(d2.id, d.id);
    });
  });

  // ─── Classroom ────────────────────────────────────────────────────────────
  group('Classroom.fromJson', () {
    test('parses all fields', () {
      final c = Classroom.fromJson({
        'id': 'cls-1',
        'name': 'Room 101',
        'description': 'Test room',
        'createdBy': 'user-1',
        'createdAt': '2024-01-01T00:00:00Z',
        'updatedAt': '2024-01-01T00:00:00Z',
      });
      expect(c.id, 'cls-1');
      expect(c.name, 'Room 101');
      expect(c.description, 'Test room');
    });

    // C-009: description is non-nullable String; null from JSON maps to ''.
    test('maps null description to empty string', () {
      final c = Classroom.fromJson({
        'id': 'c', 'name': 'n', 'description': null,
        'createdBy': 'u', 'createdAt': '', 'updatedAt': '',
      });
      expect(c.description, '');
    });
  });

  // ─── Lesson ───────────────────────────────────────────────────────────────
  group('Lesson.fromJson', () {
    test('parses weekday lesson', () {
      final l = Lesson.fromJson({
        'id': 'l1',
        'classroomId': 'c1',
        'subject': 'Math',
        'dayOfWeek': 1,
        'startsAt': '09:00',
        'endsAt': '10:00',
        'notes': '',
        'createdAt': '',
        'updatedAt': '',
      });
      expect(l.subject, 'Math');
      expect(l.dayOfWeek, 1);
      expect(l.startsAt, '09:00');
    });
  });

  // ─── Scene ────────────────────────────────────────────────────────────────
  group('Scene.fromJson', () {
    test('parses steps correctly', () {
      final s = Scene.fromJson({
        'id': 's1',
        'classroomId': 'c1',
        'name': 'Morning',
        'description': 'Start of day',
        'steps': [
          {'deviceId': 'd1', 'command': 'ON'},
          {'deviceId': 'd2', 'command': 'SET_VALUE', 'value': 22},
        ],
        'createdAt': '',
        'updatedAt': '',
      });
      expect(s.steps.length, 2);
      expect(s.steps.first.command, 'ON');
      expect(s.steps.last.value, 22);
    });
  });

  // ─── SensorReading ────────────────────────────────────────────────────────
  group('SensorReading.fromJson', () {
    test('parses temperature reading', () {
      final r = SensorReading.fromJson({
        'deviceId': 'd1',
        'metric': 'temperature',
        'value': 23.5,
        'unit': 'C',
        'recordedAt': '2024-01-01T00:00:00Z',
      });
      expect(r.metric, 'temperature');
      expect(r.value, 23.5);
      expect(r.unit, 'C');
    });
  });

  // ─── AppNotification ──────────────────────────────────────────────────────
  group('AppNotification', () {
    test('isRead false when readAt is null', () {
      final n = AppNotification.fromJson({
        'id': 'n1', 'userId': 'u1', 'type': 'info',
        'title': 'Hi', 'message': 'There', 'createdAt': '',
      });
      expect(n.isRead, isFalse);
    });

    test('isRead true when readAt is set', () {
      final n = AppNotification.fromJson({
        'id': 'n1', 'userId': 'u1', 'type': 'warning',
        'title': 'Hi', 'message': 'There',
        'readAt': '2024-01-02T00:00:00Z', 'createdAt': '',
      });
      expect(n.isRead, isTrue);
    });

    test('parses all three types', () {
      for (final t in ['info', 'warning', 'error']) {
        final n = AppNotification.fromJson({
          'id': 'n', 'userId': 'u', 'type': t,
          'title': t, 'message': t, 'createdAt': '',
        });
        expect(n.type, t);
      }
    });
  });

  // ─── HassFlowStep ─────────────────────────────────────────────────────────
  group('HassFlowStep.fromJson', () {
    test('parses form step with data_schema', () {
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
      expect(step.dataSchema?.length, 2);
      expect(step.dataSchema?.first.name, 'username');
      expect(step.dataSchema?.first.required, isTrue);
    });

    test('parses create_entry', () {
      final step = HassFlowStep.fromJson({
        'flow_id': 'flow-2',
        'type': 'create_entry',
        'title': 'Done',
      });
      expect(step.type, 'create_entry');
      expect(step.title, 'Done');
    });

    test('parses abort with reason', () {
      final step = HassFlowStep.fromJson({
        'flow_id': 'flow-3',
        'type': 'abort',
        'reason': 'already_configured',
      });
      expect(step.reason, 'already_configured');
    });

    test('parses external_step with url', () {
      final step = HassFlowStep.fromJson({
        'flow_id': 'flow-4',
        'type': 'external_step',
        'url': 'https://auth.example.com/oauth',
      });
      expect(step.url, contains('oauth'));
    });

    test('safe cast for non-string description_placeholders', () {
      // HA sometimes sends non-string values in placeholders
      final step = HassFlowStep.fromJson({
        'flow_id': 'f',
        'type': 'form',
        'description_placeholders': {
          'count': 3,
          'link_left': '<a href="https://auth.example.com">',
        },
      });
      // Should not throw, and should coerce to string
      expect(step.descriptionPlaceholders?['count'], '3');
      expect(step.descriptionPlaceholders?['link_left'],
          contains('href'));
    });
  });

  // ─── HassSchemaField option normalization ─────────────────────────────────
  group('HassSchemaField.normalizedOptions', () {
    test('flat string array', () {
      final f = HassSchemaField.fromJson({
        'name': 'region', 'type': 'select',
        'options': ['cn', 'sg', 'us'],
      });
      expect(f.normalizedOptions.length, 3);
      expect(f.normalizedOptions.first.$1, 'cn');
      expect(f.normalizedOptions.first.$2, 'cn');
    });

    test('dict {"cn": "China"}', () {
      final f = HassSchemaField.fromJson({
        'name': 'server', 'type': 'select',
        'options': {'cn': 'China', 'sg': 'Singapore'},
      });
      final opts = f.normalizedOptions;
      expect(opts.any((o) => o.$1 == 'cn' && o.$2 == 'China'), isTrue);
    });

    test('array-of-pairs [["cn","China"]]', () {
      final f = HassSchemaField.fromJson({
        'name': 'zone', 'type': 'select',
        'options': [['cn', 'China'], ['sg', 'Singapore']],
      });
      expect(f.normalizedOptions.first.$1, 'cn');
      expect(f.normalizedOptions.first.$2, 'China');
    });

    test('array-of-objects [{value, label}]', () {
      final f = HassSchemaField.fromJson({
        'name': 'area', 'type': 'select',
        'options': [
          {'value': 'living_room', 'label': 'Living Room'},
        ],
      });
      expect(f.normalizedOptions.first.$1, 'living_room');
      expect(f.normalizedOptions.first.$2, 'Living Room');
    });

    test('returns empty for null options', () {
      final f = HassSchemaField.fromJson({'name': 'f', 'type': 'string'});
      expect(f.normalizedOptions, isEmpty);
    });
  });

  // ─── ApiEnvelope ──────────────────────────────────────────────────────────
  group('ApiEnvelope.fromJson', () {
    test('parses success response with data', () {
      final env = ApiEnvelope.fromJson(
        {'data': 'hello'},
        (d) => d as String,
      );
      expect(env.ok, isTrue);
      expect(env.data, 'hello');
      expect(env.error, isNull);
    });

    test('parses error response with error object', () {
      final env = ApiEnvelope.fromJson(
        {'error': {'code': 'not_found', 'message': 'Not found'}},
        (d) => d as String,
      );
      expect(env.ok, isFalse);
      expect(env.error, 'Not found');
    });

    test('parses error response with string error', () {
      final env = ApiEnvelope.fromJson(
        {'error': 'Something went wrong'},
        (d) => d as String,
      );
      expect(env.ok, isFalse);
      expect(env.error, 'Something went wrong');
    });

    test('treats missing error key as success', () {
      final env = ApiEnvelope.fromJson(
        {'data': 42},
        (d) => d as int,
      );
      expect(env.ok, isTrue);
      expect(env.data, 42);
    });
  });

  // ─── TimePoint ────────────────────────────────────────────────────────────
  group('TimePoint.fromJson', () {
    test('parses RFC3339 bucket string to DateTime', () {
      final tp = TimePoint.fromJson({
        'bucket': '2026-01-15T09:00:00Z',
        'avg': 22.5,
        'min': 20.0,
        'max': 25.0,
        'count': 12,
      });
      expect(tp.bucket, isA<DateTime>());
      expect(tp.bucket.year, 2026);
      expect(tp.bucket.month, 1);
      expect(tp.bucket.day, 15);
      expect(tp.avg, 22.5);
      expect(tp.min, 20.0);
      expect(tp.max, 25.0);
      expect(tp.count, 12);
    });

    test('parses RFC3339 with timezone offset', () {
      final tp = TimePoint.fromJson({
        'bucket': '2026-03-10T14:30:00+05:00',
        'avg': 1.0,
        'min': 0.5,
        'max': 1.5,
        'count': 3,
      });
      // DateTime.parse handles offset — result is a valid DateTime.
      expect(tp.bucket, isA<DateTime>());
      expect(tp.bucket.year, 2026);
    });

    test('throws FormatException for malformed bucket string', () {
      // DateTime.parse throws FormatException on invalid input.
      // Callers should treat this as a contract violation from the backend.
      expect(
        () => TimePoint.fromJson({
          'bucket': 'not-a-date',
          'avg': 1.0,
          'min': 0.0,
          'max': 2.0,
          'count': 1,
        }),
        throwsA(isA<FormatException>()),
      );
    });

    test('throws when bucket key is missing', () {
      // Missing 'bucket' → null → cast to String throws TypeError.
      expect(
        () => TimePoint.fromJson({
          'avg': 1.0,
          'min': 0.0,
          'max': 2.0,
          'count': 1,
        }),
        throwsA(isA<TypeError>()),
      );
    });
  });

  // ─── friendlyError ────────────────────────────────────────────────────────
  group('friendlyError', () {
    test('strips ApiException prefix', () {
      final msg = friendlyError(Exception('ApiException(200): Device offline'));
      expect(msg, isNot(contains('ApiException')));
    });

    test('returns plain message for generic exception', () {
      final msg = friendlyError(Exception('Something went wrong'));
      expect(msg, contains('Something went wrong'));
    });
  });
}
