import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/i18n/app_localizations.dart';
import '../../shared/models/hass_models.dart';
import '../../shared/providers/classroom_provider.dart';
import '../../shared/providers/hass_provider.dart';
import '../../shared/widgets/error_view.dart';
import '../../shared/widgets/loading_indicator.dart';
import 'brand_picker.dart';
import 'wizard_step_view.dart';

enum _WizardState {
  home,
  pickBrand,
  brandHint,
  pickIntegration,
  wizard,
  entities,
  done,
}

class IotWizardPage extends ConsumerStatefulWidget {
  const IotWizardPage({super.key});

  @override
  ConsumerState<IotWizardPage> createState() => _IotWizardPageState();
}

class _IotWizardPageState extends ConsumerState<IotWizardPage> {
  _WizardState _state = _WizardState.home;
  BrandInfo? _selectedBrand;
  HassFlowHandler? _selectedHandler;
  HassFlowStep? _currentStep;
  String? _flowId;
  bool _loading = false;
  String? _error;
  String _searchQuery = '';
  final _tokenCtrl = TextEditingController();

  @override
  void dispose() {
    _tokenCtrl.dispose();
    super.dispose();
  }

  Future<void> _saveToken() async {
    if (_tokenCtrl.text.isEmpty) return;
    setState(() => _loading = true);
    try {
      await ref
          .read(hassEndpointsProvider)
          .saveToken(_tokenCtrl.text.trim());
      ref.invalidate(hassStatusProvider);
      setState(() => _state = _WizardState.pickBrand);
    } catch (e) {
      setState(() => _error = e.toString());
    } finally {
      setState(() => _loading = false);
    }
  }

  Future<void> _startFlow(String handler) async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final step =
          await ref.read(hassEndpointsProvider).startFlow(handler);
      _flowId = step.flowId;
      setState(() {
        _currentStep = step;
        _state = _WizardState.wizard;
      });
    } catch (e) {
      setState(() => _error = e.toString());
    } finally {
      setState(() => _loading = false);
    }
  }

  Future<void> _submitStep(Map<String, dynamic> data) async {
    final flowId = _flowId;
    if (flowId == null) return;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final step =
          await ref.read(hassEndpointsProvider).submitStep(flowId, data);
      if (step.type == 'create_entry') {
        // Move to entities list
        ref.invalidate(hassEntitiesProvider);
        setState(() {
          _state = _WizardState.entities;
          _currentStep = null;
          _flowId = null;
        });
      } else {
        setState(() => _currentStep = step);
      }
    } catch (e) {
      setState(() => _error = e.toString());
    } finally {
      setState(() => _loading = false);
    }
  }

  Future<void> _abortFlow() async {
    final flowId = _flowId;
    if (flowId != null) {
      try {
        await ref.read(hassEndpointsProvider).deleteFlow(flowId);
      } catch (_) {}
      _flowId = null;
    }
    setState(() {
      _currentStep = null;
      _state = _WizardState.pickBrand;
    });
  }

  Future<void> _adoptEntity(HassEntity entity) async {
    final l = AppLocalizations.of(context)!;
    final classroom = ref.read(activeClassroomProvider);
    if (classroom == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l.homeNoClassroom)),
      );
      return;
    }
    setState(() => _loading = true);
    try {
      await ref.read(hassEndpointsProvider).adopt(
            entityId: entity.entityId,
            classroomId: classroom.id,
            name: entity.friendlyName,
            brand: _selectedBrand?.name,
          );
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              entity.online
                  ? l.hassVerifyOk(entity.state)
                  : l.hassVerifyOffline,
            ),
            backgroundColor:
                entity.online ? Colors.green : Colors.orange,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(e.toString())));
      }
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context)!;
    return Scaffold(
      appBar: AppBar(
        title: Text(l.hassTitle),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () {
            if (_state == _WizardState.home ||
                _state == _WizardState.pickBrand ||
                _state == _WizardState.done) {
              Navigator.of(context).pop();
            } else if (_state == _WizardState.wizard) {
              _abortFlow();
            } else {
              setState(() => _state = _WizardState.pickBrand);
            }
          },
        ),
      ),
      body: _buildBody(),
    );
  }

  Widget _buildBody() {
    return ref.watch(hassStatusProvider).when(
      loading: () => const LoadingIndicator(),
      error: (e, _) => ErrorView(message: e.toString()),
      data: (status) {
        if (!status.configured || !status.onboarded) {
          return _buildTokenSetup(status);
        }
        return _buildWizardContent();
      },
    );
  }

  Widget _buildTokenSetup(HassStatus status) {
    final l = AppLocalizations.of(context)!;
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const Icon(Icons.home, size: 64, color: Colors.blue),
          const SizedBox(height: 16),
          Text(
            l.hassTitle,
            style: const TextStyle(fontSize: 20, fontWeight: FontWeight.bold),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 8),
          if (status.reason != null)
            Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: Text(
                status.reason!,
                style: const TextStyle(color: Colors.orange),
                textAlign: TextAlign.center,
              ),
            ),
          Text(
            l.hassAlreadySetup,
            textAlign: TextAlign.center,
            style: const TextStyle(color: Colors.grey),
          ),
          const SizedBox(height: 16),
          TextField(
            controller: _tokenCtrl,
            obscureText: true,
            decoration: InputDecoration(
              labelText: l.hassSaveToken,
              border: const OutlineInputBorder(),
              prefixIcon: const Icon(Icons.key_outlined),
            ),
          ),
          const SizedBox(height: 16),
          if (_error != null)
            Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: Text(_error!,
                  style: const TextStyle(color: Colors.red)),
            ),
          FilledButton(
            onPressed: _loading ? null : _saveToken,
            child: _loading
                ? const CircularProgressIndicator(strokeWidth: 2)
                : Text(l.hassSaveToken),
          ),
        ],
      ),
    );
  }

  Widget _buildWizardContent() {
    final l = AppLocalizations.of(context)!;
    switch (_state) {
      case _WizardState.home:
      case _WizardState.pickBrand:
        return SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: BrandPickerWidget(
            onBrandSelected: (brand) {
              setState(() {
                _selectedBrand = brand;
                _state = _WizardState.brandHint;
              });
            },
            onShowAll: () =>
                setState(() => _state = _WizardState.pickIntegration),
          ),
        );

      case _WizardState.brandHint:
        final brand = _selectedBrand!;
        return SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(brand.emoji,
                  textAlign: TextAlign.center,
                  style: const TextStyle(fontSize: 64)),
              const SizedBox(height: 12),
              Text(brand.name,
                  textAlign: TextAlign.center,
                  style: const TextStyle(
                      fontSize: 20, fontWeight: FontWeight.bold)),
              const SizedBox(height: 16),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Text(
                    brand.pairing == 'cloud'
                        ? l.hassCloudHint
                        : l.hassLanHint,
                  ),
                ),
              ),
              const SizedBox(height: 16),
              if (_error != null)
                Padding(
                  padding: const EdgeInsets.only(bottom: 12),
                  child: Text(_error!,
                      style: const TextStyle(color: Colors.red)),
                ),
              _IntegrationPicker(
                brand: brand,
                onSelected: (handler) => _startFlow(handler.domain),
                loading: _loading,
              ),
              const SizedBox(height: 8),
              TextButton(
                onPressed: () => setState(() => _state = _WizardState.pickBrand),
                child: Text(l.hassPickBrand),
              ),
            ],
          ),
        );

      case _WizardState.pickIntegration:
        return _AllIntegrationsPicker(
          searchQuery: _searchQuery,
          onSearchChanged: (q) => setState(() => _searchQuery = q),
          onSelected: (handler) => _startFlow(handler.domain),
          loading: _loading,
        );

      case _WizardState.wizard:
        final step = _currentStep;
        if (step == null) return const LoadingIndicator();
        return SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (step.title != null) ...[
                Text(step.title!,
                    style: const TextStyle(
                        fontSize: 18, fontWeight: FontWeight.bold)),
                const SizedBox(height: 16),
              ],
              WizardStepView(
                step: step,
                loading: _loading,
                onSubmit: _submitStep,
                onOauthDone: () => _submitStep({}),
              ),
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(_error!,
                    style: const TextStyle(color: Colors.red)),
              ],
              const SizedBox(height: 12),
              TextButton(
                onPressed: _abortFlow,
                child: Text(l.hassAbort),
              ),
            ],
          ),
        );

      case _WizardState.entities:
        return _EntitiesView(
          brand: _selectedBrand,
          onAdopt: _adoptEntity,
          loading: _loading,
        );

      case _WizardState.done:
        return Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.check_circle, color: Colors.green, size: 64),
              const SizedBox(height: 16),
              Text(l.hassCreated,
                  style: const TextStyle(
                      fontSize: 20, fontWeight: FontWeight.bold)),
            ],
          ),
        );
    }
  }
}

class _IntegrationPicker extends ConsumerWidget {
  final BrandInfo brand;
  final ValueChanged<HassFlowHandler> onSelected;
  final bool loading;

  const _IntegrationPicker({
    required this.brand,
    required this.onSelected,
    required this.loading,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context)!;
    return ref.watch(hassIntegrationsProvider).when(
      loading: () => const LoadingIndicator(),
      error: (e, _) => ErrorView(message: e.toString()),
      data: (handlers) {
        final matching = handlers
            .where((h) => brand.domains.contains(h.domain))
            .toList();

        if (matching.isEmpty) {
          return Column(
            children: [
              Text(
                l.hassBrandNotAvailable,
                style: const TextStyle(color: Colors.grey),
              ),
              const SizedBox(height: 8),
              OutlinedButton(
                onPressed: () => onSelected(HassFlowHandler(
                  domain: brand.domains.first,
                  name: brand.name,
                  configFlow: true,
                )),
                child: Text('Try ${brand.domains.first} anyway'),
              ),
            ],
          );
        }

        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: matching
              .map((h) => Card(
                    margin: const EdgeInsets.only(bottom: 8),
                    child: ListTile(
                      title: Text(h.name),
                      subtitle: h.iotClass != null
                          ? Text(h.iotClass!)
                          : null,
                      trailing: loading
                          ? const SizedBox(
                              width: 20,
                              height: 20,
                              child:
                                  CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Icon(Icons.arrow_forward),
                      onTap: loading ? null : () => onSelected(h),
                    ),
                  ))
              .toList(),
        );
      },
    );
  }
}

class _AllIntegrationsPicker extends ConsumerWidget {
  final String searchQuery;
  final ValueChanged<String> onSearchChanged;
  final ValueChanged<HassFlowHandler> onSelected;
  final bool loading;

  const _AllIntegrationsPicker({
    required this.searchQuery,
    required this.onSearchChanged,
    required this.onSelected,
    required this.loading,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context)!;
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.all(16),
          child: TextField(
            decoration: InputDecoration(
              prefixIcon: const Icon(Icons.search),
              hintText: l.hassSearchIntegration,
              border: const OutlineInputBorder(),
              isDense: true,
            ),
            onChanged: onSearchChanged,
          ),
        ),
        Expanded(
          child: ref.watch(hassIntegrationsProvider).when(
            loading: () => const LoadingIndicator(),
            error: (e, _) => ErrorView(message: e.toString()),
            data: (handlers) {
              final filtered = searchQuery.isEmpty
                  ? handlers
                  : handlers
                      .where((h) =>
                          h.name
                              .toLowerCase()
                              .contains(searchQuery.toLowerCase()) ||
                          h.domain
                              .toLowerCase()
                              .contains(searchQuery.toLowerCase()))
                      .toList();

              return ListView.separated(
                itemCount: filtered.length,
                separatorBuilder: (_, __) =>
                    const Divider(height: 1),
                itemBuilder: (context, i) {
                  final h = filtered[i];
                  return ListTile(
                    title: Text(h.name),
                    subtitle: Text(h.domain),
                    trailing: loading
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(
                                strokeWidth: 2),
                          )
                        : const Icon(Icons.arrow_forward),
                    onTap: loading ? null : () => onSelected(h),
                  );
                },
              );
            },
          ),
        ),
      ],
    );
  }
}

class _EntitiesView extends ConsumerWidget {
  final BrandInfo? brand;
  final ValueChanged<HassEntity> onAdopt;
  final bool loading;

  const _EntitiesView({
    required this.brand,
    required this.onAdopt,
    required this.loading,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context)!;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.all(16),
          child: Text(
            l.hassDiscoveredEntities,
            style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.bold,
                ),
          ),
        ),
        Expanded(
          child: ref.watch(hassEntitiesProvider).when(
            loading: () => const LoadingIndicator(),
            error: (e, _) => ErrorView(message: e.toString()),
            data: (entities) {
              if (entities.isEmpty) {
                return Center(
                  child: Padding(
                    padding: const EdgeInsets.all(32),
                    child: Text(
                      l.hassNoEntities,
                      style: const TextStyle(color: Colors.grey),
                      textAlign: TextAlign.center,
                    ),
                  ),
                );
              }

              return ListView.separated(
                padding: const EdgeInsets.symmetric(horizontal: 16),
                itemCount: entities.length,
                separatorBuilder: (_, __) =>
                    const SizedBox(height: 8),
                itemBuilder: (context, i) {
                  final entity = entities[i];
                  return Card(
                    child: ListTile(
                      leading: CircleAvatar(
                        backgroundColor: entity.online
                            ? Colors.green.withOpacity(0.2)
                            : Colors.grey.withOpacity(0.2),
                        child: Icon(
                          entity.online
                              ? Icons.check_circle_outline
                              : Icons.circle_outlined,
                          color: entity.online
                              ? Colors.green
                              : Colors.grey,
                          size: 20,
                        ),
                      ),
                      title: Text(entity.displayName),
                      subtitle: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(entity.entityId,
                              style:
                                  const TextStyle(fontSize: 11)),
                          Text(
                              'State: ${entity.state}',
                              style: const TextStyle(
                                  fontSize: 11,
                                  color: Colors.grey)),
                        ],
                      ),
                      trailing: loading
                          ? const SizedBox(
                              width: 20,
                              height: 20,
                              child: CircularProgressIndicator(
                                  strokeWidth: 2),
                            )
                          : FilledButton.tonal(
                              onPressed: () => onAdopt(entity),
                              child: Text(l.hassAddToClassroom,
                                  style: const TextStyle(fontSize: 12)),
                            ),
                      isThreeLine: true,
                    ),
                  );
                },
              );
            },
          ),
        ),
      ],
    );
  }
}

extension _HassEntityExt on HassEntity {
  bool get online => state != 'unavailable' && state != 'unknown';
}
