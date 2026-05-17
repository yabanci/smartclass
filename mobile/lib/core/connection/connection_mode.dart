// C-016: added `unreachable` for when both local and remote are unavailable.
enum ConnectionMode { local, remote, unreachable }

class ConnectionState {
  final ConnectionMode mode;
  final String baseUrl;

  const ConnectionState({required this.mode, required this.baseUrl});

  bool get isLocal => mode == ConnectionMode.local;
  bool get isUnreachable => mode == ConnectionMode.unreachable;
}
