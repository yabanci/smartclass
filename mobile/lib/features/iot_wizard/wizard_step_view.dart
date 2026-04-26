import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../shared/models/hass_models.dart';

/// HA sends OAuth URL wrapped in HTML inside description_placeholders,
/// e.g. {"link_left": "<a href='https://...'>", "link_right": "</a>"}.
/// Parse any href attribute out of all placeholder values.
String? _extractOAuthHref(Map<String, String>? placeholders) {
  if (placeholders == null) return null;
  // Also check direct 'url' key first
  if (placeholders['url'] != null) return placeholders['url'];
  final hrefRe = RegExp(r'''href\s*=\s*["']([^"']+)["']''', caseSensitive: false);
  for (final v in placeholders.values) {
    final m = hrefRe.firstMatch(v);
    if (m != null) return m.group(1);
  }
  return null;
}

/// Dynamic form renderer for HassFlowStep data_schema.
class WizardStepView extends StatefulWidget {
  final HassFlowStep step;
  final ValueChanged<Map<String, dynamic>> onSubmit;
  final VoidCallback? onOauthDone;
  final bool loading;

  const WizardStepView({
    super.key,
    required this.step,
    required this.onSubmit,
    this.onOauthDone,
    this.loading = false,
  });

  @override
  State<WizardStepView> createState() => _WizardStepViewState();
}

class _WizardStepViewState extends State<WizardStepView> {
  final _formKey = GlobalKey<FormState>();
  final Map<String, dynamic> _values = {};

  void _initDefaults() {
    for (final field in widget.step.dataSchema ?? []) {
      if (!_values.containsKey(field.name) && field.defaultValue != null) {
        _values[field.name] = field.defaultValue;
      }
    }
  }

  @override
  void initState() {
    super.initState();
    _initDefaults();
  }

  @override
  void didUpdateWidget(WizardStepView old) {
    super.didUpdateWidget(old);
    if (old.step != widget.step) {
      _values.clear();
      _initDefaults();
    }
  }

  void _submit() {
    if (_formKey.currentState?.validate() == true) {
      widget.onSubmit(_values);
    }
  }

  @override
  Widget build(BuildContext context) {
    final step = widget.step;

    // progress / external_step — OAuth or async HA operation
    if (step.type == 'progress' || step.type == 'external_step') {
      final url = step.url ?? _extractOAuthHref(step.descriptionPlaceholders);
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: Colors.blue.withOpacity(0.08),
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: Colors.blue.withOpacity(0.2)),
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Icon(Icons.info_outline, color: Colors.blue, size: 18),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    step.description ??
                        'Open the manufacturer\'s sign-in page, authorize access, then tap "I\'m authorized".',
                    style: const TextStyle(fontSize: 13),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          if (url != null) ...[
            FilledButton.icon(
              icon: const Icon(Icons.open_in_new, size: 18),
              label: const Text('Open sign-in page'),
              onPressed: () => launchUrl(
                Uri.parse(url),
                mode: LaunchMode.externalApplication,
              ),
            ),
            const SizedBox(height: 8),
          ] else ...[
            // No URL from HA — show manual instruction
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: Colors.orange.withOpacity(0.08),
                borderRadius: BorderRadius.circular(12),
              ),
              child: const Text(
                'Home Assistant is processing the request. If you need to authorise manually, open Home Assistant at http://localhost:8123 and complete the setup there.',
                style: TextStyle(fontSize: 12, color: Colors.grey),
              ),
            ),
            const SizedBox(height: 8),
            OutlinedButton.icon(
              icon: const Icon(Icons.open_in_browser, size: 16),
              label: const Text('Open Home Assistant'),
              onPressed: () => launchUrl(
                Uri.parse('http://localhost:8123'),
                mode: LaunchMode.externalApplication,
              ),
            ),
            const SizedBox(height: 8),
          ],
          FilledButton(
            onPressed: widget.loading ? null : () => widget.onOauthDone?.call(),
            child: widget.loading
                ? const SizedBox(
                    width: 20, height: 20,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Text("I'm authorized"),
          ),
        ],
      );
    }

    // abort
    if (step.type == 'abort') {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Icon(Icons.error_outline,
              color: Theme.of(context).colorScheme.error, size: 48),
          const SizedBox(height: 12),
          Text(
            'Setup failed: ${step.reason ?? 'Unknown reason'}',
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 16),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Close'),
          ),
        ],
      );
    }

    // create_entry
    if (step.type == 'create_entry') {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const Icon(Icons.check_circle, color: Colors.green, size: 48),
          const SizedBox(height: 12),
          const Text(
            'Integration added successfully!',
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
          ),
          if (step.title != null) ...[
            const SizedBox(height: 8),
            Text(step.title!, textAlign: TextAlign.center),
          ],
          const SizedBox(height: 16),
          FilledButton(
            onPressed: () => widget.onSubmit({}),
            child: const Text('Continue to devices'),
          ),
        ],
      );
    }

    // form (default)
    final schema = step.dataSchema ?? [];

    return Form(
      key: _formKey,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (step.description != null) ...[
            Text(step.description!,
                style: const TextStyle(color: Colors.grey)),
            const SizedBox(height: 12),
          ],
          ...schema.map((field) => Padding(
                padding: const EdgeInsets.only(bottom: 12),
                child: _buildField(field),
              )),
          const SizedBox(height: 8),
          FilledButton(
            onPressed: widget.loading ? null : _submit,
            child: widget.loading
                ? const SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Text('Next'),
          ),
        ],
      ),
    );
  }

  Widget _buildField(HassSchemaField field) {
    final options = field.normalizedOptions;
    final isMulti =
        field.type == 'multi_select' || (field.multiple && options.isNotEmpty);

    // multi_select → checkboxes
    if (isMulti && options.isNotEmpty) {
      final selected = (_values[field.name] as List?)?.cast<String>() ?? [];
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(field.name,
              style: const TextStyle(fontWeight: FontWeight.w500)),
          ...options.map((opt) => CheckboxListTile(
                dense: true,
                contentPadding: EdgeInsets.zero,
                title: Text(opt.$2),
                value: selected.contains(opt.$1),
                onChanged: (checked) {
                  final list = [...selected];
                  if (checked == true) {
                    list.add(opt.$1);
                  } else {
                    list.remove(opt.$1);
                  }
                  setState(() => _values[field.name] = list);
                },
              )),
        ],
      );
    }

    // dropdown
    if (options.isNotEmpty) {
      final current = _values[field.name]?.toString() ??
          (options.isNotEmpty ? options.first.$1 : null);
      if (!_values.containsKey(field.name) && current != null) {
        _values[field.name] = current;
      }
      return DropdownButtonFormField<String>(
        value: current,
        decoration: InputDecoration(
          labelText: field.name,
          border: const OutlineInputBorder(),
        ),
        items: options
            .map((o) => DropdownMenuItem(value: o.$1, child: Text(o.$2)))
            .toList(),
        onChanged: (v) => setState(() => _values[field.name] = v),
        validator: field.required
            ? (v) => v == null ? '${field.name} is required' : null
            : null,
      );
    }

    // boolean → switch
    if (field.type == 'boolean') {
      return SwitchListTile(
        contentPadding: EdgeInsets.zero,
        title: Text(field.name),
        value: _values[field.name] as bool? ?? false,
        onChanged: (v) => setState(() => _values[field.name] = v),
      );
    }

    // numeric
    final isNumeric =
        field.type == 'integer' || field.type == 'number' || field.type == 'float';

    // password/secret/token
    final isPassword = field.name.toLowerCase().contains('password') ||
        field.name.toLowerCase().contains('secret') ||
        field.name.toLowerCase().contains('token');

    return TextFormField(
      initialValue: _values[field.name]?.toString() ??
          field.defaultValue?.toString() ?? '',
      obscureText: isPassword,
      keyboardType: isNumeric
          ? const TextInputType.numberWithOptions(decimal: true)
          : TextInputType.text,
      decoration: InputDecoration(
        labelText: field.name,
        border: const OutlineInputBorder(),
      ),
      onChanged: (v) {
        if (isNumeric) {
          _values[field.name] = num.tryParse(v) ?? v;
        } else {
          _values[field.name] = v;
        }
      },
      validator: field.required
          ? (v) => v == null || v.isEmpty ? '${field.name} is required' : null
          : null,
    );
  }
}
