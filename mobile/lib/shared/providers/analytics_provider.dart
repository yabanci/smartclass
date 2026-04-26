import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/endpoints/analytics_endpoints.dart';
import '../models/time_point.dart';
import '../models/device_usage.dart';
import 'auth_provider.dart';

final analyticsEndpointsProvider = Provider<AnalyticsEndpoints>(
  (ref) => AnalyticsEndpoints(ref.watch(apiClientProvider)),
);

class AnalyticsQuery {
  final String classroomId;
  final String metric;
  final String bucket;
  final String? from;
  final String? to;

  const AnalyticsQuery({
    required this.classroomId,
    required this.metric,
    this.bucket = 'hour',
    this.from,
    this.to,
  });
}

final sensorSeriesProvider =
    FutureProvider.family<List<TimePoint>, AnalyticsQuery>((ref, query) {
  return ref.watch(analyticsEndpointsProvider).sensors(
        classroomId: query.classroomId,
        metric: query.metric,
        bucket: query.bucket,
        from: query.from,
        to: query.to,
      );
});

final deviceUsageProvider =
    FutureProvider.family<List<DeviceUsage>, String>((ref, classroomId) {
  final from =
      DateTime.now().subtract(const Duration(days: 7)).toIso8601String();
  return ref.watch(analyticsEndpointsProvider).usage(
        classroomId: classroomId,
        from: from,
      );
});

final energyProvider = FutureProvider.family<double, String>((ref, classroomId) {
  final from =
      DateTime.now().subtract(const Duration(days: 7)).toIso8601String();
  return ref.watch(analyticsEndpointsProvider).energy(
        classroomId: classroomId,
        from: from,
      );
});
