import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../features/analytics/analytics_page.dart';
import '../../features/auth/login_page.dart';
import '../../features/auth/register_page.dart';
import '../../features/devices/devices_page.dart';
import '../../features/home/home_page.dart';
import '../../features/iot_wizard/iot_wizard_page.dart';
import '../../features/notifications/notifications_page.dart';
import '../../features/profile/profile_page.dart';
import '../../features/scenes/scenes_page.dart';
import '../../features/schedule/schedule_page.dart';
import '../../shared/providers/auth_provider.dart';
import '../../shared/providers/notification_provider.dart';
import '../i18n/app_localizations.dart';

final _rootNavigatorKey = GlobalKey<NavigatorState>();
final _shellNavigatorKey = GlobalKey<NavigatorState>();

final routerProvider = Provider<GoRouter>((ref) {
  final authState = ref.watch(authProvider);

  return GoRouter(
    navigatorKey: _rootNavigatorKey,
    initialLocation: '/',
    redirect: (context, state) {
      final isAuth = authState.isAuthenticated;
      final isAuthRoute = state.matchedLocation == '/login' ||
          state.matchedLocation == '/register';

      if (!isAuth && !isAuthRoute) return '/login';
      if (isAuth && isAuthRoute) return '/';
      return null;
    },
    routes: [
      GoRoute(
        path: '/login',
        builder: (context, state) => const LoginPage(),
      ),
      GoRoute(
        path: '/register',
        builder: (context, state) => const RegisterPage(),
      ),
      ShellRoute(
        navigatorKey: _shellNavigatorKey,
        builder: (context, state, child) =>
            _ScaffoldWithNavBar(child: child),
        routes: [
          GoRoute(
            path: '/',
            builder: (context, state) => const HomePage(),
          ),
          GoRoute(
            path: '/devices',
            builder: (context, state) => const DevicesPage(),
            routes: [
              GoRoute(
                path: 'iot-wizard',
                parentNavigatorKey: _rootNavigatorKey,
                builder: (context, state) => const IotWizardPage(),
              ),
            ],
          ),
          GoRoute(
            path: '/schedule',
            builder: (context, state) => const SchedulePage(),
          ),
          GoRoute(
            path: '/scenes',
            builder: (context, state) => const ScenesPage(),
          ),
          GoRoute(
            path: '/profile',
            builder: (context, state) => const ProfilePage(),
          ),
        ],
      ),
      GoRoute(
        path: '/notifications',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const NotificationsPage(),
      ),
      GoRoute(
        path: '/analytics',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const AnalyticsPage(),
      ),
    ],
  );
});

class _ScaffoldWithNavBar extends ConsumerWidget {
  final Widget child;
  const _ScaffoldWithNavBar({required this.child});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Scaffold(
      body: child,
      bottomNavigationBar: _BottomNav(),
    );
  }
}

class _BottomNav extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context)!;
    final location = GoRouterState.of(context).matchedLocation;
    final unreadAsync = ref.watch(unreadCountProvider);
    final unread = unreadAsync.valueOrNull ?? 0;

    int selectedIndex = 0;
    if (location.startsWith('/devices')) {
      selectedIndex = 1;
    } else if (location.startsWith('/schedule')) {
      selectedIndex = 2;
    } else if (location.startsWith('/scenes')) {
      selectedIndex = 3;
    } else if (location.startsWith('/profile')) {
      selectedIndex = 4;
    }

    return NavigationBar(
      selectedIndex: selectedIndex,
      onDestinationSelected: (index) {
        switch (index) {
          case 0:
            context.go('/');
          case 1:
            context.go('/devices');
          case 2:
            context.go('/schedule');
          case 3:
            context.go('/scenes');
          case 4:
            context.go('/profile');
        }
      },
      destinations: [
        NavigationDestination(
          icon: const Icon(Icons.home_outlined),
          selectedIcon: const Icon(Icons.home),
          label: l.navHome,
        ),
        NavigationDestination(
          icon: const Icon(Icons.devices_outlined),
          selectedIcon: const Icon(Icons.devices),
          label: l.navDevices,
        ),
        NavigationDestination(
          icon: const Icon(Icons.calendar_today_outlined),
          selectedIcon: const Icon(Icons.calendar_today),
          label: l.navSchedule,
        ),
        NavigationDestination(
          icon: const Icon(Icons.auto_awesome_outlined),
          selectedIcon: const Icon(Icons.auto_awesome),
          label: l.navScenes,
        ),
        NavigationDestination(
          icon: Badge(
            isLabelVisible: unread > 0,
            label: Text('$unread'),
            child: const Icon(Icons.person_outlined),
          ),
          selectedIcon: const Icon(Icons.person),
          label: l.navProfile,
        ),
      ],
    );
  }
}
