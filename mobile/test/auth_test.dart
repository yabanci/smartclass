import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:smartclass/core/api/client.dart';
import 'package:smartclass/core/api/endpoints/auth_endpoints.dart';
import 'package:smartclass/core/api/endpoints/user_endpoints.dart';
import 'package:smartclass/core/storage/token_storage.dart';
import 'package:smartclass/features/auth/login_page.dart';
import 'package:smartclass/features/auth/register_page.dart';
import 'package:smartclass/shared/providers/auth_provider.dart';

// Minimal stubs so AuthNotifier constructor doesn't crash during widget tests
class _StubApiClient extends ApiClient {
  _StubApiClient() : super();
}

class _StubAuthEndpoints extends AuthEndpoints {
  _StubAuthEndpoints() : super(_StubApiClient());
}

class _StubUserEndpoints extends UserEndpoints {
  _StubUserEndpoints() : super(_StubApiClient());
}

class _StubTokenStorage extends TokenStorage {
  _StubTokenStorage();

  @override
  Future<String?> getAccessToken() async => null;

  @override
  Future<bool> isRefreshExpired() async => true;

  @override
  Future<void> clear() async {}
}

Widget _wrapWithProviders(Widget child) {
  return ProviderScope(
    child: MaterialApp(home: child),
  );
}

void main() {
  group('LoginPage form validation', () {
    testWidgets('shows error when email is empty', (tester) async {
      await tester.pumpWidget(_wrapWithProviders(const LoginPage()));
      await tester.pumpAndSettle();

      final signInButton = find.widgetWithText(FilledButton, 'Sign in');
      expect(signInButton, findsOneWidget);
      await tester.tap(signInButton);
      await tester.pump();

      expect(find.text('Email is required'), findsOneWidget);
    });

    testWidgets('shows error when email format is invalid', (tester) async {
      await tester.pumpWidget(_wrapWithProviders(const LoginPage()));
      await tester.pumpAndSettle();

      await tester.enterText(
          find.byKey(const Key('email_field')), 'notanemail');
      await tester.enterText(
          find.byKey(const Key('password_field')), 'password123');

      final signInButton = find.widgetWithText(FilledButton, 'Sign in');
      await tester.tap(signInButton);
      await tester.pump();

      expect(find.text('Enter a valid email'), findsOneWidget);
    });

    testWidgets('shows password required when password is empty', (tester) async {
      await tester.pumpWidget(_wrapWithProviders(const LoginPage()));
      await tester.pumpAndSettle();

      await tester.enterText(
          find.byKey(const Key('email_field')), 'test@example.com');

      final signInButton = find.widgetWithText(FilledButton, 'Sign in');
      await tester.tap(signInButton);
      await tester.pump();

      expect(find.text('Password is required'), findsOneWidget);
    });
  });

  group('RegisterPage form validation', () {
    testWidgets('shows password mismatch error', (tester) async {
      await tester.pumpWidget(_wrapWithProviders(const RegisterPage()));
      await tester.pumpAndSettle();

      // Fill password and confirm with different values
      final passwordFields = find.byType(TextFormField);
      // Fields: name, email, role dropdown, password, confirm
      // Use key for confirm password
      final confirmField = find.byKey(const Key('confirm_password_field'));
      expect(confirmField, findsOneWidget);

      // Enter mismatched passwords via the confirm field directly
      // We also need at least something in password field
      // Find all TextFormFields and fill them minimally
      await tester.enterText(passwordFields.at(0), 'John Doe'); // name
      await tester.enterText(passwordFields.at(1), 'test@test.com'); // email
      await tester.enterText(passwordFields.at(2), 'password123'); // password
      await tester.enterText(confirmField, 'different123'); // confirm

      final registerButton = find.widgetWithText(FilledButton, 'Register');
      await tester.tap(registerButton);
      await tester.pump();

      expect(find.text('Passwords do not match'), findsOneWidget);
    });
  });
}
