import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { FormEvent, useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { Modal } from '@/components/ui/Modal';
import { scheduleApi } from '@/api/endpoints';
import { errorMessage } from '@/api/client';
import { useActiveClassroom } from '@/stores/classroom';
import { ClassroomPicker } from '@/features/common/ClassroomGate';

export function SchedulePage() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const activeID = useActiveClassroom((s) => s.id);
  const [showAdd, setShowAdd] = useState(false);

  const weekQ = useQuery({
    queryKey: ['schedule', activeID],
    queryFn: () => scheduleApi.week(activeID!),
    enabled: !!activeID,
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => scheduleApi.remove(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['schedule', activeID] }),
  });

  const days = t('schedule.daysShort', { returnObjects: true }) as string[];

  return (
    <div className="p-4 flex flex-col gap-4 animate-fadeIn">
      <header className="flex items-center justify-between pt-2">
        <h1 className="text-2xl font-bold text-primary">{t('schedule.title')}</h1>
        <Button onClick={() => setShowAdd(true)} disabled={!activeID}>
          <Plus size={16} className="inline -mt-0.5 mr-1" />
          {t('schedule.addLesson')}
        </Button>
      </header>

      <ClassroomPicker />

      {activeID && weekQ.isLoading && <p className="text-slate-500">{t('common.loading')}</p>}

      {activeID && weekQ.data && (
        <div className="flex flex-col gap-3">
          {[1, 2, 3, 4, 5].map((day) => (
            <div key={day}>
              <h3 className="text-sm font-semibold text-slate-500 mb-1.5 ml-2">
                {days[day - 1]}
              </h3>
              <div className="flex flex-col gap-2">
                {(weekQ.data[day] ?? []).length === 0 && (
                  <Card className="text-sm text-slate-400 text-center py-3">—</Card>
                )}
                {(weekQ.data[day] ?? []).map((l) => (
                  <Card key={l.id} className="flex items-center justify-between">
                    <div>
                      <p className="font-semibold text-primary">{l.subject}</p>
                      <p className="text-xs text-slate-500">
                        {l.startsAt} – {l.endsAt}
                      </p>
                    </div>
                    <button
                      onClick={() => deleteMut.mutate(l.id)}
                      className="p-2 text-slate-400 hover:text-danger"
                    >
                      <Trash2 size={16} />
                    </button>
                  </Card>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}

      <Modal open={showAdd} onClose={() => setShowAdd(false)} title={t('schedule.addLesson')}>
        <AddLessonForm
          classroomID={activeID!}
          onCreated={() => {
            qc.invalidateQueries({ queryKey: ['schedule', activeID] });
            setShowAdd(false);
          }}
        />
      </Modal>
    </div>
  );
}

function AddLessonForm({ classroomID, onCreated }: { classroomID: string; onCreated: () => void }) {
  const { t } = useTranslation();
  const [subject, setSubject] = useState('');
  const [day, setDay] = useState('1');
  const [start, setStart] = useState('09:00');
  const [end, setEnd] = useState('10:00');
  const [err, setErr] = useState('');
  const [loading, setLoading] = useState(false);
  const days = t('schedule.daysShort', { returnObjects: true }) as string[];

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr('');
    setLoading(true);
    try {
      await scheduleApi.create({
        classroomId: classroomID,
        subject,
        dayOfWeek: Number(day),
        startsAt: start,
        endsAt: end,
      });
      onCreated();
    } catch (e) {
      setErr(errorMessage(e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      <Input label={t('schedule.subject')} value={subject} onChange={(e) => setSubject(e.target.value)} required />
      <label className="block">
        <span className="mb-1 block text-xs font-semibold text-slate-600">{t('schedule.day')}</span>
        <select className="input-field" value={day} onChange={(e) => setDay(e.target.value)}>
          {[1, 2, 3, 4, 5].map((d) => (
            <option key={d} value={d}>{days[d - 1]}</option>
          ))}
        </select>
      </label>
      <div className="grid grid-cols-2 gap-3">
        <Input type="time" label={t('schedule.startsAt')} value={start} onChange={(e) => setStart(e.target.value)} />
        <Input type="time" label={t('schedule.endsAt')} value={end} onChange={(e) => setEnd(e.target.value)} />
      </div>
      {err && <p className="text-sm text-danger">{err}</p>}
      <Button type="submit" disabled={loading}>{t('common.create')}</Button>
    </form>
  );
}
