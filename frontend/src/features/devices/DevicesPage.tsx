import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { useMemo, useState, useEffect } from 'react';
import { Plus, Trash2, Wifi, WifiOff, Radar, Pencil } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/ui/Modal';
import { deviceApi } from '@/api/endpoints';
import type { Device, CommandType } from '@/api/types';
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
    mutationFn: ({ id, type, value }: { id: string; type: CommandType; value?: unknown }) =>
      deviceApi.command(id, { type, value }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['devices', activeID] }),
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => deviceApi.remove(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['devices', activeID] }),
  });

  const list = devicesQ.data ?? [];
  const allOn = async () =>
    Promise.all(list.filter((d) => d.status !== 'on').map((d) => commandMut.mutateAsync({ id: d.id, type: 'ON' })));
  const allOff = async () =>
    Promise.all(list.filter((d) => d.status === 'on' || d.status === 'open').map((d) => commandMut.mutateAsync({ id: d.id, type: 'OFF' })));

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

      {activeID && list.length > 0 && (
        <Card className="!p-3">
          <p className="text-xs font-bold mb-2 text-slate-500 dark:text-slate-300">
            {t('devices.quickControls')}
          </p>
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
          </div>
        </Card>
      )}

      {activeID && devicesQ.isLoading && <p className="text-slate-500">{t('common.loading')}</p>}

      {activeID && devicesQ.data && devicesQ.data.length === 0 && (
        <Card className="text-center text-slate-500">{t('devices.empty')}</Card>
      )}

      <div className="flex flex-col gap-3">
        {list.map((d) => (
          <DeviceCard
            key={d.id}
            device={d}
            onCommand={(type, value) => commandMut.mutate({ id: d.id, type, value })}
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

type Kind = 'climate' | 'light' | 'fan' | 'cover' | 'switch' | 'sensor' | 'other';

function kindOf(d: Device): Kind {
  const t = (d.type || '').toLowerCase();
  if (t.includes('climat') || t.includes('ac') || t.includes('thermo')) return 'climate';
  if (t.includes('light')) return 'light';
  if (t.includes('fan') || t.includes('fresh')) return 'fan';
  if (t.includes('cover') || t.includes('blind') || t.includes('curtain')) return 'cover';
  if (t.includes('sensor')) return 'sensor';
  if (t === 'switch' || t === 'relay' || t === 'projector' || t === 'lock') return 'switch';
  return 'other';
}

function iconFor(kind: Kind): string {
  switch (kind) {
    case 'climate': return '❄️';
    case 'light': return '💡';
    case 'fan': return '🌬️';
    case 'cover': return '🪟';
    case 'sensor': return '🌡️';
    default: return '🔌';
  }
}

function tintFor(kind: Kind): string {
  switch (kind) {
    case 'climate': return 'bg-blue-50 dark:bg-blue-900/30';
    case 'light': return 'bg-yellow-50 dark:bg-yellow-900/30';
    case 'fan': return 'bg-green-50 dark:bg-green-900/30';
    case 'cover': return 'bg-purple-50 dark:bg-purple-900/30';
    case 'sensor': return 'bg-red-50 dark:bg-red-900/30';
    default: return 'bg-slate-50 dark:bg-slate-800/40';
  }
}

function defaultValue(kind: Kind): number {
  switch (kind) {
    case 'climate': return 22;
    case 'light': return 75;
    case 'fan': return 66;
    case 'cover': return 50;
    default: return 0;
  }
}

function DeviceCard({
  device,
  onCommand,
  onEdit,
  onDelete,
}: {
  device: Device;
  onCommand: (type: CommandType, value?: unknown) => void;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const { t } = useTranslation();
  const kind = kindOf(device);
  const on = device.status === 'on' || device.status === 'open';
  const icon = iconFor(kind);
  const tint = tintFor(kind);

  const persisted = Number((device.config as { lastValue?: number })?.lastValue);
  const [value, setValue] = useState<number>(
    Number.isFinite(persisted) ? persisted : defaultValue(kind),
  );

  useEffect(() => {
    if (Number.isFinite(persisted)) setValue(persisted);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [device.id, persisted]);

  const commit = (v: number) => {
    setValue(v);
    onCommand('SET_VALUE', v);
  };

  return (
    <div className="glass rounded-2xl p-4 card-shadow">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3 min-w-0 flex-1">
          <div className={`w-11 h-11 rounded-xl ${tint} flex items-center justify-center text-2xl flex-shrink-0`}>
            {icon}
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-1.5">
              <p className="text-sm font-bold truncate">{device.name}</p>
              {device.online ? (
                <Wifi size={12} className="text-accent flex-shrink-0" aria-label={t('common.online')} />
              ) : (
                <WifiOff size={12} className="text-slate-400 flex-shrink-0" aria-label={t('common.offline')} />
              )}
            </div>
            <p className="text-xs text-slate-500 dark:text-slate-400 truncate capitalize">
              {device.brand} · {device.type} · {on ? t('common.online') : t('common.offline')}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2 flex-shrink-0">
          {kind !== 'sensor' && (
            <div
              className={`toggle ${on ? 'on' : ''}`}
              role="switch"
              aria-checked={on}
              aria-label={on ? t('devices.off') : t('devices.on')}
              onClick={() => onCommand(on ? 'OFF' : 'ON')}
            />
          )}
          <button
            onClick={onEdit}
            className="w-7 h-7 rounded-lg bg-primary/10 text-primary flex items-center justify-center"
            aria-label={t('common.edit')}
          >
            <Pencil size={13} />
          </button>
          <button
            onClick={onDelete}
            className="w-7 h-7 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-500 flex items-center justify-center"
            aria-label={t('common.delete')}
          >
            <Trash2 size={13} />
          </button>
        </div>
      </div>

      {on && kind === 'climate' && (
        <SliderControl
          label={t('devices.temperature')}
          unit="°C"
          min={16}
          max={30}
          value={value}
          onChange={setValue}
          onCommit={commit}
        />
      )}
      {on && kind === 'light' && (
        <SliderControl
          label={t('devices.brightness')}
          unit="%"
          min={0}
          max={100}
          value={value}
          onChange={setValue}
          onCommit={commit}
        />
      )}
      {on && kind === 'cover' && (
        <SliderControl
          label={t('devices.level')}
          unit="%"
          min={0}
          max={100}
          value={value}
          onChange={setValue}
          onCommit={commit}
        />
      )}
      {on && kind === 'fan' && (
        <LevelControl
          active={valueToLevel(value)}
          onPick={(level) => commit(levelToValue(level))}
          labels={[t('devices.levelLow'), t('devices.levelMid'), t('devices.levelHigh')]}
        />
      )}
    </div>
  );
}

function SliderControl({
  label,
  unit,
  min,
  max,
  value,
  onChange,
  onCommit,
}: {
  label: string;
  unit: string;
  min: number;
  max: number;
  value: number;
  onChange: (v: number) => void;
  onCommit: (v: number) => void;
}) {
  const clamped = Math.min(Math.max(value, min), max);
  return (
    <div className="mt-3">
      <div className="flex items-center justify-between mb-1.5">
        <span className="text-xs text-slate-500 dark:text-slate-400">{label}</span>
        <span className="text-xs font-semibold text-primary">
          {clamped}
          {unit}
        </span>
      </div>
      <input
        type="range"
        className="range-slider"
        min={min}
        max={max}
        value={clamped}
        onChange={(e) => onChange(Number(e.target.value))}
        onMouseUp={(e) => onCommit(Number((e.target as HTMLInputElement).value))}
        onTouchEnd={(e) => onCommit(Number((e.target as HTMLInputElement).value))}
        onKeyUp={(e) => onCommit(Number((e.target as HTMLInputElement).value))}
      />
    </div>
  );
}

function LevelControl({
  active,
  onPick,
  labels,
}: {
  active: 1 | 2 | 3;
  onPick: (level: 1 | 2 | 3) => void;
  labels: [string, string, string];
}) {
  const levels: (1 | 2 | 3)[] = [1, 2, 3];
  return (
    <div className="mt-3 flex gap-1.5">
      {levels.map((l, i) => (
        <button
          key={l}
          onClick={() => onPick(l)}
          className={`flex-1 py-1.5 rounded-lg text-xs font-semibold transition ${
            active === l
              ? 'bg-primary text-white'
              : 'bg-gray-200 dark:bg-dark-surface text-slate-700 dark:text-slate-200'
          }`}
        >
          {labels[i]}
        </button>
      ))}
    </div>
  );
}

function valueToLevel(v: number): 1 | 2 | 3 {
  if (v <= 40) return 1;
  if (v <= 75) return 2;
  return 3;
}

function levelToValue(l: 1 | 2 | 3): number {
  return l === 1 ? 33 : l === 2 ? 66 : 100;
}
