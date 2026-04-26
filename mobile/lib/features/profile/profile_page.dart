import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app.dart';
import '../../core/connection/resolver.dart';
import '../../core/i18n/app_localizations.dart';
import '../../shared/providers/auth_provider.dart';
import '../../shared/widgets/app_button.dart';

class ProfilePage extends ConsumerStatefulWidget {
  const ProfilePage({super.key});

  @override
  ConsumerState<ProfilePage> createState() => _ProfilePageState();
}

class _ProfilePageState extends ConsumerState<ProfilePage> {
  bool _editMode = false;
  late TextEditingController _nameCtrl;
  late TextEditingController _phoneCtrl;
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    final user = ref.read(authProvider).user;
    _nameCtrl = TextEditingController(text: user?.fullName ?? '');
    _phoneCtrl = TextEditingController(text: user?.phone ?? '');
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _phoneCtrl.dispose();
    super.dispose();
  }

  Future<void> _saveProfile() async {
    setState(() => _saving = true);
    try {
      final user = await ref.read(userEndpointsProvider).updateMe(
            fullName: _nameCtrl.text.trim(),
            phone: _phoneCtrl.text.isNotEmpty
                ? _phoneCtrl.text.trim()
                : null,
          );
      ref.read(authProvider.notifier).updateUser(user);
      setState(() => _editMode = false);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(e.toString())));
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context)!;
    final user = ref.watch(authProvider).user;
    if (user == null) return const SizedBox.shrink();

    return Scaffold(
      appBar: AppBar(
        title: Text(l.profileTitle),
        actions: [
          if (!_editMode)
            IconButton(
              icon: const Icon(Icons.edit_outlined),
              onPressed: () => setState(() => _editMode = true),
            ),
        ],
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          // Avatar + basic info
          Center(
            child: Column(
              children: [
                CircleAvatar(
                  radius: 40,
                  child: Text(
                    user.fullName.isNotEmpty
                        ? user.fullName[0].toUpperCase()
                        : '?',
                    style: const TextStyle(fontSize: 32),
                  ),
                ),
                const SizedBox(height: 12),
                Text(user.email,
                    style: const TextStyle(color: Colors.grey)),
                const SizedBox(height: 4),
                Chip(label: Text(user.role)),
              ],
            ),
          ),
          const SizedBox(height: 24),

          // Edit fields
          if (_editMode) ...[
            TextFormField(
              controller: _nameCtrl,
              decoration: InputDecoration(
                labelText: l.authFullName,
                border: const OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 12),
            TextFormField(
              controller: _phoneCtrl,
              decoration: InputDecoration(
                labelText: l.profilePhone,
                border: const OutlineInputBorder(),
              ),
              keyboardType: TextInputType.phone,
            ),
            const SizedBox(height: 16),
            Row(
              children: [
                Expanded(
                  child: AppButton(
                    label: l.commonSave,
                    loading: _saving,
                    onPressed: _saveProfile,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: AppButton(
                    label: l.commonCancel,
                    outlined: true,
                    onPressed: () => setState(() => _editMode = false),
                  ),
                ),
              ],
            ),
          ] else ...[
            ListTile(
              leading: const Icon(Icons.person_outlined),
              title: Text(l.authFullName),
              subtitle: Text(user.fullName),
            ),
            if (user.phone != null)
              ListTile(
                leading: const Icon(Icons.phone_outlined),
                title: Text(l.profilePhone),
                subtitle: Text(user.phone!),
              ),
          ],

          const Divider(),

          // Language
          ListTile(
            leading: const Icon(Icons.language),
            title: Text(l.profileLanguage),
            trailing: DropdownButton<Locale>(
              value: ref.watch(localeProvider),
              underline: const SizedBox(),
              items: const [
                DropdownMenuItem(value: Locale('en'), child: Text('English')),
                DropdownMenuItem(value: Locale('ru'), child: Text('Русский')),
                DropdownMenuItem(value: Locale('kk'), child: Text('Қазақша')),
              ],
              onChanged: (v) {
                if (v != null) ref.read(localeProvider.notifier).state = v;
              },
            ),
          ),

          // Server URL
          _ServerUrlTile(),

          // Change password
          ListTile(
            leading: const Icon(Icons.lock_outlined),
            title: Text(l.profileChangePassword),
            trailing: const Icon(Icons.chevron_right),
            onTap: () => _showChangePassword(context),
          ),

          // Analytics
          ListTile(
            leading: const Icon(Icons.analytics_outlined),
            title: Text(l.analyticsTitle),
            trailing: const Icon(Icons.chevron_right),
            onTap: () => context.push('/analytics'),
          ),

          const Divider(),

          // Logout
          ListTile(
            leading: Icon(Icons.logout,
                color: Theme.of(context).colorScheme.error),
            title: Text(l.authLogout,
                style:
                    TextStyle(color: Theme.of(context).colorScheme.error)),
            onTap: () async {
              await ref.read(authProvider.notifier).logout();
            },
          ),
        ],
      ),
    );
  }

  void _showChangePassword(BuildContext context) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (_) => const _ChangePasswordSheet(),
    );
  }
}

class _ServerUrlTile extends ConsumerStatefulWidget {
  @override
  ConsumerState<_ServerUrlTile> createState() => _ServerUrlTileState();
}

class _ServerUrlTileState extends ConsumerState<_ServerUrlTile> {
  final _ctrl = TextEditingController();
  bool _editing = false;

  @override
  void initState() {
    super.initState();
    ConnectionResolver.instance.getLocalUrl().then((url) {
      if (mounted) _ctrl.text = url ?? '';
    });
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context)!;
    if (_editing) {
      return Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        child: Row(
          children: [
            Expanded(
              child: TextFormField(
                controller: _ctrl,
                decoration: InputDecoration(
                  labelText: l.profileLocalUrl,
                  hintText: l.profileLocalUrlHint,
                  border: const OutlineInputBorder(),
                  isDense: true,
                ),
                keyboardType: TextInputType.url,
              ),
            ),
            const SizedBox(width: 8),
            IconButton(
              icon: const Icon(Icons.check),
              onPressed: () async {
                await ConnectionResolver.instance.setLocalUrl(_ctrl.text.trim());
                setState(() => _editing = false);
              },
            ),
            IconButton(
              icon: const Icon(Icons.close),
              onPressed: () => setState(() => _editing = false),
            ),
          ],
        ),
      );
    }

    return ListTile(
      leading: const Icon(Icons.dns_outlined),
      title: Text(l.profileLocalUrl),
      subtitle: Text(
        _ctrl.text.isNotEmpty ? _ctrl.text : 'Not set',
        style: const TextStyle(fontSize: 12),
      ),
      trailing: const Icon(Icons.edit_outlined, size: 18),
      onTap: () => setState(() => _editing = true),
    );
  }
}

class _ChangePasswordSheet extends ConsumerStatefulWidget {
  const _ChangePasswordSheet();

  @override
  ConsumerState<_ChangePasswordSheet> createState() =>
      _ChangePasswordSheetState();
}

class _ChangePasswordSheetState
    extends ConsumerState<_ChangePasswordSheet> {
  final _formKey = GlobalKey<FormState>();
  final _currentCtrl = TextEditingController();
  final _newCtrl = TextEditingController();
  bool _saving = false;

  @override
  void dispose() {
    _currentCtrl.dispose();
    _newCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _saving = true);
    try {
      await ref.read(userEndpointsProvider).changePassword(
            currentPassword: _currentCtrl.text,
            newPassword: _newCtrl.text,
          );
      if (mounted) {
        Navigator.of(context).pop();
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Password changed successfully')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(e.toString())));
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context)!;
    return Padding(
      padding: EdgeInsets.only(
        left: 16,
        right: 16,
        top: 16,
        bottom: MediaQuery.of(context).viewInsets.bottom + 16,
      ),
      child: Form(
        key: _formKey,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(l.profileChangePassword,
                style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
            const SizedBox(height: 16),
            TextFormField(
              controller: _currentCtrl,
              obscureText: true,
              decoration: InputDecoration(
                labelText: l.profileCurrentPassword,
                border: const OutlineInputBorder(),
              ),
              validator: (v) =>
                  v == null || v.isEmpty ? 'Required' : null,
            ),
            const SizedBox(height: 12),
            TextFormField(
              controller: _newCtrl,
              obscureText: true,
              decoration: InputDecoration(
                labelText: l.profileNewPassword,
                border: const OutlineInputBorder(),
              ),
              validator: (v) =>
                  v == null || v.length < 6 ? 'Min 6 characters' : null,
            ),
            const SizedBox(height: 16),
            FilledButton(
              onPressed: _saving ? null : _submit,
              child: _saving
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Text(l.profileChangePassword),
            ),
          ],
        ),
      ),
    );
  }
}
