// ignore: unused_import
import 'package:intl/intl.dart' as intl;
import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for Russian (`ru`).
class AppLocalizationsRu extends AppLocalizations {
  AppLocalizationsRu([String locale = 'ru']) : super(locale);

  @override
  String get commonLoading => 'Загрузка...';

  @override
  String get commonSave => 'Сохранить';

  @override
  String get commonCancel => 'Отмена';

  @override
  String get commonDelete => 'Удалить';

  @override
  String get commonEdit => 'Редактировать';

  @override
  String get commonCreate => 'Создать';

  @override
  String get commonClose => 'Закрыть';

  @override
  String get commonConfirm => 'Подтвердить';

  @override
  String get commonSearch => 'Поиск';

  @override
  String get commonEmpty => 'Нет элементов';

  @override
  String get commonRetry => 'Повторить';

  @override
  String get commonOnline => 'Онлайн';

  @override
  String get commonOffline => 'Оффлайн';

  @override
  String get navHome => 'Главная';

  @override
  String get navDevices => 'Устройства';

  @override
  String get navSchedule => 'Расписание';

  @override
  String get navScenes => 'Сцены';

  @override
  String get navProfile => 'Профиль';

  @override
  String get authLogin => 'Войти';

  @override
  String get authRegister => 'Регистрация';

  @override
  String get authEmail => 'Email';

  @override
  String get authPassword => 'Пароль';

  @override
  String get authConfirmPassword => 'Подтвердите пароль';

  @override
  String get authFullName => 'Полное имя';

  @override
  String get authRole => 'Роль';

  @override
  String get authRoleTeacher => 'Учитель';

  @override
  String get authRoleAdmin => 'Администратор';

  @override
  String get authRoleTechnician => 'Техник';

  @override
  String get authHaveAccount => 'Уже есть аккаунт?';

  @override
  String get authNoAccount => 'Нет аккаунта?';

  @override
  String get authLogout => 'Выйти';

  @override
  String get authPasswordMismatch => 'Пароли не совпадают';

  @override
  String get homeTitle => 'Умный класс';

  @override
  String get homeNoClassroom => 'Создайте первый класс';

  @override
  String get homeCreateClassroom => 'Новый класс';

  @override
  String get homeClassroomName => 'Название класса';

  @override
  String get homeActiveDevices => 'Активные устройства';

  @override
  String get homeCurrentLesson => 'Текущий урок';

  @override
  String get homeNoLesson => 'Урок не идёт';

  @override
  String get homeTemperature => 'Температура';

  @override
  String get homeHumidity => 'Влажность';

  @override
  String get devicesTitle => 'Устройства';

  @override
  String get devicesAdd => 'Добавить устройство';

  @override
  String get devicesFindIot => 'Найти IoT';

  @override
  String get devicesName => 'Название';

  @override
  String get devicesType => 'Тип';

  @override
  String get devicesBrand => 'Бренд';

  @override
  String get devicesDriver => 'Драйвер';

  @override
  String get devicesConfig => 'Конфигурация (JSON)';

  @override
  String get devicesOn => 'Включить';

  @override
  String get devicesOff => 'Выключить';

  @override
  String get devicesEmpty => 'Нет устройств';

  @override
  String get devicesStatus => 'Статус';

  @override
  String get devicesBrightness => 'Яркость';

  @override
  String get devicesTemperature => 'Температура';

  @override
  String get devicesLevel => 'Уровень';

  @override
  String get devicesLevelLow => 'Низкий';

  @override
  String get devicesLevelMid => 'Средний';

  @override
  String get devicesLevelHigh => 'Высокий';

  @override
  String get devicesAllOn => 'Все вкл';

  @override
  String get devicesAllOff => 'Все выкл';

  @override
  String get devicesEco => 'Эко';

  @override
  String get devicesQuickControls => 'Быстрое управление';

  @override
  String get devicesSending => 'Отправка...';

  @override
  String get hassTitle => 'Найти IoT';

  @override
  String get hassNotReady => 'Home Assistant ещё запускается. Подождите...';

  @override
  String get hassAlreadySetup =>
      'HA настроен вручную. Вставьте долгосрочный токен доступа:';

  @override
  String get hassSaveToken => 'Сохранить токен';

  @override
  String get hassPickBrand => 'Выберите производителя';

  @override
  String get hassAllBrands => 'Показать все интеграции';

  @override
  String get hassBrandNotAvailable =>
      'Интеграция этого бренда ещё не загружена.';

  @override
  String get hassPickIntegration => 'Выберите интеграцию';

  @override
  String get hassSearchIntegration => 'Поиск...';

  @override
  String get hassDiscoveredEntities => 'Обнаруженные устройства';

  @override
  String get hassNoEntities => 'Ничего нет. Сначала подключите интеграцию.';

  @override
  String get hassAddToClassroom => 'Добавить в класс';

  @override
  String get hassNext => 'Далее';

  @override
  String get hassAbort => 'Отмена';

  @override
  String get hassCreated => 'Готово!';

  @override
  String get hassLoadingEntities => 'Загрузка устройств...';

  @override
  String get hassCloudHint =>
      'Перед началом: установите приложение производителя, создайте аккаунт и добавьте устройство.';

  @override
  String get hassLanHint =>
      'Перед началом: убедитесь, что устройство в той же сети Wi-Fi, что и сервер.';

  @override
  String hassVerifyOk(String state) {
    return 'Устройство ответило. Текущее состояние: $state';
  }

  @override
  String get hassVerifyOffline => 'Устройство добавлено, но сейчас оффлайн.';

  @override
  String get hassOauthHint =>
      'Откройте ссылку в новой вкладке, войдите и вернитесь.';

  @override
  String get hassOauthOpen => 'Открыть страницу входа';

  @override
  String get hassOauthDone => 'Я авторизован';

  @override
  String get scheduleTitle => 'Расписание';

  @override
  String get scheduleAddLesson => 'Добавить урок';

  @override
  String get scheduleSubject => 'Предмет';

  @override
  String get scheduleDay => 'День';

  @override
  String get scheduleStartsAt => 'Начало';

  @override
  String get scheduleEndsAt => 'Конец';

  @override
  String get scheduleNotes => 'Заметки';

  @override
  String get scheduleDayMon => 'Пн';

  @override
  String get scheduleDayTue => 'Вт';

  @override
  String get scheduleDayWed => 'Ср';

  @override
  String get scheduleDayThu => 'Чт';

  @override
  String get scheduleDayFri => 'Пт';

  @override
  String get scenesTitle => 'Сцены';

  @override
  String get scenesAdd => 'Новая сцена';

  @override
  String get scenesRun => 'Запустить';

  @override
  String get scenesName => 'Название сцены';

  @override
  String get scenesDescription => 'Описание';

  @override
  String get scenesAddStep => 'Добавить шаг';

  @override
  String get scenesEmpty => 'Нет сцен';

  @override
  String get analyticsTitle => 'Аналитика';

  @override
  String get analyticsSensorsSeries => 'Серия сенсоров';

  @override
  String get analyticsDeviceUsage => 'Использование устройств';

  @override
  String get analyticsEnergy => 'Энергия';

  @override
  String get analyticsMetric => 'Метрика';

  @override
  String get analyticsBucket => 'Интервал';

  @override
  String get analyticsLastWeek => 'Последние 7 дней';

  @override
  String get notificationsTitle => 'Уведомления';

  @override
  String get notificationsMarkAllRead => 'Отметить всё прочитанным';

  @override
  String get notificationsEmpty => 'Нет уведомлений';

  @override
  String get profileTitle => 'Профиль';

  @override
  String get profileLanguage => 'Язык';

  @override
  String get profilePhone => 'Телефон';

  @override
  String get profileChangePassword => 'Изменить пароль';

  @override
  String get profileCurrentPassword => 'Текущий пароль';

  @override
  String get profileNewPassword => 'Новый пароль';

  @override
  String get profileLocalUrl => 'Локальный URL сервера';

  @override
  String get profileLocalUrlHint => 'например http://192.168.1.100:8080';
}
