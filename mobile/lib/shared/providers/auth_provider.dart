import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/client.dart';
import '../../core/api/endpoints/auth_endpoints.dart';
import '../../core/api/endpoints/user_endpoints.dart';
import '../../core/storage/token_storage.dart';
import '../models/user.dart';

final tokenStorageProvider = Provider<TokenStorage>((ref) => TokenStorage());

final apiClientProvider = Provider<ApiClient>((ref) {
  final client = ApiClient(tokenStorage: ref.read(tokenStorageProvider));
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

  const AuthState({this.user, this.loading = false, this.error});

  bool get isAuthenticated => user != null;

  AuthState copyWith({User? user, bool? loading, String? error}) => AuthState(
        user: user ?? this.user,
        loading: loading ?? this.loading,
        error: error,
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
    try {
      final user = await _users.getMe();
      state = state.copyWith(user: user);
    } catch (_) {
      await _storage.clear();
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
      state = state.copyWith(loading: false, error: e.toString());
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
      state = state.copyWith(loading: false, error: e.toString());
      return false;
    }
  }

  Future<void> logout() async {
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
  void forceLogout() {
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
    notifier.forceLogout();
  });
  return notifier;
});
