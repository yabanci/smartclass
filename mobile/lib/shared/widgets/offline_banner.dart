import 'dart:async';

import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:flutter/material.dart';

import '../../app.dart';
import '../../core/connection/connection_mode.dart';
import '../../core/connection/resolver.dart';
import '../../core/i18n/app_localizations.dart';

class OfflineBanner extends StatefulWidget {
  final Widget child;
  const OfflineBanner({super.key, required this.child});

  @override
  State<OfflineBanner> createState() => _OfflineBannerState();
}

class _OfflineBannerState extends State<OfflineBanner> {
  bool _offline = false;
  // C-016: separate flag for when network is up but server is unreachable.
  bool _unreachable = false;
  late StreamSubscription<List<ConnectivityResult>> _sub;

  @override
  void initState() {
    super.initState();
    _sub = Connectivity().onConnectivityChanged.listen((results) {
      final isOffline = results.every((r) => r == ConnectivityResult.none);
      if (!mounted) return;
      if (isOffline) {
        // No connectivity at all.
        setState(() {
          _offline = true;
          _unreachable = false;
        });
      } else {
        // Network is up — resolve server reachability.
        _checkReachability();
      }
    });
  }

  Future<void> _checkReachability() async {
    final state = await ConnectionResolver.instance.resolve();
    if (!mounted) return;
    setState(() {
      _offline = false;
      _unreachable = state.mode == ConnectionMode.unreachable;
    });
  }

  @override
  void dispose() {
    _sub.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final showBanner = _offline || _unreachable;
    final message = _offline
        ? l.offlineNoInternet
        // C-016: distinct message when server is unreachable.
        : l.offlineUnreachable;

    return Column(
      children: [
        AnimatedContainer(
          duration: const Duration(milliseconds: 300),
          height: showBanner ? 32 : 0,
          color: kDanger,
          child: showBanner
              ? Center(
                  child: Text(
                    message,
                    style: const TextStyle(color: Colors.white, fontSize: 12),
                  ),
                )
              : null,
        ),
        Expanded(child: widget.child),
      ],
    );
  }
}
