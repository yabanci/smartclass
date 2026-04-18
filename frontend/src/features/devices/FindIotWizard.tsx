import { useState, useEffect, useMemo, FormEvent } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { Search, ArrowRight, Plus, AlertTriangle, CheckCircle2, WifiOff } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { hassApi, HassFlowHandler, HassFlowStep, HassSchemaField, HassEntity } from '@/api/endpoints';
import { errorMessage } from '@/api/client';
import { BRANDS, Brand, resolveHandler } from './brands';

type View = 'home' | 'pick-brand' | 'pick-integration' | 'brand-hint' | 'wizard' | 'done';

// FindIotWizard drives our backend's HA proxy: status → brand/integration →
// flow → entity discovery. The form renderer (SchemaFieldInput) is
// intentionally dumb — it walks HA's `data_schema` array and renders the
// right input per field type, so any new HA integration works without code
// changes here.
export function FindIotWizard({ classroomID, onAdopted }: { classroomID: string; onAdopted: () => void }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [view, setView] = useState<View>('home');
  const [chosen, setChosen] = useState<HassFlowHandler | null>(null);
  const [brand, setBrand] = useState<Brand | null>(null);
  const [step, setStep] = useState<HassFlowStep | null>(null);
  const [stepErr, setStepErr] = useState('');
  const [justAdopted, setJustAdopted] = useState<HassEntity | null>(null);

  const status = useQuery({
    queryKey: ['hass', 'status'],
    queryFn: () => hassApi.status(),
    refetchInterval: (q) => (q.state.data?.configured ? false : 5000),
  });

  const integrations = useQuery({
    queryKey: ['hass', 'integrations'],
    queryFn: () => hassApi.integrations(),
    enabled: status.data?.configured === true,
  });

  const entities = useQuery({
    queryKey: ['hass', 'entities'],
    queryFn: () => hassApi.entities(),
    enabled: status.data?.configured === true,
  });

  const startMut = useMutation({
    mutationFn: (h: HassFlowHandler) => hassApi.startFlow(h.domain),
    onSuccess: (s, h) => {
      setChosen(h);
      setStep(s);
      setStepErr('');
      setView('wizard');
    },
    onError: (e) => setStepErr(errorMessage(e)),
  });

  const stepMut = useMutation({
    mutationFn: (data: Record<string, unknown>) => hassApi.stepFlow(step!.flow_id!, data),
    onSuccess: (s) => {
      setStep(s);
      setStepErr('');
      if (s.type === 'create_entry' || s.type === 'abort') {
        qc.invalidateQueries({ queryKey: ['hass', 'entities'] });
        if (s.type === 'create_entry') setView('done');
      }
    },
    onError: (e) => setStepErr(errorMessage(e)),
  });

  const adoptMut = useMutation({
    mutationFn: (entity: HassEntity) =>
      hassApi.adopt({ entityId: entity.entity_id, classroomId: classroomID }).then(() => entity),
    onSuccess: (entity) => {
      setJustAdopted(entity);
      qc.invalidateQueries({ queryKey: ['devices', classroomID] });
      onAdopted();
    },
  });

  useEffect(() => {
    return () => {
      setView('home');
      setStep(null);
      setChosen(null);
      setBrand(null);
      setJustAdopted(null);
    };
  }, []);

  if (status.isLoading) return <p className="text-slate-500">{t('common.loading')}</p>;

  if (!status.data?.configured) {
    return <NotReadyView reason={status.data?.reason} onTokenSaved={() => status.refetch()} />;
  }

  const pickBrand = (b: Brand) => {
    const handler = resolveHandler(b, integrations.data ?? []);
    if (!handler) {
      setStepErr(t('hass.brandNotAvailable'));
      return;
    }
    setBrand(b);
    setChosen(handler);
    setStepErr('');
    setView('brand-hint');
  };

  return (
    <div className="flex flex-col gap-3">
      {view === 'home' && (
        <HomeView
          entities={entities.data ?? []}
          loading={entities.isLoading}
          onPair={() => {
            setStepErr('');
            setView('pick-brand');
          }}
          onAdopt={(e) => adoptMut.mutate(e)}
          adopting={adoptMut.isPending}
          adoptError={adoptMut.error ? errorMessage(adoptMut.error) : ''}
          justAdopted={justAdopted}
          onDismissAdopted={() => setJustAdopted(null)}
        />
      )}

      {view === 'pick-brand' && (
        <BrandPicker
          available={integrations.data ?? []}
          onPick={pickBrand}
          onAll={() => {
            setStepErr('');
            setView('pick-integration');
          }}
          onBack={() => setView('home')}
          error={stepErr}
        />
      )}

      {view === 'brand-hint' && brand && chosen && (
        <BrandHint
          brand={brand}
          handler={chosen}
          onContinue={() => startMut.mutate(chosen)}
          onBack={() => {
            setBrand(null);
            setChosen(null);
            setView('pick-brand');
          }}
          starting={startMut.isPending}
        />
      )}

      {view === 'pick-integration' && (
        <IntegrationPicker
          items={integrations.data ?? []}
          loading={integrations.isLoading}
          onPick={(h) => startMut.mutate(h)}
          starting={startMut.isPending}
          error={stepErr}
          onBack={() => setView('pick-brand')}
        />
      )}

      {view === 'wizard' && step && chosen && (
        <WizardStep
          step={step}
          handler={chosen}
          onSubmit={(data) => stepMut.mutate(data)}
          submitting={stepMut.isPending}
          error={stepErr}
          onAbort={() => {
            if (step.flow_id) hassApi.abortFlow(step.flow_id).catch(() => {});
            setView('home');
            setStep(null);
          }}
        />
      )}

      {view === 'done' && (
        <Card className="text-center">
          <p className="text-lg font-bold text-accent">{t('hass.created')}</p>
          <p className="text-sm text-slate-500 mt-1">{step?.title ?? chosen?.name}</p>
          <Button className="mt-3" onClick={() => setView('home')}>
            {t('hass.discoveredEntities')}
          </Button>
        </Card>
      )}
    </div>
  );
}

function NotReadyView({ reason, onTokenSaved }: { reason?: string; onTokenSaved: () => void }) {
  const { t } = useTranslation();
  const [token, setToken] = useState('');
  const [err, setErr] = useState('');
  const tokenMut = useMutation({
    mutationFn: () => hassApi.setToken(token),
    onSuccess: () => onTokenSaved(),
    onError: (e) => setErr(errorMessage(e)),
  });
  const alreadyManual = !!reason && /already_onboarded|already onboard/i.test(reason);
  return (
    <Card>
      <div className="flex items-start gap-2">
        <AlertTriangle className="text-amber-500 shrink-0 mt-0.5" size={18} />
        <div className="flex-1">
          <p className="text-sm text-slate-700">
            {alreadyManual ? t('hass.alreadySetup') : t('hass.notReady')}
          </p>
          {alreadyManual && (
            <form
              className="mt-3 flex flex-col gap-2"
              onSubmit={(e) => {
                e.preventDefault();
                tokenMut.mutate();
              }}
            >
              <Input
                placeholder="eyJhbGciOi..."
                value={token}
                onChange={(e) => setToken(e.target.value)}
                required
                minLength={20}
              />
              {err && <p className="text-xs text-danger">{err}</p>}
              <Button type="submit" disabled={tokenMut.isPending}>
                {t('hass.saveToken')}
              </Button>
            </form>
          )}
        </div>
      </div>
    </Card>
  );
}

function HomeView({
  entities,
  loading,
  onPair,
  onAdopt,
  adopting,
  adoptError,
  justAdopted,
  onDismissAdopted,
}: {
  entities: HassEntity[];
  loading: boolean;
  onPair: () => void;
  onAdopt: (entity: HassEntity) => void;
  adopting: boolean;
  adoptError: string;
  justAdopted: HassEntity | null;
  onDismissAdopted: () => void;
}) {
  const { t } = useTranslation();
  return (
    <>
      <Button onClick={onPair}>
        <Plus size={16} className="inline -mt-0.5 mr-1" />
        {t('hass.pickBrand')}
      </Button>

      {justAdopted && <AdoptResult entity={justAdopted} onDismiss={onDismissAdopted} />}

      <div>
        <p className="text-xs font-semibold text-slate-600 mb-2">{t('hass.discoveredEntities')}</p>
        {loading && <p className="text-sm text-slate-500">{t('hass.loadingEntities')}</p>}
        {!loading && entities.length === 0 && (
          <Card className="text-center text-sm text-slate-500">{t('hass.noEntities')}</Card>
        )}
        {adoptError && <p className="text-xs text-danger mb-2">{adoptError}</p>}
        <div className="flex flex-col gap-2">
          {entities.map((e) => (
            <Card key={e.entity_id} className="!p-3">
              <div className="flex items-center justify-between gap-2">
                <div className="flex-1 min-w-0">
                  <p className="font-semibold text-sm text-primary truncate">
                    {e.friendly_name || e.entity_id}
                  </p>
                  <p className="text-xs text-slate-500">
                    {e.domain} · {e.state}
                  </p>
                </div>
                <Button
                  variant="ghost"
                  className="!py-1.5 !px-3 text-xs"
                  disabled={adopting}
                  onClick={() => onAdopt(e)}
                >
                  {t('hass.addToClassroom')}
                </Button>
              </div>
            </Card>
          ))}
        </div>
      </div>
    </>
  );
}

function AdoptResult({ entity, onDismiss }: { entity: HassEntity; onDismiss: () => void }) {
  const { t } = useTranslation();
  const offline = /unavailable|unknown/i.test(entity.state);
  return (
    <Card className={offline ? '!bg-amber-50 !border-amber-200' : '!bg-emerald-50 !border-emerald-200'}>
      <div className="flex items-start gap-2">
        {offline ? (
          <WifiOff size={18} className="text-amber-600 shrink-0 mt-0.5" />
        ) : (
          <CheckCircle2 size={18} className="text-emerald-600 shrink-0 mt-0.5" />
        )}
        <div className="flex-1">
          <p className="text-sm font-semibold text-slate-800">
            {entity.friendly_name || entity.entity_id}
          </p>
          <p className="text-xs text-slate-700 mt-0.5">
            {offline
              ? t('hass.verifyOffline')
              : t('hass.verifyOk', { state: entity.state })}
          </p>
        </div>
        <button onClick={onDismiss} className="text-xs text-slate-500 shrink-0">
          ×
        </button>
      </div>
    </Card>
  );
}

function BrandPicker({
  available,
  onPick,
  onAll,
  onBack,
  error,
}: {
  available: HassFlowHandler[];
  onPick: (b: Brand) => void;
  onAll: () => void;
  onBack: () => void;
  error: string;
}) {
  const { t } = useTranslation();
  return (
    <>
      <button onClick={onBack} className="text-xs text-slate-500 self-start">
        ← {t('common.cancel')}
      </button>
      <p className="text-sm font-semibold text-slate-700">{t('hass.pickBrand')}</p>
      {error && <p className="text-xs text-danger">{error}</p>}
      <div className="grid grid-cols-2 gap-2">
        {BRANDS.map((b) => {
          const ok = !!resolveHandler(b, available);
          return (
            <button
              key={b.id}
              onClick={() => onPick(b)}
              disabled={!ok}
              className="text-left p-3 rounded-xl border border-slate-200 hover:border-primary disabled:opacity-40 disabled:cursor-not-allowed bg-white/60 flex flex-col gap-1"
            >
              <span className="text-2xl leading-none">{b.emoji}</span>
              <span className="text-xs font-semibold text-slate-800 leading-tight">{b.label}</span>
              <span className="text-[10px] uppercase tracking-wide text-slate-400">
                {b.pairing === 'cloud' ? '☁️ cloud' : '🏠 LAN'}
              </span>
            </button>
          );
        })}
      </div>
      <button onClick={onAll} className="mt-2 text-xs text-primary font-medium self-center">
        {t('hass.allBrands')} →
      </button>
    </>
  );
}

function BrandHint({
  brand,
  handler,
  onContinue,
  onBack,
  starting,
}: {
  brand: Brand;
  handler: HassFlowHandler;
  onContinue: () => void;
  onBack: () => void;
  starting: boolean;
}) {
  const { t } = useTranslation();
  return (
    <>
      <button onClick={onBack} className="text-xs text-slate-500 self-start">
        ← {t('common.cancel')}
      </button>
      <Card className="!bg-sky-50 !border-sky-200">
        <div className="flex items-start gap-3">
          <span className="text-3xl leading-none">{brand.emoji}</span>
          <div className="flex-1">
            <p className="font-semibold text-sm text-slate-800">{brand.label}</p>
            <p className="text-xs text-slate-500 mt-0.5">
              {handler.name} · {handler.domain}
            </p>
            <p className="text-sm text-slate-700 mt-3">
              {brand.pairing === 'cloud' ? t('hass.cloudHint') : t('hass.lanHint')}
            </p>
          </div>
        </div>
      </Card>
      <Button onClick={onContinue} disabled={starting}>
        {t('hass.next')} →
      </Button>
    </>
  );
}

function IntegrationPicker({
  items,
  loading,
  onPick,
  starting,
  error,
  onBack,
}: {
  items: HassFlowHandler[];
  loading: boolean;
  onPick: (h: HassFlowHandler) => void;
  starting: boolean;
  error: string;
  onBack: () => void;
}) {
  const { t } = useTranslation();
  const [q, setQ] = useState('');
  const filtered = useMemo(
    () =>
      items
        .filter((i) => i.config_flow !== false)
        .filter((i) =>
          [i.name, i.domain, i.integration].some((s) =>
            (s ?? '').toLowerCase().includes(q.toLowerCase()),
          ),
        )
        .sort((a, b) => a.name.localeCompare(b.name))
        .slice(0, 60),
    [items, q],
  );
  return (
    <>
      <button onClick={onBack} className="text-xs text-slate-500 self-start">
        ← {t('common.cancel')}
      </button>
      <div className="relative">
        <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
        <input
          className="input-field pl-9"
          placeholder={t('hass.searchIntegration')}
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
      </div>
      {error && <p className="text-xs text-danger">{error}</p>}
      {loading && <p className="text-sm text-slate-500">{t('common.loading')}</p>}
      <div className="flex flex-col gap-1 max-h-[50vh] overflow-y-auto">
        {filtered.map((h) => (
          <button
            key={h.domain}
            disabled={starting}
            onClick={() => onPick(h)}
            className="text-left px-3 py-2 rounded-xl hover:bg-slate-100 flex items-center justify-between gap-2"
          >
            <div className="flex-1 min-w-0">
              <p className="font-semibold text-sm truncate">{h.name}</p>
              <p className="text-xs text-slate-400 truncate">
                {h.domain}
                {h.iot_class ? ` · ${h.iot_class}` : ''}
              </p>
            </div>
            <ArrowRight size={14} className="text-slate-400 shrink-0" />
          </button>
        ))}
      </div>
    </>
  );
}

function WizardStep({
  step,
  handler,
  onSubmit,
  submitting,
  error,
  onAbort,
}: {
  step: HassFlowStep;
  handler: HassFlowHandler;
  onSubmit: (data: Record<string, unknown>) => void;
  submitting: boolean;
  error: string;
  onAbort: () => void;
}) {
  const { t } = useTranslation();
  const [values, setValues] = useState<Record<string, unknown>>({});

  useEffect(() => {
    const init: Record<string, unknown> = {};
    (step.data_schema ?? []).forEach((f) => {
      if (f.default !== undefined) init[f.name] = f.default;
    });
    setValues(init);
  }, [step.flow_id, step.step_id]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    onSubmit(values);
  };

  if (step.type === 'abort') {
    return (
      <Card>
        <p className="font-semibold text-danger">{step.reason ?? 'aborted'}</p>
        <Button className="mt-3" onClick={onAbort}>
          {t('common.close')}
        </Button>
      </Card>
    );
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      <div>
        <p className="text-xs uppercase tracking-wide text-slate-400">{handler.name}</p>
        {step.description && <p className="text-sm text-slate-700 mt-1">{step.description}</p>}
      </div>
      {(step.data_schema ?? []).map((f) => (
        <SchemaFieldInput
          key={f.name}
          field={f}
          value={values[f.name]}
          fieldError={step.errors?.[f.name]}
          onChange={(v) => setValues((vs) => ({ ...vs, [f.name]: v }))}
        />
      ))}
      {error && <p className="text-xs text-danger">{error}</p>}
      <div className="flex gap-2">
        <Button type="button" variant="ghost" onClick={onAbort} className="flex-1">
          {t('hass.abort')}
        </Button>
        <Button type="submit" disabled={submitting} className="flex-1">
          {t('hass.next')}
        </Button>
      </div>
    </form>
  );
}

function SchemaFieldInput({
  field,
  value,
  fieldError,
  onChange,
}: {
  field: HassSchemaField;
  value: unknown;
  fieldError?: string;
  onChange: (v: unknown) => void;
}) {
  const label = `${field.name}${field.required ? ' *' : ''}`;
  if (field.options && field.options.length > 0) {
    return (
      <label className="block">
        <span className="mb-1 block text-xs font-semibold text-slate-600">{label}</span>
        <select
          className="input-field"
          value={String(value ?? '')}
          onChange={(e) => onChange(e.target.value)}
        >
          <option value="" />
          {field.options.map((o) => (
            <option key={String(o)} value={String(o)}>
              {String(o)}
            </option>
          ))}
        </select>
        {fieldError && <span className="mt-1 block text-xs text-danger">{fieldError}</span>}
      </label>
    );
  }
  if (field.type === 'boolean') {
    return (
      <label className="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          checked={Boolean(value)}
          onChange={(e) => onChange(e.target.checked)}
        />
        <span>{label}</span>
      </label>
    );
  }
  if (field.type === 'integer' || field.type === 'number' || field.type === 'float') {
    return (
      <Input
        label={label}
        type="number"
        value={value === undefined || value === null ? '' : String(value)}
        error={fieldError}
        onChange={(e) =>
          onChange(e.target.value === '' ? undefined : Number(e.target.value))
        }
        required={field.required}
      />
    );
  }
  const isPassword = /password|secret|token|api[_-]?key/i.test(field.name);
  return (
    <Input
      label={label}
      type={isPassword ? 'password' : 'text'}
      value={value === undefined || value === null ? '' : String(value)}
      error={fieldError}
      onChange={(e) => onChange(e.target.value)}
      required={field.required}
    />
  );
}
