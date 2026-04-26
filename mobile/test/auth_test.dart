import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:smartclass/features/auth/login_page.dart';
import 'package:smartclass/features/auth/register_page.dart';

import 'test_helpers.dart';

void main() {
  group('LoginPage form validation', () {
    testWidgets('shows error when email is empty', (tester) async {
      await tester.pumpWidget(testApp(const LoginPage()));
      await tester.pumpAndSettle();

      // Tap sign-in without filling anything
      await tester.tap(find.byType(FilledButton).first);
      await tester.pump();

      expect(find.textContaining('required'), findsAtLeastNWidgets(1));
    });

    testWidgets('shows error when email format is invalid', (tester) async {
      await tester.pumpWidget(testApp(const LoginPage()));
      await tester.pumpAndSettle();

      await tester.enterText(find.byKey(const Key('email_field')), 'notanemail');
      await tester.enterText(find.byKey(const Key('password_field')), 'pass123');
      await tester.tap(find.byType(FilledButton).first);
      await tester.pump();

      expect(find.text('Enter a valid email'), findsOneWidget);
    });

    testWidgets('shows password required when only email filled', (tester) async {
      await tester.pumpWidget(testApp(const LoginPage()));
      await tester.pumpAndSettle();

      await tester.enterText(
          find.byKey(const Key('email_field')), 'test@example.com');
      await tester.tap(find.byType(FilledButton).first);
      await tester.pump();

      expect(find.textContaining('required'), findsAtLeastNWidgets(1));
    });

    testWidgets('has email_field and password_field keys', (tester) async {
      await tester.pumpWidget(testApp(const LoginPage()));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('email_field')), findsOneWidget);
      expect(find.byKey(const Key('password_field')), findsOneWidget);
    });
  });

  group('RegisterPage form validation', () {
    testWidgets('shows password mismatch error', (tester) async {
      await tester.pumpWidget(testApp(const RegisterPage()));
      await tester.pumpAndSettle();

      // Fill in name
      await tester.enterText(find.byType(TextFormField).at(0), 'John Doe');
      // Fill in email
      await tester.enterText(find.byType(TextFormField).at(1), 'john@test.com');
      // Fill in password
      await tester.enterText(find.byType(TextFormField).at(2), 'password123');
      // Fill in confirm (different)
      await tester.enterText(
          find.byKey(const Key('confirm_password_field')), 'different123');

      await tester.tap(find.byType(FilledButton).first);
      await tester.pump();

      expect(find.textContaining('do not match'), findsOneWidget);
    });

    testWidgets('has confirm_password_field key', (tester) async {
      await tester.pumpWidget(testApp(const RegisterPage()));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('confirm_password_field')), findsOneWidget);
    });

    testWidgets('shows role dropdown', (tester) async {
      await tester.pumpWidget(testApp(const RegisterPage()));
      await tester.pumpAndSettle();
      expect(find.byType(DropdownButtonFormField<String>), findsOneWidget);
    });
  });
}
