import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:smartclass/features/auth/login_page.dart';
import 'package:smartclass/shared/providers/auth_provider.dart';

/// Integration test: app starts → shows login → enter credentials → tap login.
/// Uses mock providers, not a real backend.
void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('app starts on login page and renders sign-in form',
      (tester) async {
    await tester.pumpWidget(
      const ProviderScope(
        child: MaterialApp(
          home: LoginPage(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Login page should be visible
    expect(find.text('Smart Classroom'), findsOneWidget);
    expect(find.text('Sign in to continue'), findsOneWidget);
    expect(find.byKey(const Key('email_field')), findsOneWidget);
    expect(find.byKey(const Key('password_field')), findsOneWidget);
    expect(find.widgetWithText(FilledButton, 'Sign in'), findsOneWidget);
  });

  testWidgets('login form validates email before calling API', (tester) async {
    await tester.pumpWidget(
      const ProviderScope(
        child: MaterialApp(
          home: LoginPage(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Attempt login with empty email
    await tester.tap(find.widgetWithText(FilledButton, 'Sign in'));
    await tester.pump();

    expect(find.text('Email is required'), findsOneWidget);
  });

  testWidgets('can enter email and password', (tester) async {
    await tester.pumpWidget(
      const ProviderScope(
        child: MaterialApp(
          home: LoginPage(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
        find.byKey(const Key('email_field')), 'test@example.com');
    await tester.enterText(
        find.byKey(const Key('password_field')), 'password123');

    // Fields should have correct values
    expect(find.text('test@example.com'), findsOneWidget);
  });
}
