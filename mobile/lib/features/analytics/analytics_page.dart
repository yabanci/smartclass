import '../../core/utils/error_utils.dart';
import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/i18n/app_localizations.dart';
import '../../shared/providers/analytics_provider.dart';
import '../../shared/providers/classroom_provider.dart';
import '../../shared/providers/device_provider.dart';
import '../../shared/widgets/error_view.dart';
import '../../shared/widgets/loading_indicator.dart';

class AnalyticsPage extends ConsumerStatefulWidget {
  const AnalyticsPage({super.key});

  @override
  ConsumerState<AnalyticsPage> createState() => _AnalyticsPageState();
}

class _AnalyticsPageState extends ConsumerState<AnalyticsPage> {
  String _metric = 'temperature';
  String _bucket = 'hour';

  static const _metrics = ['temperature', 'humidity', 'co2', 'light'];
  static const _buckets = ['hour', 'day', 'week', 'month'];

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context)!;
    final classroom = ref.watch(activeClassroomProvider);

    return Scaffold(
      appBar: AppBar(title: Text(l.analyticsTitle)),
      body: classroom == null
          ? Center(child: Text(l.homeNoClassroom))
          : _AnalyticsBody(
              classroomId: classroom.id,
              metric: _metric,
              bucket: _bucket,
              onMetricChanged: (v) => setState(() => _metric = v),
              onBucketChanged: (v) => setState(() => _bucket = v),
              metrics: _metrics,
              buckets: _buckets,
            ),
    );
  }
}

class _AnalyticsBody extends ConsumerWidget {
  final String classroomId;
  final String metric;
  final String bucket;
  final ValueChanged<String> onMetricChanged;
  final ValueChanged<String> onBucketChanged;
  final List<String> metrics;
  final List<String> buckets;

  const _AnalyticsBody({
    required this.classroomId,
    required this.metric,
    required this.bucket,
    required this.onMetricChanged,
    required this.onBucketChanged,
    required this.metrics,
    required this.buckets,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context)!;
    final from =
        DateTime.now().subtract(const Duration(days: 7)).toIso8601String();
    final query = AnalyticsQuery(
      classroomId: classroomId,
      metric: metric,
      bucket: bucket,
      from: from,
    );
    final seriesAsync = ref.watch(sensorSeriesProvider(query));
    final usageAsync = ref.watch(deviceUsageProvider(classroomId));
    final energyAsync = ref.watch(energyProvider(classroomId));
    final devicesAsync = ref.watch(deviceListProvider(classroomId));

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        // Selectors
        Row(
          children: [
            Expanded(
              child: DropdownButtonFormField<String>(
                value: metric,
                decoration: InputDecoration(
                  labelText: l.analyticsMetric,
                  border: const OutlineInputBorder(),
                  isDense: true,
                ),
                items: metrics
                    .map((m) => DropdownMenuItem(value: m, child: Text(m)))
                    .toList(),
                onChanged: (v) => v != null ? onMetricChanged(v) : null,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: DropdownButtonFormField<String>(
                value: bucket,
                decoration: InputDecoration(
                  labelText: l.analyticsBucket,
                  border: const OutlineInputBorder(),
                  isDense: true,
                ),
                items: buckets
                    .map((b) => DropdownMenuItem(value: b, child: Text(b)))
                    .toList(),
                onChanged: (v) => v != null ? onBucketChanged(v) : null,
              ),
            ),
          ],
        ),
        const SizedBox(height: 16),
        Text(l.analyticsSensorsSeries,
            style: const TextStyle(fontWeight: FontWeight.bold)),
        const SizedBox(height: 8),
        seriesAsync.when(
          loading: () => const LoadingIndicator(),
          error: (e, _) =>
              ErrorView(message: friendlyError(e)),
          data: (points) {
            if (points.isEmpty) {
              return const Center(
                  child: Padding(
                padding: EdgeInsets.all(32),
                child:
                    Text('No data', style: TextStyle(color: Colors.grey)),
              ));
            }
            return SizedBox(
              height: 200,
              child: LineChart(
                LineChartData(
                  gridData: const FlGridData(show: true),
                  titlesData: const FlTitlesData(
                    leftTitles: AxisTitles(
                      sideTitles: SideTitles(showTitles: true, reservedSize: 40),
                    ),
                    bottomTitles: AxisTitles(
                      sideTitles: SideTitles(showTitles: false),
                    ),
                    topTitles: AxisTitles(
                      sideTitles: SideTitles(showTitles: false),
                    ),
                    rightTitles: AxisTitles(
                      sideTitles: SideTitles(showTitles: false),
                    ),
                  ),
                  borderData: FlBorderData(show: true),
                  lineBarsData: [
                    LineChartBarData(
                      spots: points
                          .asMap()
                          .entries
                          .map((e) => FlSpot(
                              e.key.toDouble(), e.value.avg))
                          .toList(),
                      isCurved: true,
                      color: Theme.of(context).colorScheme.primary,
                      barWidth: 2,
                      dotData: const FlDotData(show: false),
                    ),
                  ],
                ),
              ),
            );
          },
        ),
        const SizedBox(height: 24),
        Text('${l.analyticsDeviceUsage} (${l.analyticsLastWeek})',
            style: const TextStyle(fontWeight: FontWeight.bold)),
        const SizedBox(height: 8),
        usageAsync.when(
          loading: () => const LoadingIndicator(),
          error: (e, _) =>
              ErrorView(message: friendlyError(e)),
          data: (usages) {
            if (usages.isEmpty) {
              return const Center(
                  child: Padding(
                padding: EdgeInsets.all(16),
                child: Text('No usage data',
                    style: TextStyle(color: Colors.grey)),
              ));
            }
            final devices = devicesAsync.valueOrNull ?? [];
            return SizedBox(
              height: 200,
              child: BarChart(
                BarChartData(
                  gridData: const FlGridData(show: true),
                  titlesData: FlTitlesData(
                    bottomTitles: AxisTitles(
                      sideTitles: SideTitles(
                        showTitles: true,
                        reservedSize: 30,
                        getTitlesWidget: (value, meta) {
                          final idx = value.toInt();
                          if (idx >= 0 && idx < usages.length) {
                            final id = usages[idx].deviceId;
                            final name = devices
                                .where((d) => d.id == id)
                                .map((d) => d.name)
                                .firstOrNull ?? id.substring(0, 6);
                            return Padding(
                              padding: const EdgeInsets.only(top: 4),
                              child: Text(name,
                                  style: const TextStyle(fontSize: 9),
                                  overflow: TextOverflow.ellipsis),
                            );
                          }
                          return const SizedBox.shrink();
                        },
                      ),
                    ),
                    leftTitles: const AxisTitles(
                      sideTitles:
                          SideTitles(showTitles: true, reservedSize: 30),
                    ),
                    topTitles: const AxisTitles(
                        sideTitles: SideTitles(showTitles: false)),
                    rightTitles: const AxisTitles(
                        sideTitles: SideTitles(showTitles: false)),
                  ),
                  barGroups: usages
                      .asMap()
                      .entries
                      .map(
                        (e) => BarChartGroupData(
                          x: e.key,
                          barRods: [
                            BarChartRodData(
                              toY: e.value.commandCount.toDouble(),
                              color:
                                  Theme.of(context).colorScheme.secondary,
                            ),
                          ],
                        ),
                      )
                      .toList(),
                ),
              ),
            );
          },
        ),
        const SizedBox(height: 24),
        energyAsync.when(
          loading: () => const SizedBox.shrink(),
          error: (_, __) => const SizedBox.shrink(),
          data: (total) => Card(
            child: ListTile(
              leading: const Icon(Icons.bolt, color: Colors.amber),
              title: Text('${l.analyticsEnergy} (${l.analyticsLastWeek})'),
              trailing: Text(
                '${total.toStringAsFixed(2)} kWh',
                style: const TextStyle(
                    fontWeight: FontWeight.bold, fontSize: 16),
              ),
            ),
          ),
        ),
      ],
    );
  }
}
