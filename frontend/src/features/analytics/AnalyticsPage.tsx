import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { useMemo, useState } from 'react';
import { Card } from '@/components/ui/Card';
import { analyticsApi } from '@/api/endpoints';
import { useActiveClassroom } from '@/stores/classroom';
import { ClassroomPicker } from '@/features/common/ClassroomGate';

export function AnalyticsPage() {
  const { t } = useTranslation();
  const activeID = useActiveClassroom((s) => s.id);
  const [metric, setMetric] = useState('temperature');
  const [bucket, setBucket] = useState<'hour' | 'day' | 'week' | 'month'>('hour');

  const range = useMemo(() => {
    const to = new Date();
    const from = new Date(to.getTime() - 7 * 24 * 3600 * 1000);
    return { from: from.toISOString(), to: to.toISOString() };
  }, []);

  const seriesQ = useQuery({
    queryKey: ['analytics-sensors', activeID, metric, bucket],
    queryFn: () => analyticsApi.sensors(activeID!, { metric, bucket, ...range }),
    enabled: !!activeID,
  });

  const usageQ = useQuery({
    queryKey: ['analytics-usage', activeID],
    queryFn: () => analyticsApi.usage(activeID!, range),
    enabled: !!activeID,
  });

  const energyQ = useQuery({
    queryKey: ['analytics-energy', activeID],
    queryFn: () => analyticsApi.energy(activeID!, range),
    enabled: !!activeID,
  });

  const maxY = seriesQ.data ? Math.max(...seriesQ.data.map((p) => p.max), 1) : 1;

  return (
    <div className="p-4 flex flex-col gap-4 animate-fadeIn">
      <header className="pt-2">
        <h1 className="text-2xl font-bold text-primary">{t('analytics.title')}</h1>
        <p className="text-sm text-slate-500">{t('analytics.lastWeek')}</p>
      </header>

      <ClassroomPicker />

      <Card>
        <div className="flex items-center justify-between mb-2">
          <span className="font-semibold text-primary">{t('analytics.sensorsSeries')}</span>
          <div className="flex gap-2">
            <select className="input-field !py-1 !text-xs" value={metric} onChange={(e) => setMetric(e.target.value)}>
              <option value="temperature">Temperature</option>
              <option value="humidity">Humidity</option>
              <option value="motion">Motion</option>
            </select>
            <select className="input-field !py-1 !text-xs" value={bucket} onChange={(e) => setBucket(e.target.value as 'hour' | 'day' | 'week' | 'month')}>
              <option value="hour">Hour</option>
              <option value="day">Day</option>
              <option value="week">Week</option>
            </select>
          </div>
        </div>
        {seriesQ.data && seriesQ.data.length > 0 ? (
          <div className="flex items-end gap-1 h-32 pt-2">
            {seriesQ.data.map((p) => (
              <div
                key={p.bucket}
                title={`${p.bucket}: avg ${p.avg.toFixed(1)}`}
                className="flex-1 bg-gradient-to-t from-secondary to-primary rounded-t"
                style={{ height: `${Math.max(5, (p.avg / maxY) * 100)}%` }}
              />
            ))}
          </div>
        ) : (
          <p className="text-sm text-slate-400 py-4 text-center">{t('common.empty')}</p>
        )}
      </Card>

      <Card>
        <p className="font-semibold text-primary mb-2">{t('analytics.deviceUsage')}</p>
        {usageQ.data && usageQ.data.length > 0 ? (
          <ul className="flex flex-col gap-1 text-sm">
            {usageQ.data.map((u) => (
              <li key={u.deviceId} className="flex justify-between">
                <span className="font-mono text-xs text-slate-500">{u.deviceId.slice(0, 8)}</span>
                <span className="font-semibold text-primary">{u.commandCount}</span>
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-sm text-slate-400 text-center py-3">{t('common.empty')}</p>
        )}
      </Card>

      <Card>
        <p className="font-semibold text-primary mb-2">{t('analytics.energy')}</p>
        <p className="text-3xl font-bold text-primary">
          {energyQ.data?.total?.toFixed(1) ?? '—'}
          <span className="text-base text-slate-400 ml-1">kWh</span>
        </p>
      </Card>
    </div>
  );
}
