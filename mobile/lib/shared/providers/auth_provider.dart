import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/client.dart';
import '../../core/api/endpoints/auth_endpoints.dart';
import '../../core/api/endpoints/user_endpoints.dart';
import '../../core/storage/token_storage.dart';
import '../../core/utils/error_utils.dart';
import '../models/user.dart';

final tokenStorageProvider = Provider<TokenStorage>((ref) => TokenStorage());

final apiClientProvider = Provider<ApiClient>((ref) {
  // B-004: use ref.watch so provider rebuilds if tokenStorage changes
  final client = ApiClient(tokenStorage: ref.watch(tokenStorageProvider));
  return client;
});

final authEndpointsProvider = Provider<AuthEndpoints>(
  (ref) => AuthEndpoints(ref.watch(apiClientProvider)),
);

final userEndpointsProvider = Provider<UserEndpoints>(
  (ref) => UserEndpoints(ref.watch(apiClientProvider)),
);

// Auth state
class AuthState {
  final User? user;
  final bool loading;
  final String? error;
  // C-022: true while init() is awaiting getMe() so login/register pages can
  // show a spinner instead of a usable form.
  final bool isInitializing;

  const AuthState({
    this.user,
    this.loading = false,
    this.error,
    this.isInitializing = false,
  });

  bool get isAuthenticated => user != null;

  AuthState copyWith({
    User? user,
    bool? loading,
    String? error,
    bool? isInitializing,
  }) =>
      AuthState(
        user: user ?? this.user,
        loading: loading ?? this.loading,
        error: error,
        isInitializing: isInitializing ?? this.isInitializing,
      );
}

class AuthNotifier extends StateNotifier<AuthState> {
  final AuthEndpoints _auth;
  final UserEndpoints _users;
  final TokenStorage _storage;

  AuthNotifier(this._auth, this._users, this._storage)
      : super(const AuthState());

  Future<void> init() async {
    final token = await _storage.getAccessToken();
    if (token == null) return;
    final expired = await _storage.isRefreshExpired();
    if (expired) {
      await _storage.clear();
      return;
    }
    // C-022: signal initializing so UI shows a spinner.
    state = state.copyWith(isInitializing: true);
    try {
      final user = await _users.getMe();
      state = state.copyWith(user: user, isInitializing: false);
    } catch (_) {
      await _storage.clear();
      state = state.copyWith(isInitializing: false);
    }
  }

  Future<bool> login(String email, String password) async {
    state = state.copyWith(loading: true, error: null);
    try {
      final response = await _auth.login(email, password);
      await _storage.saveTokens(
        accessToken: response.tokens.accessToken,
        refreshToken: response.tokens.refreshToken,
        accessExpiresAt: response.tokens.accessExpiresAt,
        refreshExpiresAt: response.tokens.refreshExpiresAt,
      );
      state = AuthState(user: response.user);
      return true;
    } catch (e) {
      state = state.copyWith(loading: false, error: friendlyError(e));
      return false;
    }
  }

  Future<bool> register({
    required String email,
    required String password,
    required String fullName,
    required String role,
    String? language,
  }) async {
    state = state.copyWith(loading: true, error: null);
    try {
      final response = await _auth.register(
        email: email,
        password: password,
        fullName: fullName,
        role: role,
        language: language,
      );
      await _storage.saveTokens(
        accessToken: response.tokens.accessToken,
        refreshToken: response.tokens.refreshToken,
        accessExpiresAt: response.tokens.accessExpiresAt,
        refreshExpiresAt: response.tokens.refreshExpiresAt,
      );
      state = AuthState(user: response.user);
      return true;
    } catch (e) {
      state = state.copyWith(loading: false, error: friendlyError(e));
      return false;
    }
  }

  // C-008: call backend logout before clearing local storage.
  // If the backend call fails (offline), still clear locally — user must
  // be able to log in again even without network.
  Future<void> logout() async {
    try {
      await _auth.logout();
    } catch (_) {
      // Backend unreachable or already invalidated — proceed with local clear.
    }
    await _storage.clear();
    state = const AuthState();
  }

  Future<void> refreshUser() async {
    try {
      final user = await _users.getMe();
      state = state.copyWith(user: user);
    } catch (_) {}
  }

  void updateUser(User user) {
    state = state.copyWith(user: user);
  }

  // Called by ApiClient when refresh token is expired/invalid
  // B-208: clear token storage in addition to resetting state
  Future<void> forceLogout() async {
    await _storage.clear();
    state = const AuthState();
  }
}

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>((ref) {
  final notifier = AuthNotifier(
    ref.read(authEndpointsProvider),
    ref.read(userEndpointsProvider),
    ref.read(tokenStorageProvider),
  );
  // Wire logout so expired tokens redirect to login
  ref.read(apiClientProvider).setLogoutCallback(() async {
    await notifier.forceLogout();
  });
  return notifier;
});
