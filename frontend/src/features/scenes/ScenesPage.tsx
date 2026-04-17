import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { FormEvent, useState } from 'react';
import { Plus, Play, Trash2 } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Modal } from '@/components/ui/Modal';
import { sceneApi, deviceApi } from '@/api/endpoints';
import { errorMessage } from '@/api/client';
import { useActiveClassroom } from '@/stores/classroom';
import { ClassroomPicker } from '@/features/common/ClassroomGate';
import type { SceneStep, CommandType } from '@/api/types';

export function ScenesPage() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const activeID = useActiveClassroom((s) => s.id);
  const [showAdd, setShowAdd] = useState(false);
  const [runResult, setRunResult] = useState<string | null>(null);

  const scenesQ = useQuery({
    queryKey: ['scenes', activeID],
    queryFn: () => sceneApi.listByClassroom(activeID!),
    enabled: !!activeID,
  });

  const runMut = useMutation({
    mutationFn: (id: string) => sceneApi.run(id),
    onSuccess: (res) => {
      const failed = res.steps.filter((s) => !s.success).length;
      setRunResult(failed === 0 ? `${res.steps.length} steps OK` : `${failed}/${res.steps.length} failed`);
      setTimeout(() => setRunResult(null), 3500);
    },
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => sceneApi.remove(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['scenes', activeID] }),
  });

  return (
    <div className="p-4 flex flex-col gap-4 animate-fadeIn">
      <header className="flex items-center justify-between pt-2">
        <h1 className="text-2xl font-bold text-primary">{t('scenes.title')}</h1>
        <Button onClick={() => setShowAdd(true)} disabled={!activeID}>
          <Plus size={16} className="inline -mt-0.5 mr-1" />
          {t('scenes.add')}
        </Button>
      </header>

      <ClassroomPicker />

      {runResult && <Card className="bg-accent/10 text-accent font-semibold text-sm">{runResult}</Card>}

      {activeID && scenesQ.data && scenesQ.data.length === 0 && (
        <Card className="text-center text-slate-500">{t('scenes.empty')}</Card>
      )}

      <div className="flex flex-col gap-3">
        {(scenesQ.data ?? []).map((sc) => (
          <Card key={sc.id} className="flex items-center justify-between">
            <div>
              <p className="font-semibold text-primary">{sc.name}</p>
              <p className="text-xs text-slate-500">{sc.steps.length} steps</p>
            </div>
            <div className="flex gap-2">
              <button
                onClick={() => runMut.mutate(sc.id)}
                className="p-2 rounded-xl bg-accent text-white"
                aria-label={t('scenes.run')}
              >
                <Play size={18} />
              </button>
              <button onClick={() => deleteMut.mutate(sc.id)} className="p-2 rounded-xl text-slate-400 hover:text-danger">
                <Trash2 size={16} />
              </button>
            </div>
          </Card>
        ))}
      </div>

      <Modal open={showAdd} onClose={() => setShowAdd(false)} title={t('scenes.add')}>
        {activeID && (
          <AddSceneForm
            classroomID={activeID}
            onCreated={() => {
              qc.invalidateQueries({ queryKey: ['scenes', activeID] });
              setShowAdd(false);
            }}
          />
        )}
      </Modal>
    </div>
  );
}

function AddSceneForm({ classroomID, onCreated }: { classroomID: string; onCreated: () => void }) {
  const { t } = useTranslation();
  const devicesQ = useQuery({
    queryKey: ['devices', classroomID],
    queryFn: () => deviceApi.listByClassroom(classroomID),
  });
  const [name, setName] = useState('');
  const [steps, setSteps] = useState<SceneStep[]>([]);
  const [err, setErr] = useState('');
  const [loading, setLoading] = useState(false);

  const addStep = () => {
    const firstDevice = devicesQ.data?.[0];
    if (!firstDevice) return;
    setSteps((s) => [...s, { deviceId: firstDevice.id, command: 'ON' }]);
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr('');
    setLoading(true);
    try {
      await sceneApi.create({ classroomId: classroomID, name, steps });
      onCreated();
    } catch (e) {
      setErr(errorMessage(e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      <Input label={t('scenes.name')} value={name} onChange={(e) => setName(e.target.value)} required />
      <div className="flex flex-col gap-2">
        {steps.map((s, i) => (
          <div key={i} className="flex gap-2 items-center">
            <select
              className="input-field flex-1"
              value={s.deviceId}
              onChange={(e) =>
                setSteps((arr) => arr.map((it, j) => (j === i ? { ...it, deviceId: e.target.value } : it)))
              }
            >
              {(devicesQ.data ?? []).map((d) => (
                <option key={d.id} value={d.id}>{d.name}</option>
              ))}
            </select>
            <select
              className="input-field w-28"
              value={s.command}
              onChange={(e) =>
                setSteps((arr) => arr.map((it, j) => (j === i ? { ...it, command: e.target.value as CommandType } : it)))
              }
            >
              {['ON', 'OFF', 'OPEN', 'CLOSE'].map((c) => (
                <option key={c} value={c}>{c}</option>
              ))}
            </select>
            <button type="button" onClick={() => setSteps((arr) => arr.filter((_, j) => j !== i))} className="text-slate-400 hover:text-danger">
              <Trash2 size={16} />
            </button>
          </div>
        ))}
        <Button type="button" variant="soft" onClick={addStep} disabled={!devicesQ.data?.length}>
          {t('scenes.addStep')}
        </Button>
      </div>
      {err && <p className="text-sm text-danger">{err}</p>}
      <Button type="submit" disabled={loading}>{t('common.create')}</Button>
    </form>
  );
}
