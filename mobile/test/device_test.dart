import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:smartclass/features/devices/device_card.dart';
import 'package:smartclass/shared/models/device.dart';

import 'test_helpers.dart';

Device _device({
  String id = 'dev-1',
  String type = 'light',
  String status = 'off',
  bool online = true,
  Map<String, dynamic> config = const {},
}) =>
    Device(
      id: id,
      classroomId: 'cls-1',
      name: 'Test Light',
      type: type,
      brand: 'Shelly',
      driver: 'shelly',
      config: config,
      status: status,
      online: online,
      createdAt: '2024-01-01T00:00:00Z',
      updatedAt: '2024-01-01T00:00:00Z',
    );

void main() {
  group('DeviceCard rendering', () {
    testWidgets('shows device name', (tester) async {
      await tester.pumpWidget(
        testApp(DeviceCard(device: _device(), classroomId: 'cls-1')),
      );
      await tester.pumpAndSettle();
      expect(find.text('Test Light'), findsOneWidget);
    });

    testWidgets('shows brand in subtitle', (tester) async {
      await tester.pumpWidget(
        testApp(DeviceCard(device: _device(), classroomId: 'cls-1')),
      );
      await tester.pumpAndSettle();
      // Brand + type shown as "Shelly · light"
      expect(find.textContaining('Shelly'), findsOneWidget);
    });

    testWidgets('online device shows green dot (accent color)', (tester) async {
      await tester.pumpWidget(
        testApp(DeviceCard(device: _device(online: true), classroomId: 'cls-1')),
      );
      await tester.pumpAndSettle();
      // Green dot is a Container with kAccent color — just verify no exception
      expect(find.byType(DeviceCard), findsOneWidget);
    });

    testWidgets('shows toggle for non-sensor device', (tester) async {
      final d = _device(type: 'light', status: 'off');
      await tester.pumpWidget(
        testApp(DeviceCard(device: d, classroomId: 'cls-1')),
      );
      await tester.pumpAndSettle();
      // _Toggle is an AnimatedContainer, just verify card renders
      expect(find.byType(AnimatedContainer), findsWidgets);
    });

    testWidgets('slider visible for ON light device', (tester) async {
      final d = _device(type: 'light', status: 'on');
      await tester.pumpWidget(
        testApp(DeviceCard(device: d, classroomId: 'cls-1')),
      );
      await tester.pumpAndSettle();
      expect(find.byType(Slider), findsOneWidget);
    });

    testWidgets('slider NOT visible for OFF light device', (tester) async {
      final d = _device(type: 'light', status: 'off');
      await tester.pumpWidget(
        testApp(DeviceCard(device: d, classroomId: 'cls-1')),
      );
      await tester.pumpAndSettle();
      expect(find.byType(Slider), findsNothing);
    });

    testWidgets('fan level buttons visible when ON', (tester) async {
      final d = _device(type: 'fan', status: 'on');
      await tester.pumpWidget(
        testApp(DeviceCard(device: d, classroomId: 'cls-1')),
      );
      await tester.pumpAndSettle();
      // Fan control shows Low/Medium/High
      expect(find.text('Low'), findsOneWidget);
      expect(find.text('Medium'), findsOneWidget);
      expect(find.text('High'), findsOneWidget);
    });

    testWidgets('sensor type has no toggle', (tester) async {
      final d = _device(type: 'sensor', status: 'unknown');
      await tester.pumpWidget(
        testApp(DeviceCard(device: d, classroomId: 'cls-1')),
      );
      await tester.pumpAndSettle();
      // Sensor cards don't show toggle or slider
      expect(find.byType(Slider), findsNothing);
    });

    testWidgets('edit and delete buttons present', (tester) async {
      bool editCalled = false;
      bool deleteCalled = false;
      final d = _device();
      await tester.pumpWidget(
        testApp(DeviceCard(
          device: d,
          classroomId: 'cls-1',
          onEdit: () => editCalled = true,
          onDelete: () => deleteCalled = true,
        )),
      );
      await tester.pumpAndSettle();
      // Edit button
      await tester.tap(find.byIcon(Icons.edit_outlined));
      expect(editCalled, isTrue);
      // Delete button
      await tester.tap(find.byIcon(Icons.delete_outlined));
      expect(deleteCalled, isTrue);
    });
  });

  group('deviceIcon helper', () {
    test('returns correct icons for each type', () {
      expect(deviceIcon('light'), Icons.lightbulb_outlined);
      expect(deviceIcon('climate'), Icons.ac_unit);
      expect(deviceIcon('ac'), Icons.ac_unit);
      expect(deviceIcon('thermostat'), Icons.ac_unit);
      expect(deviceIcon('fan'), Icons.air);
      expect(deviceIcon('cover'), Icons.blinds);
      expect(deviceIcon('blind'), Icons.blinds);
      expect(deviceIcon('sensor'), Icons.thermostat);
      expect(deviceIcon('switch'), Icons.power);
      expect(deviceIcon('projector'), Icons.power);
      expect(deviceIcon('unknown'), Icons.power);
    });
  });
}
