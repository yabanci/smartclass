import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:google_fonts/google_fonts.dart';

import 'config/app_config.dart';
import 'core/i18n/app_localizations.dart';
import 'core/router/router.dart';
import 'shared/widgets/offline_banner.dart';

final localeProvider = StateProvider<Locale>((ref) => const Locale('en'));

// Exact colors from tailwind.config.js
const kPrimary = Color(0xFF3A7BFF);
const kAccent = Color(0xFF34C759);
const kSecondary = Color(0xFF06B6D4);
const kWarn = Color(0xFFF59E0B);
const kDanger = Color(0xFFEF4444);
const kSurface = Color(0xFFF5F7FA);
const kDarkBg = Color(0xFF1a1a2e);
const kDarkCard = Color(0xFF16213e);
const kDarkSurface = Color(0xFF0f3460);

class SmartClassApp extends ConsumerWidget {
  const SmartClassApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final router = ref.watch(routerProvider);
    final locale = ref.watch(localeProvider);

    return MaterialApp.router(
      title: appConfig.appName,
      debugShowCheckedModeBanner: appConfig.debugBanner,
      locale: locale,
      supportedLocales: const [
        Locale('en'),
        Locale('ru'),
        Locale('kk'),
      ],
      localizationsDelegates: const [
        AppLocalizations.delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      theme: _buildTheme(Brightness.light),
      darkTheme: _buildTheme(Brightness.dark),
      routerConfig: router,
      builder: (context, child) => OfflineBanner(child: child ?? const SizedBox.shrink()),
    );
  }

  ThemeData _buildTheme(Brightness brightness) {
    final isDark = brightness == Brightness.dark;

    final colorScheme = ColorScheme(
      brightness: brightness,
      primary: kPrimary,
      onPrimary: Colors.white,
      primaryContainer: kPrimary.withOpacity(0.12),
      onPrimaryContainer: kPrimary,
      secondary: kAccent,
      onSecondary: Colors.white,
      secondaryContainer: kAccent.withOpacity(0.12),
      onSecondaryContainer: kAccent,
      tertiary: kSecondary,
      onTertiary: Colors.white,
      tertiaryContainer: kSecondary.withOpacity(0.12),
      onTertiaryContainer: kSecondary,
      error: kDanger,
      onError: Colors.white,
      errorContainer: kDanger.withOpacity(0.12),
      onErrorContainer: kDanger,
      surface: isDark ? kDarkCard : Colors.white,
      onSurface: isDark ? Colors.white : const Color(0xFF1e293b),
      surfaceContainerHighest: isDark ? kDarkSurface : kSurface,
      outline: isDark ? Colors.white12 : const Color(0xFF1e3a8a).withOpacity(0.1),
    );

    final textTheme = GoogleFonts.nunitoTextTheme(
      isDark ? ThemeData.dark().textTheme : ThemeData.light().textTheme,
    );

    return ThemeData(
      useMaterial3: true,
      brightness: brightness,
      colorScheme: colorScheme,
      scaffoldBackgroundColor: isDark ? kDarkBg : kSurface,
      textTheme: textTheme,
      appBarTheme: AppBarTheme(
        backgroundColor: isDark ? kDarkBg : kSurface,
        foregroundColor: isDark ? Colors.white : const Color(0xFF1e293b),
        elevation: 0,
        scrolledUnderElevation: 0,
        systemOverlayStyle: isDark
            ? SystemUiOverlayStyle.light
            : SystemUiOverlayStyle.dark,
        titleTextStyle: GoogleFonts.nunito(
          fontSize: 20,
          fontWeight: FontWeight.w700,
          color: isDark ? Colors.white : const Color(0xFF1e293b),
        ),
      ),
      cardTheme: CardThemeData(
        elevation: 0,
        color: isDark ? kDarkCard : Colors.white,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(16),
          side: BorderSide(
            color: isDark
                ? Colors.white.withOpacity(0.08)
                : const Color(0xFF1e3a8a).withOpacity(0.08),
          ),
        ),
        shadowColor: kPrimary.withOpacity(0.08),
        surfaceTintColor: Colors.transparent,
        margin: EdgeInsets.zero,
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: isDark
            ? const Color(0xFF0f3460).withOpacity(0.4)
            : Colors.white.withOpacity(0.5),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(
            color: const Color(0xFF1e3a8a).withOpacity(0.1),
          ),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(
            color: const Color(0xFF1e3a8a).withOpacity(0.1),
          ),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: kPrimary, width: 1.5),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: kDanger),
        ),
        focusedErrorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: kDanger, width: 1.5),
        ),
        contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
        labelStyle: TextStyle(
          color: isDark ? Colors.white60 : const Color(0xFF64748b),
          fontFamily: 'Nunito',
        ),
        hintStyle: TextStyle(
          color: isDark ? Colors.white38 : const Color(0xFF94a3b8),
          fontFamily: 'Nunito',
        ),
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: kPrimary,
          foregroundColor: Colors.white,
          elevation: 0,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
          padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
          textStyle: const TextStyle(
            fontFamily: 'Nunito',
            fontSize: 14,
            fontWeight: FontWeight.w700,
          ),
        ),
      ),
      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          backgroundColor: kPrimary,
          foregroundColor: Colors.white,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
          padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
          textStyle: const TextStyle(
            fontFamily: 'Nunito',
            fontSize: 14,
            fontWeight: FontWeight.w700,
          ),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          foregroundColor: kPrimary,
          side: const BorderSide(color: kPrimary),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
          padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
          textStyle: const TextStyle(
            fontFamily: 'Nunito',
            fontSize: 14,
            fontWeight: FontWeight.w700,
          ),
        ),
      ),
      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(
          foregroundColor: kPrimary,
          textStyle: const TextStyle(
            fontFamily: 'Nunito',
            fontSize: 14,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
      chipTheme: ChipThemeData(
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
        side: BorderSide.none,
      ),
      bottomNavigationBarTheme: BottomNavigationBarThemeData(
        backgroundColor: isDark ? kDarkCard : Colors.white,
        selectedItemColor: kPrimary,
        unselectedItemColor: isDark ? Colors.white38 : const Color(0xFF94a3b8),
        elevation: 0,
        type: BottomNavigationBarType.fixed,
        selectedLabelStyle: const TextStyle(
          fontFamily: 'Nunito',
          fontSize: 11,
          fontWeight: FontWeight.w700,
        ),
        unselectedLabelStyle: const TextStyle(
          fontFamily: 'Nunito',
          fontSize: 11,
        ),
      ),
      navigationBarTheme: NavigationBarThemeData(
        backgroundColor: isDark ? kDarkCard : Colors.white,
        indicatorColor: kPrimary.withOpacity(0.12),
        iconTheme: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.selected)) {
            return const IconThemeData(color: kPrimary, size: 22);
          }
          return IconThemeData(
            color: isDark ? Colors.white38 : const Color(0xFF94a3b8),
            size: 22,
          );
        }),
        labelTextStyle: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.selected)) {
            return const TextStyle(
              fontFamily: 'Nunito',
              fontSize: 11,
              fontWeight: FontWeight.w700,
              color: kPrimary,
            );
          }
          return TextStyle(
            fontFamily: 'Nunito',
            fontSize: 11,
            color: isDark ? Colors.white38 : const Color(0xFF94a3b8),
          );
        }),
        elevation: 0,
        shadowColor: Colors.transparent,
        surfaceTintColor: Colors.transparent,
      ),
      dividerTheme: DividerThemeData(
        color: isDark ? Colors.white12 : const Color(0xFF1e3a8a).withOpacity(0.08),
        thickness: 1,
        space: 1,
      ),
      snackBarTheme: SnackBarThemeData(
        backgroundColor: isDark ? kDarkCard : const Color(0xFF1e293b),
        contentTextStyle: const TextStyle(
          fontFamily: 'Nunito',
          color: Colors.white,
          fontSize: 14,
        ),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
        behavior: SnackBarBehavior.floating,
      ),
    );
  }
}
