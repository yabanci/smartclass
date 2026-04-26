import '../client.dart';
import '../../../shared/models/classroom.dart';

class ClassroomEndpoints {
  final ApiClient _client;
  ClassroomEndpoints(this._client);

  Future<List<Classroom>> list() => _client.unwrap(
        _client.get('/classrooms'),
        (d) => (d as List).map((e) => Classroom.fromJson(e as Map<String, dynamic>)).toList(),
      );

  Future<Classroom> create({required String name, String? description}) =>
      _client.unwrap(
        _client.post('/classrooms', data: {
          'name': name,
          if (description != null) 'description': description,
        }),
        (d) => Classroom.fromJson(d as Map<String, dynamic>),
      );

  Future<Classroom> get(String id) => _client.unwrap(
        _client.get('/classrooms/$id'),
        (d) => Classroom.fromJson(d as Map<String, dynamic>),
      );

  Future<Classroom> update(String id, {String? name, String? description}) =>
      _client.unwrap(
        _client.patch('/classrooms/$id', data: {
          if (name != null) 'name': name,
          if (description != null) 'description': description,
        }),
        (d) => Classroom.fromJson(d as Map<String, dynamic>),
      );

  Future<void> delete(String id) => _client.unwrap(
        _client.delete('/classrooms/$id'),
        (_) {},
      );
}
