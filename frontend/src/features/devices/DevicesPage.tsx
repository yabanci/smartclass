import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { useMemo, useState, FormEvent } from 'react';
import { Plus, Power, Trash2, Wifi, WifiOff } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Modal } from '@/components/ui/Modal';
import { deviceApi } from '@/api/endpoints';
import { errorMessage } from '@/api/client';
import type { Device } from '@/api/types';
import { useActiveClassroom } from '@/stores/classroom';
import { ClassroomPicker } from '@/features/common/ClassroomGate';
import { useWebSocket } from '@/hooks/useWebSocket';

export function DevicesPage() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const activeID = useActiveClassroom((s) => s.id);
  const [showAdd, setShowAdd] = useState(false);

  const devicesQ = useQuery({
    queryKey: ['devices', activeID],
    queryFn: () => deviceApi.listByClassroom(activeID!),
    enabled: !!activeID,
  });

  const topics = useMemo(
    () => (activeID ? [`classroom:${activeID}:devices`] : []),
    [activeID],
  );
  useWebSocket(topics, () => qc.invalidateQueries({ queryKey: ['devices', activeID] }));

  const commandMut = useMutation({
    mutationFn: ({ id, type }: { id: string; type: 'ON' | 'OFF' }) => deviceApi.command(id, { type }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['devices', activeID] }),
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => deviceApi.remove(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['devices', activeID] }),
  });

  return (
    <div className="p-4 flex flex-col gap-4 animate-fadeIn">
      <header className="flex items-center justify-between pt-2">
        <h1 className="text-2xl font-bold text-primary">{t('devices.title')}</h1>
        <Button onClick={() => setShowAdd(true)} disabled={!activeID}>
          <Plus size={16} className="inline -mt-0.5 mr-1" />
          {t('devices.add')}
        </Button>
      </header>

      <ClassroomPicker />

      {activeID && devicesQ.isLoading && <p className="text-slate-500">{t('common.loading')}</p>}

      {activeID && devicesQ.data && devicesQ.data.length === 0 && (
        <Card className="text-center text-slate-500">{t('devices.empty')}</Card>
      )}

      <div className="flex flex-col gap-3">
        {(devicesQ.data ?? []).map((d) => (
          <DeviceCard
            key={d.id}
            device={d}
            onCommand={(type) => commandMut.mutate({ id: d.id, type })}
            onDelete={() => deleteMut.mutate(d.id)}
          />
        ))}
      </div>

      <Modal open={showAdd} onClose={() => setShowAdd(false)} title={t('devices.add')}>
        <AddDeviceForm
          classroomID={activeID!}
          onCreated={() => {
            qc.invalidateQueries({ queryKey: ['devices', activeID] });
            setShowAdd(false);
          }}
        />
      </Modal>
    </div>
  );
}

function DeviceCard({ device, onCommand, onDelete }: { device: Device; onCommand: (c: 'ON' | 'OFF') => void; onDelete: () => void }) {
  const { t } = useTranslation();
  const on = device.status === 'on' || device.status === 'open';
  return (
    <Card>
      <div className="flex items-center justify-between">
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <span className="font-semibold text-primary">{device.name}</span>
            {device.online ? (
              <Wifi size={14} className="text-accent" aria-label={t('common.online')} />
            ) : (
              <WifiOff size={14} className="text-slate-400" aria-label={t('common.offline')} />
            )}
          </div>
          <p className="text-xs text-slate-500 capitalize">
            {device.brand} · {device.type} · {device.driver}
          </p>
          <p className="text-sm mt-1 text-slate-600">
            {t('devices.status')}: <span className="font-semibold">{device.status}</span>
          </p>
        </div>
        <div className="flex flex-col gap-1">
          <button
            onClick={() => onCommand(on ? 'OFF' : 'ON')}
            className={`p-2 rounded-xl ${on ? 'bg-accent text-white' : 'bg-slate-200 text-slate-500'}`}
            aria-label={on ? t('devices.off') : t('devices.on')}
          >
            <Power size={20} />
          </button>
          <button onClick={onDelete} className="p-2 rounded-xl text-slate-400 hover:text-danger">
            <Trash2 size={16} />
          </button>
        </div>
      </div>
    </Card>
  );
}

function AddDeviceForm({ classroomID, onCreated }: { classroomID: string; onCreated: () => void }) {
  const { t } = useTranslation();
  const [name, setName] = useState('');
  const [type, setType] = useState('light');
  const [brand, setBrand] = useState('generic');
  const [driver, setDriver] = useState('generic_http');
  const [configText, setConfigText] = useState(
    '{\n  "baseUrl": "http://192.168.1.100",\n  "commands": {\n    "ON":  {"method":"POST","path":"/relay/0?turn=on"},\n    "OFF": {"method":"POST","path":"/relay/0?turn=off"}\n  }\n}',
  );
  const [err, setErr] = useState('');
  const [loading, setLoading] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr('');
    setLoading(true);
    try {
      const config = JSON.parse(configText || '{}');
      await deviceApi.create({ classroomId: classroomID, name, type, brand, driver, config });
      onCreated();
    } catch (e) {
      setErr(errorMessage(e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      <Input label={t('devices.name')} value={name} onChange={(e) => setName(e.target.value)} required />
      <Input label={t('devices.type')} value={type} onChange={(e) => setType(e.target.value)} placeholder="light / relay / sensor" />
      <Input label={t('devices.brand')} value={brand} onChange={(e) => setBrand(e.target.value)} placeholder="tuya / shelly / sonoff / aqara" />
      <Input label={t('devices.driver')} value={driver} onChange={(e) => setDriver(e.target.value)} />
      <label className="block">
        <span className="mb-1 block text-xs font-semibold text-slate-600">{t('devices.config')}</span>
        <textarea
          className="input-field font-mono text-xs"
          rows={8}
          value={configText}
          onChange={(e) => setConfigText(e.target.value)}
        />
      </label>
      {err && <p className="text-sm text-danger">{err}</p>}
      <Button type="submit" disabled={loading}>{t('common.create')}</Button>
    </form>
  );
}
