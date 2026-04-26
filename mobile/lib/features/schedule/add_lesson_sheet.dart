import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/i18n/app_localizations.dart';
import '../../shared/providers/schedule_provider.dart';

class AddLessonSheet extends ConsumerStatefulWidget {
  final String classroomId;

  const AddLessonSheet({super.key, required this.classroomId});

  @override
  ConsumerState<AddLessonSheet> createState() => _AddLessonSheetState();
}

class _AddLessonSheetState extends ConsumerState<AddLessonSheet> {
  final _formKey = GlobalKey<FormState>();
  final _subjectCtrl = TextEditingController();
  final _notesCtrl = TextEditingController();
  int _day = 1;
  TimeOfDay _startTime = const TimeOfDay(hour: 8, minute: 0);
  TimeOfDay _endTime = const TimeOfDay(hour: 9, minute: 0);
  bool _saving = false;

  String _formatTime(TimeOfDay t) =>
      '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';

  Future<void> _pickTime(bool isStart) async {
    final picked = await showTimePicker(
      context: context,
      initialTime: isStart ? _startTime : _endTime,
    );
    if (picked != null) {
      setState(() {
        if (isStart) _startTime = picked;
        else _endTime = picked;
      });
    }
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _saving = true);
    try {
      await ref.read(scheduleProvider(widget.classroomId).notifier).addLesson(
            subject: _subjectCtrl.text.trim(),
            dayOfWeek: _day,
            startsAt: _formatTime(_startTime),
            endsAt: _formatTime(_endTime),
            notes: _notesCtrl.text.isNotEmpty ? _notesCtrl.text.trim() : null,
          );
      if (mounted) Navigator.of(context).pop();
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
    final days = [
      (1, l.scheduleDayMon),
      (2, l.scheduleDayTue),
      (3, l.scheduleDayWed),
      (4, l.scheduleDayThu),
      (5, l.scheduleDayFri),
    ];

    return Padding(
      padding: EdgeInsets.only(
        left: 16,
        right: 16,
        top: 16,
        bottom: MediaQuery.of(context).viewInsets.bottom + 16,
      ),
      child: Form(
        key: _formKey,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(l.scheduleAddLesson,
                  style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
              const SizedBox(height: 16),
              TextFormField(
                controller: _subjectCtrl,
                decoration: InputDecoration(
                  labelText: l.scheduleSubject,
                  border: const OutlineInputBorder(),
                ),
                validator: (v) =>
                    v == null || v.isEmpty ? '${l.scheduleSubject} is required' : null,
              ),
              const SizedBox(height: 12),
              DropdownButtonFormField<int>(
                value: _day,
                decoration: InputDecoration(
                  labelText: l.scheduleDay,
                  border: const OutlineInputBorder(),
                ),
                items: days
                    .map((d) =>
                        DropdownMenuItem(value: d.$1, child: Text(d.$2)))
                    .toList(),
                onChanged: (v) => setState(() => _day = v ?? 1),
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    child: OutlinedButton.icon(
                      icon: const Icon(Icons.access_time),
                      label: Text('${l.scheduleStartsAt}: ${_formatTime(_startTime)}'),
                      onPressed: () => _pickTime(true),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: OutlinedButton.icon(
                      icon: const Icon(Icons.access_time),
                      label: Text('${l.scheduleEndsAt}: ${_formatTime(_endTime)}'),
                      onPressed: () => _pickTime(false),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _notesCtrl,
                decoration: InputDecoration(
                  labelText: '${l.scheduleNotes} (optional)',
                  border: const OutlineInputBorder(),
                ),
                maxLines: 2,
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
                    : Text(l.scheduleAddLesson),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
