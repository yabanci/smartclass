// Curated catalogue of IoT brands the UI knows by name, each mapped to the
// Home Assistant integration domains that pair its devices. `handlers` is a
// priority-ordered list — the wizard prefers the first one HA actually
// reports as available. `pairing` drives the "how to prepare" hint shown to
// the user: cloud brands need a vendor-app account, LAN brands just need the
// device on the same WiFi as the backend.
export type Pairing = 'cloud' | 'lan';

export type Brand = {
  id: string;
  label: string;
  emoji: string;
  handlers: string[];
  pairing: Pairing;
};

export const BRANDS: Brand[] = [
  {
    id: 'xiaomi',
    label: 'Xiaomi · Mi Home · Yeelight',
    emoji: '🏮',
    handlers: ['xiaomi_miio', 'yeelight', 'xiaomi_miot', 'xiaomi_bluetooth'],
    pairing: 'cloud',
  },
  {
    id: 'aqara',
    label: 'Aqara',
    emoji: '🔷',
    handlers: ['aqara_gateway', 'xiaomi_aqara'],
    pairing: 'cloud',
  },
  {
    id: 'samsung',
    label: 'Samsung SmartThings',
    emoji: '📱',
    handlers: ['smartthings'],
    pairing: 'cloud',
  },
  {
    id: 'tuya',
    label: 'Tuya · Smart Life',
    emoji: '🟪',
    handlers: ['tuya'],
    pairing: 'cloud',
  },
  {
    id: 'hue',
    label: 'Philips Hue',
    emoji: '💡',
    handlers: ['hue'],
    pairing: 'lan',
  },
  {
    id: 'tplink',
    label: 'TP-Link Tapo · Kasa',
    emoji: '🟢',
    handlers: ['tplink'],
    pairing: 'lan',
  },
  {
    id: 'shelly',
    label: 'Shelly',
    emoji: '⚡',
    handlers: ['shelly'],
    pairing: 'lan',
  },
  {
    id: 'sonoff',
    label: 'Sonoff',
    emoji: '🔌',
    handlers: ['sonoff'],
    pairing: 'lan',
  },
  {
    id: 'mqtt',
    label: 'MQTT',
    emoji: '📨',
    handlers: ['mqtt'],
    pairing: 'lan',
  },
];

export function resolveHandler<T extends { domain: string }>(brand: Brand, available: T[]): T | null {
  for (const d of brand.handlers) {
    const m = available.find((i) => i.domain === d);
    if (m) return m;
  }
  return null;
}
