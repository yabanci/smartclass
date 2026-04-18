import { FormEvent, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { deviceApi, hassApi } from '@/api/endpoints';
import { errorMessage } from '@/api/client';
import type { Device } from '@/api/types';

type DriverID = 'homeassistant' | 'smartthings' | 'generic_http';

const DRIVERS: { id: DriverID; label: string; hint: string }[] = [
  {
    id: 'homeassistant',
    label: 'Home Assistant',
    hint: 'Xiaomi, Tuya, Aqara, Samsung, Zigbee — через HA',
  },
  {
    id: 'smartthings',
    label: 'SmartThings',
    hint: 'Прямое подключение к Samsung SmartThings cloud',
  },
  {
    id: 'generic_http',
    label: 'Generic HTTP',
    hint: 'Shelly Gen1, Sonoff DIY, Tasmota — любая железка с REST API',
  },
];

const TYPES = ['light', 'switch', 'relay', 'cover', 'lock', 'climate', 'fan', 'sensor', 'projector', 'other'];
const BRANDS = ['generic', 'xiaomi', 'samsung', 'tuya', 'aqara', 'shelly', 'sonoff', 'philips', 'huawei', 'other'];

type GenericCommand = { method: string; path: string };
type GenericConfig = {
  baseUrl: string;
  commands: Partial<Record<'ON' | 'OFF' | 'OPEN' | 'CLOSE', GenericCommand>>;
  status?: GenericCommand;
};

interface Props {
  classroomID: string;
  device?: Device;
  onSubmitted: () => void;
}

// DeviceForm renders a friendly per-driver form instead of the raw JSON
// textarea. Same component handles create + edit; if `device` is passed we
// pre-populate state and call PATCH instead of POST.
export function DeviceForm({ classroomID, device, onSubmitted }: Props) {
  const { t } = useTranslation();
  const isEdit = !!device;

  const [name, setName] = useState(device?.name ?? '');
  const [type, setType] = useState(device?.type ?? 'light');
  const [brand, setBrand] = useState(BRANDS.includes(device?.brand ?? '') ? device!.brand : (device ? 'other' : 'generic'));
  const [brandOther, setBrandOther] = useState(BRANDS.includes(device?.brand ?? '') ? '' : (device?.brand ?? ''));
  const [driver, setDriver] = useState<DriverID>(((device?.driver ?? 'homeassistant') as DriverID));
  const [config, setConfig] = useState<Record<string, unknown>>(device?.config ?? {});
  const [showJson, setShowJson] = useState(false);
  const [jsonText, setJsonText] = useState(JSON.stringify(device?.config ?? {}, null, 2));
  const [jsonErr, setJsonErr] = useState('');
  const [err, setErr] = useState('');
  const [loading, setLoading] = useState(false);

  // When user toggles JSON view off, sync from text → typed state.
  useEffect(() => {
    if (showJson) setJsonText(JSON.stringify(config, null, 2));
  }, [showJson, config]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr('');
    setLoading(true);
    try {
      let cfg = config;
      if (showJson) {
        try {
          cfg = JSON.parse(jsonText || '{}');
          setJsonErr('');
        } catch (e) {
          setJsonErr(String(e));
          setLoading(false);
          return;
        }
      }
      const finalBrand = brand === 'other' ? brandOther.trim() || 'generic' : brand;
      if (isEdit) {
        await deviceApi.update(device!.id, { name, type, brand: finalBrand, driver, config: cfg });
      } else {
        await deviceApi.create({
          classroomId: classroomID,
          name, type, brand: finalBrand, driver, config: cfg,
        });
      }
      onSubmitted();
    } catch (e) {
      setErr(errorMessage(e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      <Input label={t('devices.name')} value={name} onChange={(e) => setName(e.target.value)} required />

      <div className="grid grid-cols-2 gap-3">
        <Select label={t('devices.type')} value={type} onChange={setType}>
          {TYPES.map((v) => (
            <option key={v} value={v}>{v}</option>
          ))}
        </Select>
        <Select label={t('devices.brand')} value={brand} onChange={setBrand}>
          {BRANDS.map((v) => (
            <option key={v} value={v}>{v}</option>
          ))}
        </Select>
      </div>
      {brand === 'other' && (
        <Input label="Custom brand" value={brandOther} onChange={(e) => setBrandOther(e.target.value)} placeholder="e.g. lifx, ecobee" />
      )}

      <DriverPicker value={driver} onChange={(v) => { setDriver(v); setConfig({}); }} />

      {!showJson && driver === 'homeassistant' && (
        <HomeAssistantFields config={config} onChange={setConfig} />
      )}
      {!showJson && driver === 'smartthings' && (
        <SmartThingsFields config={config} onChange={setConfig} />
      )}
      {!showJson && driver === 'generic_http' && (
        <GenericHttpFields config={(config as unknown) as GenericConfig} onChange={(c) => setConfig(c as unknown as Record<string, unknown>)} />
      )}

      <button
        type="button"
        className="text-xs text-slate-500 hover:text-primary self-start"
        onClick={() => setShowJson((v) => !v)}
      >
        {showJson ? '← Скрыть JSON' : 'Показать advanced (JSON) →'}
      </button>
      {showJson && (
        <label className="block">
          <span className="mb-1 block text-xs font-semibold text-slate-600">{t('devices.config')}</span>
          <textarea
            className="input-field font-mono text-xs"
            rows={8}
            value={jsonText}
            onChange={(e) => setJsonText(e.target.value)}
          />
          {jsonErr && <p className="text-xs text-danger mt-1">{jsonErr}</p>}
        </label>
      )}

      {err && <p className="text-sm text-danger">{err}</p>}
      <Button type="submit" disabled={loading}>
        {isEdit ? t('common.save') : t('common.create')}
      </Button>
    </form>
  );
}

function Select({
  label, value, onChange, children,
}: { label: string; value: string; onChange: (v: string) => void; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-xs font-semibold text-slate-600">{label}</span>
      <select className="input-field" value={value} onChange={(e) => onChange(e.target.value)}>
        {children}
      </select>
    </label>
  );
}

function DriverPicker({ value, onChange }: { value: DriverID; onChange: (v: DriverID) => void }) {
  const { t } = useTranslation();
  return (
    <div>
      <span className="mb-1 block text-xs font-semibold text-slate-600">{t('devices.driver')}</span>
      <div className="flex flex-col gap-1.5">
        {DRIVERS.map((d) => (
          <label
            key={d.id}
            className={`flex items-start gap-2 rounded-xl border p-2.5 cursor-pointer transition ${
              value === d.id ? 'border-primary bg-primary/5' : 'border-slate-200'
            }`}
          >
            <input
              type="radio"
              name="driver"
              checked={value === d.id}
              onChange={() => onChange(d.id)}
              className="mt-0.5"
            />
            <div className="flex-1 min-w-0">
              <p className="font-semibold text-sm text-primary">{d.label}</p>
              <p className="text-xs text-slate-500">{d.hint}</p>
            </div>
          </label>
        ))}
      </div>
    </div>
  );
}

function HomeAssistantFields({
  config, onChange,
}: { config: Record<string, unknown>; onChange: (c: Record<string, unknown>) => void }) {
  const status = useQuery({ queryKey: ['hass', 'status'], queryFn: () => hassApi.status() });
  const entities = useQuery({
    queryKey: ['hass', 'entities'],
    queryFn: () => hassApi.entities(),
    enabled: status.data?.configured === true,
  });

  const baseUrl = (config.baseUrl as string) ?? status.data?.baseUrl ?? '';
  const token = (config.token as string) ?? '';
  const entityId = (config.entityId as string) ?? '';

  // If creating fresh, pre-fill from /hass/status (one-shot effect).
  useEffect(() => {
    if (!config.baseUrl && status.data?.baseUrl) {
      onChange({ ...config, baseUrl: status.data.baseUrl });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status.data?.baseUrl]);

  return (
    <div className="flex flex-col gap-3">
      <Input
        label="HA Base URL"
        value={baseUrl}
        onChange={(e) => onChange({ ...config, baseUrl: e.target.value })}
        placeholder="http://homeassistant:8123"
        required
      />
      <Input
        label="HA Token"
        type="password"
        value={token}
        onChange={(e) => onChange({ ...config, token: e.target.value })}
        hint={status.data?.configured ? 'Можно оставить пустым — будет использован общий токен из /hass/status' : 'Long-lived access token из HA'}
      />
      {entities.data && entities.data.length > 0 ? (
        <div>
          <span className="mb-1 block text-xs font-semibold text-slate-600">Entity ID</span>
          <select
            className="input-field"
            value={entityId}
            onChange={(e) => onChange({ ...config, entityId: e.target.value })}
            required
          >
            <option value="">— выбери устройство —</option>
            {entities.data.map((e) => (
              <option key={e.entity_id} value={e.entity_id}>
                {e.friendly_name || e.entity_id} ({e.entity_id})
              </option>
            ))}
          </select>
          <p className="text-xs text-slate-400 mt-1">Подтянуто из HA автоматически</p>
        </div>
      ) : (
        <Input
          label="Entity ID"
          value={entityId}
          onChange={(e) => onChange({ ...config, entityId: e.target.value })}
          placeholder="light.kitchen"
          required
        />
      )}
    </div>
  );
}

function SmartThingsFields({
  config, onChange,
}: { config: Record<string, unknown>; onChange: (c: Record<string, unknown>) => void }) {
  return (
    <div className="flex flex-col gap-3">
      <Input
        label="SmartThings PAT"
        type="password"
        value={(config.token as string) ?? ''}
        onChange={(e) => onChange({ ...config, token: e.target.value })}
        placeholder="pat-..."
        hint="https://account.smartthings.com/tokens"
        required
      />
      <Input
        label="Device UUID"
        value={(config.deviceId as string) ?? ''}
        onChange={(e) => onChange({ ...config, deviceId: e.target.value })}
        placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
        required
      />
      <details className="text-sm">
        <summary className="cursor-pointer text-xs text-slate-500">Дополнительно (component, capability)</summary>
        <div className="mt-2 grid grid-cols-2 gap-3">
          <Input
            label="Component"
            value={(config.component as string) ?? ''}
            onChange={(e) => onChange({ ...config, component: e.target.value })}
            placeholder="main"
          />
          <Input
            label="Capability"
            value={(config.capability as string) ?? ''}
            onChange={(e) => onChange({ ...config, capability: e.target.value })}
            placeholder="switch / lock / windowShade"
          />
        </div>
      </details>
    </div>
  );
}

function GenericHttpFields({
  config, onChange,
}: { config: GenericConfig; onChange: (c: GenericConfig) => void }) {
  const cfg: GenericConfig = useMemo(
    () => ({
      baseUrl: config?.baseUrl ?? '',
      commands: config?.commands ?? {},
      status: config?.status,
    }),
    [config],
  );

  const setCommand = (key: 'ON' | 'OFF' | 'OPEN' | 'CLOSE', patch: Partial<GenericCommand>) => {
    const cur = cfg.commands[key] ?? { method: 'POST', path: '' };
    onChange({ ...cfg, commands: { ...cfg.commands, [key]: { ...cur, ...patch } } });
  };
  const removeCommand = (key: 'ON' | 'OFF' | 'OPEN' | 'CLOSE') => {
    const next = { ...cfg.commands };
    delete next[key];
    onChange({ ...cfg, commands: next });
  };

  return (
    <div className="flex flex-col gap-3">
      <Input
        label="Base URL"
        value={cfg.baseUrl}
        onChange={(e) => onChange({ ...cfg, baseUrl: e.target.value })}
        placeholder="http://192.168.1.100"
        required
      />
      <div>
        <span className="mb-1 block text-xs font-semibold text-slate-600">HTTP-команды</span>
        <div className="flex flex-col gap-2">
          {(['ON', 'OFF', 'OPEN', 'CLOSE'] as const).map((key) => {
            const c = cfg.commands[key];
            const enabled = !!c;
            return (
              <div key={key} className="flex items-center gap-2">
                <label className="flex items-center gap-1.5 w-20 text-xs font-mono">
                  <input
                    type="checkbox"
                    checked={enabled}
                    onChange={(e) => (e.target.checked ? setCommand(key, { method: 'POST', path: '' }) : removeCommand(key))}
                  />
                  {key}
                </label>
                <select
                  disabled={!enabled}
                  value={c?.method ?? 'POST'}
                  onChange={(e) => setCommand(key, { method: e.target.value })}
                  className="input-field !py-1.5 !w-20 text-xs"
                >
                  <option>GET</option>
                  <option>POST</option>
                  <option>PUT</option>
                  <option>DELETE</option>
                </select>
                <input
                  disabled={!enabled}
                  value={c?.path ?? ''}
                  onChange={(e) => setCommand(key, { path: e.target.value })}
                  placeholder="/relay/0?turn=on"
                  className="input-field !py-1.5 flex-1 text-xs font-mono"
                />
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
