import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { useState, useMemo, FormEvent } from 'react';
import { Cpu, Thermometer, Droplets, Calendar, Zap } from 'lucide-react';
import { Link } from 'react-router-dom';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Modal } from '@/components/ui/Modal';
import { classroomApi, deviceApi, scheduleApi, sensorApi } from '@/api/endpoints';
import type { CommandType } from '@/api/types';
import { useActiveClassroom } from '@/stores/classroom';
import { ClassroomPicker } from '@/features/common/ClassroomGate';
import { useWebSocket } from '@/hooks/useWebSocket';

export function HomePage() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const activeID = useActiveClassroom((s) => s.id);

  const [showCreate, setShowCreate] = useState(false);

  const classroomsQ = useQuery({ queryKey: ['classrooms'], queryFn: () => classroomApi.list() });

  const devicesQ = useQuery({
    queryKey: ['devices', activeID],
    queryFn: () => deviceApi.listByClassroom(activeID!),
    enabled: !!activeID,
  });

  const sensorsQ = useQuery({
    queryKey: ['sensors-latest', activeID],
    queryFn: () => sensorApi.latest(activeID!),
    enabled: !!activeID,
  });

  const currentQ = useQuery({
    queryKey: ['schedule-current', activeID],
    queryFn: () => scheduleApi.current(activeID!),
    enabled: !!activeID,
  });

  const topics = useMemo(
    () => (activeID ? [`classroom:${activeID}:devices`, `classroom:${activeID}:sensors`] : []),
    [activeID],
  );

  useWebSocket(topics, (evt) => {
    if (evt.type.startsWith('device.')) qc.invalidateQueries({ queryKey: ['devices', activeID] });
    if (evt.type === 'sensor.reading') qc.invalidateQueries({ queryKey: ['sensors-latest', activeID] });
    if (evt.type === 'notification.created') qc.invalidateQueries({ queryKey: ['notif-unread'] });
  });

  const createMut = useMutation({
    mutationFn: (name: string) => classroomApi.create({ name }),
    onSuccess: (c) => {
      qc.invalidateQueries({ queryKey: ['classrooms'] });
      useActiveClassroom.getState().set(c.id);
      setShowCreate(false);
    },
  });

  const commandMut = useMutation({
    mutationFn: ({ id, type }: { id: string; type: CommandType }) => deviceApi.command(id, { type }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['devices', activeID] }),
  });

  const list = devicesQ.data ?? [];
  const activeDevices = list.filter((d) => d.online).length;
  const onCount = list.filter((d) => d.status === 'on' || d.status === 'open').length;
  const temp = sensorsQ.data?.find((r) => r.metric === 'temperature');
  const humid = sensorsQ.data?.find((r) => r.metric === 'humidity');

  const allOn = () =>
    list.filter((d) => d.status !== 'on').forEach((d) => commandMut.mutate({ id: d.id, type: 'ON' }));
  const allOff = () =>
    list
      .filter((d) => d.status === 'on' || d.status === 'open')
      .forEach((d) => commandMut.mutate({ id: d.id, type: 'OFF' }));

  return (
    <div className="p-4 flex flex-col gap-4 animate-fadeIn">
      <ClassroomPicker onCreate={() => setShowCreate(true)} />

      {activeID && classroomsQ.data && (
        <p className="text-xs text-slate-500 dark:text-slate-400 -mt-2">
          {classroomsQ.data.find((c) => c.id === activeID)?.name}
        </p>
      )}

      {activeID && (
        <>
          <div className="grid grid-cols-2 gap-3">
            <Card className="!p-4">
              <div className="flex items-center gap-2 mb-1">
                <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
                  <Cpu size={16} className="text-primary" />
                </div>
                <span className="text-xs text-slate-500 dark:text-slate-400">{t('home.activeDevices')}</span>
              </div>
              <p className="text-2xl font-bold">{activeDevices}</p>
              <p className="text-xs text-accent">● {onCount} {t('devices.on').toLowerCase()}</p>
            </Card>
            <Card className="!p-4">
              <div className="flex items-center gap-2 mb-1">
                <div className="w-8 h-8 rounded-lg bg-accent/10 flex items-center justify-center">
                  <Zap size={16} className="text-accent" />
                </div>
                <span className="text-xs text-slate-500 dark:text-slate-400">{t('analytics.energy')}</span>
              </div>
              <p className="text-2xl font-bold">
                {((onCount || 0) * 0.2).toFixed(1)}
                <span className="text-sm"> kW</span>
              </p>
              <p className="text-xs text-slate-500">—</p>
            </Card>
          </div>

          {list.length > 0 && (
            <Card className="!p-4">
              <h3 className="text-sm font-bold mb-3">{t('devices.quickControls')}</h3>
              <div className="flex gap-2">
                <button
                  onClick={allOn}
                  className="flex-1 py-2.5 bg-primary text-white rounded-xl text-xs font-semibold hover:opacity-90 transition"
                >
                  {t('devices.allOn')}
                </button>
                <button
                  onClick={allOff}
                  className="flex-1 py-2.5 bg-gray-200 dark:bg-dark-surface text-gray-700 dark:text-gray-200 rounded-xl text-xs font-semibold hover:opacity-90 transition"
                >
                  {t('devices.allOff')}
                </button>
                <Link
                  to="/devices"
                  className="flex-1 py-2.5 bg-accent/10 text-accent rounded-xl text-xs font-semibold hover:bg-accent/20 transition flex items-center justify-center"
                >
                  🌿 {t('devices.eco')}
                </Link>
              </div>
            </Card>
          )}

          <div className="grid grid-cols-2 gap-3">
            <Card className="!p-4 flex flex-col gap-1">
              <div className="flex items-center gap-2 text-secondary">
                <Thermometer size={16} />
                <span className="text-xs font-semibold text-slate-500 uppercase">
                  {t('home.temperature')}
                </span>
              </div>
              <span className="text-2xl font-bold text-primary">
                {temp ? `${temp.value.toFixed(1)}°${temp.unit || 'C'}` : '—'}
              </span>
            </Card>
            <Card className="!p-4 flex flex-col gap-1">
              <div className="flex items-center gap-2 text-secondary">
                <Droplets size={16} />
                <span className="text-xs font-semibold text-slate-500 uppercase">
                  {t('home.humidity')}
                </span>
              </div>
              <span className="text-2xl font-bold text-primary">
                {humid ? `${humid.value.toFixed(0)}${humid.unit || '%'}` : '—'}
              </span>
            </Card>
          </div>

          <Card className="!p-4">
            <div className="flex items-center gap-2 mb-2 text-primary">
              <Calendar size={16} />
              <span className="font-semibold">{t('home.currentLesson')}</span>
            </div>
            {currentQ.data ? (
              <div className="highlight-lesson rounded-xl p-3">
                <p className="text-xs text-accent font-semibold">● {t('home.currentLesson')}</p>
                <p className="text-sm font-bold">{currentQ.data.subject}</p>
                <p className="text-xs text-slate-500">
                  {currentQ.data.startsAt} – {currentQ.data.endsAt}
                </p>
              </div>
            ) : (
              <p className="text-sm text-slate-500">{t('home.noLesson')}</p>
            )}
          </Card>
        </>
      )}

      <Modal open={showCreate} onClose={() => setShowCreate(false)} title={t('home.createClassroom')}>
        <CreateClassroomForm onCreate={(name) => createMut.mutate(name)} loading={createMut.isPending} />
      </Modal>
    </div>
  );
}

function CreateClassroomForm({ onCreate, loading }: { onCreate: (name: string) => void; loading: boolean }) {
  const { t } = useTranslation();
  const [name, setName] = useState('');
  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (name.trim()) onCreate(name.trim());
  };
  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      <Input label={t('home.classroomName')} value={name} onChange={(e) => setName(e.target.value)} required />
      <Button type="submit" disabled={loading}>{t('common.create')}</Button>
    </form>
  );
}
