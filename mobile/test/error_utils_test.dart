import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:smartclass/core/utils/error_utils.dart';

void main() {
  group('friendlyError', () {
    test('strips ApiException prefix', () {
      final result = friendlyError(Exception('ApiException(200): Device offline'));
      expect(result, isNot(contains('ApiException')));
    });

    test('returns server error with code for DioException', () {
      final dio = Dio();
      final response = Response<dynamic>(
        requestOptions: RequestOptions(path: '/test'),
        statusCode: 502,
        data: null,
      );
      final e = DioException(
        requestOptions: RequestOptions(path: '/test'),
        response: response,
        type: DioExceptionType.badResponse,
      );
      final msg = friendlyError(e);
      expect(msg, contains('502'));
    });

    test('extracts message from backend error body', () {
      final response = Response<dynamic>(
        requestOptions: RequestOptions(path: '/test'),
        statusCode: 400,
        data: {'error': {'code': 'invalid', 'message': 'Name is required'}},
      );
      final e = DioException(
        requestOptions: RequestOptions(path: '/test'),
        response: response,
        type: DioExceptionType.badResponse,
      );
      final msg = friendlyError(e);
      expect(msg, 'Name is required');
    });

    test('returns network error for connectionError type', () {
      final e = DioException(
        requestOptions: RequestOptions(path: '/test'),
        type: DioExceptionType.connectionError,
      );
      expect(friendlyError(e), contains('connect'));
    });

    test('returns plain message for generic exception', () {
      expect(friendlyError(Exception('oops')), contains('oops'));
    });
  });
}
