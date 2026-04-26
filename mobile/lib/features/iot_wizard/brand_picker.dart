import 'package:flutter/material.dart';

class BrandInfo {
  final String name;
  final String emoji;
  final List<String> domains;
  final String pairing; // cloud | lan

  const BrandInfo({
    required this.name,
    required this.emoji,
    required this.domains,
    required this.pairing,
  });
}

const kBrands = [
  BrandInfo(
    name: 'Xiaomi/Mi Home',
    emoji: '📱',
    domains: ['xiaomi_home', 'xiaomi_miio'],
    pairing: 'cloud',
  ),
  BrandInfo(
    name: 'Samsung',
    emoji: '🌡️',
    domains: ['smartthings'],
    pairing: 'cloud',
  ),
  BrandInfo(
    name: 'Tuya/Smart Life',
    emoji: '💡',
    domains: ['tuya'],
    pairing: 'cloud',
  ),
  BrandInfo(
    name: 'Aqara',
    emoji: '🏠',
    domains: ['aqara', 'xiaomi_aqara'],
    pairing: 'cloud',
  ),
  BrandInfo(
    name: 'Sonoff',
    emoji: '🔌',
    domains: ['sonoff', 'ewelink'],
    pairing: 'cloud',
  ),
  BrandInfo(
    name: 'Shelly',
    emoji: '⚡',
    domains: ['shelly'],
    pairing: 'lan',
  ),
  BrandInfo(
    name: 'Generic MQTT',
    emoji: '📡',
    domains: ['mqtt'],
    pairing: 'lan',
  ),
];

class BrandPickerWidget extends StatelessWidget {
  final ValueChanged<BrandInfo> onBrandSelected;
  final VoidCallback onShowAll;

  const BrandPickerWidget({
    super.key,
    required this.onBrandSelected,
    required this.onShowAll,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const Text(
          'Pick a manufacturer',
          style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
        ),
        const SizedBox(height: 16),
        ...kBrands.map((brand) => Card(
              margin: const EdgeInsets.only(bottom: 8),
              child: ListTile(
                leading: Text(brand.emoji,
                    style: const TextStyle(fontSize: 28)),
                title: Text(brand.name),
                subtitle: Text(
                  brand.pairing == 'cloud' ? 'Cloud setup' : 'Local network',
                  style: TextStyle(
                    color: brand.pairing == 'cloud'
                        ? Colors.blue
                        : Colors.green,
                    fontSize: 12,
                  ),
                ),
                trailing: const Icon(Icons.chevron_right),
                onTap: () => onBrandSelected(brand),
              ),
            )),
        const SizedBox(height: 8),
        OutlinedButton.icon(
          icon: const Icon(Icons.grid_view),
          label: const Text('Show all integrations'),
          onPressed: onShowAll,
        ),
      ],
    );
  }
}
