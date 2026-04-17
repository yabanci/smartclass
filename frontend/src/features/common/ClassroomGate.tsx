import { useQuery } from '@tanstack/react-query';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { classroomApi } from '@/api/endpoints';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { useActiveClassroom } from '@/stores/classroom';

export function ClassroomPicker({ onCreate }: { onCreate?: () => void }) {
  const { t } = useTranslation();
  const activeID = useActiveClassroom((s) => s.id);
  const setActive = useActiveClassroom((s) => s.set);

  const q = useQuery({ queryKey: ['classrooms'], queryFn: () => classroomApi.list() });

  useEffect(() => {
    if (!q.data) return;
    if (q.data.length === 0) {
      if (activeID) setActive(null);
      return;
    }
    if (!activeID || !q.data.find((c) => c.id === activeID)) {
      setActive(q.data[0].id);
    }
  }, [q.data, activeID, setActive]);

  if (q.isLoading) return <p className="p-4 text-slate-500">{t('common.loading')}</p>;

  if (!q.data || q.data.length === 0) {
    return (
      <Card className="text-center">
        <p className="mb-3 text-slate-600">{t('home.noClassroom')}</p>
        {onCreate && <Button onClick={onCreate}>{t('home.createClassroom')}</Button>}
      </Card>
    );
  }

  if (q.data.length === 1) return null;

  return (
    <select
      className="input-field"
      value={activeID ?? ''}
      onChange={(e) => setActive(e.target.value)}
    >
      {q.data.map((c) => (
        <option key={c.id} value={c.id}>{c.name}</option>
      ))}
    </select>
  );
}
