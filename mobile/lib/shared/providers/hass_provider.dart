import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/endpoints/hass_endpoints.dart';
import '../models/hass_models.dart';
import 'auth_provider.dart';

final hassEndpointsProvider = Provider<HassEndpoints>(
  (ref) => HassEndpoints(ref.watch(apiClientProvider)),
);

final hassStatusProvider = FutureProvider<HassStatus>((ref) {
  return ref.watch(hassEndpointsProvider).status();
});

final hassIntegrationsProvider = FutureProvider<List<HassFlowHandler>>((ref) {
  return ref.watch(hassEndpointsProvider).integrations();
});

final hassEntitiesProvider = FutureProvider<List<HassEntity>>((ref) {
  return ref.watch(hassEndpointsProvider).entities();
});
