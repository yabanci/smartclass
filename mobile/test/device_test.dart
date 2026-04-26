import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:smartclass/features/devices/device_card.dart';
import 'package:smartclass/shared/models/device.dart';

Device _makeDevice({
  String id = 'dev-1',
  String type = 'light',
  String status = 'off',
  bool online = true,
}) {
  return Device(
    id: id,
    classroomId: 'cls-1',
    name: 'Test Light',
    type: type,
    brand: 'Shelly',
    driver: 'shelly',
    status: status,
    online: online,
    createdAt: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z',
  );
}

Widget _wrap(Widget child) {
  return ProviderScope(
    child: MaterialApp(home: Scaffold(body: child)),
  );
}

void main() {
  group('DeviceCard', () {
    testWidgets('renders device name and brand', (tester) async {
      final device = _makeDevice(online: true, status: 'on');

      await tester.pumpWidget(
        _wrap(DeviceCard(device: device, classroomId: 'cls-1')),
      );
      await tester.pumpAndSettle();

      expect(find.text('Test Light'), findsOneWidget);
      expect(find.text('Shelly'), findsOneWidget);
    });

    testWidgets('shows Online badge for online device', (tester) async {
      final device = _makeDevice(online: true);

      await tester.pumpWidget(
        _wrap(DeviceCard(device: device, classroomId: 'cls-1')),
      );
      await tester.pumpAndSettle();

      expect(find.text('Online'), findsOneWidget);
    });

    testWidgets('shows Offline badge for offline device', (tester) async {
      final device = _makeDevice(online: false);

      await tester.pumpWidget(
        _wrap(DeviceCard(device: device, classroomId: 'cls-1')),
      );
      await tester.pumpAndSettle();

      expect(find.text('Offline'), findsOneWidget);
    });

    testWidgets('switch is OFF when device status is off', (tester) async {
      final device = _makeDevice(status: 'off', online: true);

      await tester.pumpWidget(
        _wrap(DeviceCard(device: device, classroomId: 'cls-1')),
      );
      await tester.pumpAndSettle();

      final switchWidget =
          tester.widget<Switch>(find.byType(Switch));
      expect(switchWidget.value, isFalse);
    });

    testWidgets('switch is ON when device status is on', (tester) async {
      final device = _makeDevice(status: 'on', online: true);

      await tester.pumpWidget(
        _wrap(DeviceCard(device: device, classroomId: 'cls-1')),
      );
      await tester.pumpAndSettle();

      final switchWidget =
          tester.widget<Switch>(find.byType(Switch));
      expect(switchWidget.value, isTrue);
    });

    testWidgets('slider appears for light device when ON', (tester) async {
      final device = _makeDevice(
        type: 'light',
        status: 'on',
        online: true,
      );

      await tester.pumpWidget(
        _wrap(DeviceCard(device: device, classroomId: 'cls-1')),
      );
      await tester.pumpAndSettle();

      // Slider should be visible for light device that is ON
      expect(find.byType(Slider), findsOneWidget);
      expect(find.text('Brightness:'), findsNothing); // it's in label Text
    });

    testWidgets('slider not visible when device is OFF', (tester) async {
      final device = _makeDevice(
        type: 'light',
        status: 'off',
        online: true,
      );

      await tester.pumpWidget(
        _wrap(DeviceCard(device: device, classroomId: 'cls-1')),
      );
      await tester.pumpAndSettle();

      expect(find.byType(Slider), findsNothing);
    });

    testWidgets('deviceIcon returns lightbulb for light type', (tester) async {
      expect(deviceIcon('light'), Icons.lightbulb_outlined);
      expect(deviceIcon('ac'), Icons.ac_unit);
      expect(deviceIcon('fan'), Icons.air);
      expect(deviceIcon('cover'), Icons.blinds);
      expect(deviceIcon('sensor'), Icons.thermostat);
      expect(deviceIcon('switch'), Icons.power);
    });
  });
}
