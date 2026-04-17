import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { useState, useMemo, FormEvent } from 'react';
import { Bell, Cpu, Thermometer, Droplets, Calendar } from 'lucide-react';
import { Link } from 'react-router-dom';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Modal } from '@/components/ui/Modal';
import { classroomApi, deviceApi, notificationApi, scheduleApi, sensorApi } from '@/api/endpoints';
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

  const unreadQ = useQuery({ queryKey: ['notif-unread'], queryFn: () => notificationApi.unreadCount() });

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

  const activeDevices = (devicesQ.data ?? []).filter((d) => d.online).length;
  const temp = sensorsQ.data?.find((r) => r.metric === 'temperature');
  const humid = sensorsQ.data?.find((r) => r.metric === 'humidity');

  return (
    <div className="p-4 flex flex-col gap-4 animate-fadeIn">
      <header className="flex items-center justify-between pt-2">
        <div>
          <h1 className="text-2xl font-bold text-primary">{t('home.title')}</h1>
          {classroomsQ.data && classroomsQ.data.length > 0 && activeID && (
            <p className="text-sm text-slate-500">
              {classroomsQ.data.find((c) => c.id === activeID)?.name}
            </p>
          )}
        </div>
        <Link to="/notifications" className="relative p-2 rounded-full bg-white soft-shadow text-primary">
          <Bell size={20} />
          {(unreadQ.data?.count ?? 0) > 0 && (
            <span className="absolute -top-0.5 -right-0.5 min-w-4 h-4 px-1 rounded-full bg-danger text-white text-[10px] flex items-center justify-center">
              {unreadQ.data?.count}
            </span>
          )}
        </Link>
      </header>

      <ClassroomPicker onCreate={() => setShowCreate(true)} />

      {activeID && (
        <>
          <div className="grid grid-cols-2 gap-3">
            <Card className="flex flex-col gap-1">
              <div className="flex items-center gap-2 text-secondary">
                <Thermometer size={18} />
                <span className="text-xs font-semibold text-slate-500 uppercase">{t('home.temperature')}</span>
              </div>
              <span className="text-2xl font-bold text-primary">
                {temp ? `${temp.value.toFixed(1)}°${temp.unit || 'C'}` : '—'}
              </span>
            </Card>
            <Card className="flex flex-col gap-1">
              <div className="flex items-center gap-2 text-secondary">
                <Droplets size={18} />
                <span className="text-xs font-semibold text-slate-500 uppercase">{t('home.humidity')}</span>
              </div>
              <span className="text-2xl font-bold text-primary">
                {humid ? `${humid.value.toFixed(0)}${humid.unit || '%'}` : '—'}
              </span>
            </Card>
          </div>

          <Card>
            <div className="flex items-center gap-2 mb-2 text-primary">
              <Cpu size={18} />
              <span className="font-semibold">{t('home.activeDevices')}</span>
            </div>
            <div className="flex items-end justify-between">
              <span className="text-3xl font-bold text-primary">
                {activeDevices}
                <span className="text-base text-slate-400">/{devicesQ.data?.length ?? 0}</span>
              </span>
              <Link to="/devices" className="text-xs text-secondary font-semibold">
                {t('nav.devices')} →
              </Link>
            </div>
          </Card>

          <Card>
            <div className="flex items-center gap-2 mb-2 text-primary">
              <Calendar size={18} />
              <span className="font-semibold">{t('home.currentLesson')}</span>
            </div>
            {currentQ.data ? (
              <>
                <p className="text-xl font-bold text-primary">{currentQ.data.subject}</p>
                <p className="text-sm text-slate-500">
                  {currentQ.data.startsAt} – {currentQ.data.endsAt}
                </p>
              </>
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
