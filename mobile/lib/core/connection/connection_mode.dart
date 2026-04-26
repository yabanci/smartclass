enum ConnectionMode { local, remote }

class ConnectionState {
  final ConnectionMode mode;
  final String baseUrl;

  const ConnectionState({required this.mode, required this.baseUrl});

  bool get isLocal => mode == ConnectionMode.local;
}
