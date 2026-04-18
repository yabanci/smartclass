import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { useMemo, useState } from 'react';
import { Plus, Power, Trash2, Wifi, WifiOff, Radar, Pencil } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/ui/Modal';
import { deviceApi } from '@/api/endpoints';
import type { Device } from '@/api/types';
import { useActiveClassroom } from '@/stores/classroom';
import { ClassroomPicker } from '@/features/common/ClassroomGate';
import { useWebSocket } from '@/hooks/useWebSocket';
import { FindIotWizard } from './FindIotWizard';
import { DeviceForm } from './DeviceForm';

export function DevicesPage() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const activeID = useActiveClassroom((s) => s.id);
  const [showAdd, setShowAdd] = useState(false);
  const [showFind, setShowFind] = useState(false);
  const [editing, setEditing] = useState<Device | null>(null);

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
      <header className="flex items-center justify-between pt-2 gap-2">
        <h1 className="text-2xl font-bold text-primary flex-1 min-w-0 truncate">{t('devices.title')}</h1>
        <Button variant="ghost" onClick={() => setShowFind(true)} disabled={!activeID} className="!py-2 !px-3">
          <Radar size={16} className="inline -mt-0.5 mr-1" />
          {t('devices.findIot')}
        </Button>
        <Button onClick={() => setShowAdd(true)} disabled={!activeID} className="!py-2 !px-3">
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
            onEdit={() => setEditing(d)}
            onDelete={() => deleteMut.mutate(d.id)}
          />
        ))}
      </div>

      <Modal open={showAdd} onClose={() => setShowAdd(false)} title={t('devices.add')}>
        {activeID && (
          <DeviceForm
            classroomID={activeID}
            onSubmitted={() => {
              qc.invalidateQueries({ queryKey: ['devices', activeID] });
              setShowAdd(false);
            }}
          />
        )}
      </Modal>

      <Modal open={!!editing} onClose={() => setEditing(null)} title={t('common.edit')}>
        {editing && activeID && (
          <DeviceForm
            classroomID={activeID}
            device={editing}
            onSubmitted={() => {
              qc.invalidateQueries({ queryKey: ['devices', activeID] });
              setEditing(null);
            }}
          />
        )}
      </Modal>

      <Modal open={showFind} onClose={() => setShowFind(false)} title={t('hass.title')}>
        {activeID && (
          <FindIotWizard
            classroomID={activeID}
            onAdopted={() => {
              qc.invalidateQueries({ queryKey: ['devices', activeID] });
            }}
          />
        )}
      </Modal>
    </div>
  );
}

function DeviceCard({ device, onCommand, onEdit, onDelete }: { device: Device; onCommand: (c: 'ON' | 'OFF') => void; onEdit: () => void; onDelete: () => void }) {
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
          <button onClick={onEdit} className="p-2 rounded-xl text-slate-400 hover:text-primary" aria-label={t('common.edit')}>
            <Pencil size={16} />
          </button>
          <button onClick={onDelete} className="p-2 rounded-xl text-slate-400 hover:text-danger" aria-label={t('common.delete')}>
            <Trash2 size={16} />
          </button>
        </div>
      </div>
    </Card>
  );
}
