// ignore: unused_import
import 'package:intl/intl.dart' as intl;
import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for English (`en`).
class AppLocalizationsEn extends AppLocalizations {
  AppLocalizationsEn([String locale = 'en']) : super(locale);

  @override
  String get commonLoading => 'Loading...';

  @override
  String get commonSave => 'Save';

  @override
  String get commonCancel => 'Cancel';

  @override
  String get commonDelete => 'Delete';

  @override
  String get commonEdit => 'Edit';

  @override
  String get commonCreate => 'Create';

  @override
  String get commonClose => 'Close';

  @override
  String get commonConfirm => 'Confirm';

  @override
  String get commonSearch => 'Search';

  @override
  String get commonEmpty => 'No items';

  @override
  String get commonRetry => 'Retry';

  @override
  String get commonOnline => 'Online';

  @override
  String get commonOffline => 'Offline';

  @override
  String get navHome => 'Home';

  @override
  String get navDevices => 'Devices';

  @override
  String get navSchedule => 'Schedule';

  @override
  String get navScenes => 'Scenes';

  @override
  String get navProfile => 'Profile';

  @override
  String get authLogin => 'Sign in';

  @override
  String get authRegister => 'Register';

  @override
  String get authEmail => 'Email';

  @override
  String get authPassword => 'Password';

  @override
  String get authConfirmPassword => 'Confirm password';

  @override
  String get authFullName => 'Full name';

  @override
  String get authRole => 'Role';

  @override
  String get authRoleTeacher => 'Teacher';

  @override
  String get authRoleAdmin => 'Admin';

  @override
  String get authRoleTechnician => 'Technician';

  @override
  String get authHaveAccount => 'Already have an account?';

  @override
  String get authNoAccount => 'Don\'t have an account?';

  @override
  String get authLogout => 'Log out';

  @override
  String get authPasswordMismatch => 'Passwords do not match';

  @override
  String get homeTitle => 'Smart Classroom';

  @override
  String get homeNoClassroom => 'Create your first classroom';

  @override
  String get homeCreateClassroom => 'New classroom';

  @override
  String get homeClassroomName => 'Classroom name';

  @override
  String get homeActiveDevices => 'Active devices';

  @override
  String get homeCurrentLesson => 'Current lesson';

  @override
  String get homeNoLesson => 'No lesson in progress';

  @override
  String get homeTemperature => 'Temperature';

  @override
  String get homeHumidity => 'Humidity';

  @override
  String get devicesTitle => 'Devices';

  @override
  String get devicesAdd => 'Add device';

  @override
  String get devicesFindIot => 'Find IoT';

  @override
  String get devicesName => 'Name';

  @override
  String get devicesType => 'Type';

  @override
  String get devicesBrand => 'Brand';

  @override
  String get devicesDriver => 'Driver';

  @override
  String get devicesConfig => 'Config (JSON)';

  @override
  String get devicesOn => 'Turn on';

  @override
  String get devicesOff => 'Turn off';

  @override
  String get devicesEmpty => 'No devices yet';

  @override
  String get devicesStatus => 'Status';

  @override
  String get devicesBrightness => 'Brightness';

  @override
  String get devicesTemperature => 'Temperature';

  @override
  String get devicesLevel => 'Level';

  @override
  String get devicesLevelLow => 'Low';

  @override
  String get devicesLevelMid => 'Medium';

  @override
  String get devicesLevelHigh => 'High';

  @override
  String get devicesAllOn => 'All on';

  @override
  String get devicesAllOff => 'All off';

  @override
  String get devicesEco => 'Eco';

  @override
  String get devicesQuickControls => 'Quick controls';

  @override
  String get devicesSending => 'Sending...';

  @override
  String get hassTitle => 'Find IoT';

  @override
  String get hassNotReady => 'Home Assistant is still starting. Please wait...';

  @override
  String get hassAlreadySetup =>
      'HA was set up manually. Paste a long-lived access token:';

  @override
  String get hassSaveToken => 'Save token';

  @override
  String get hassPickBrand => 'Pick a manufacturer';

  @override
  String get hassAllBrands => 'Show all integrations';

  @override
  String get hassBrandNotAvailable =>
      'This brand\'s integration isn\'t loaded yet.';

  @override
  String get hassPickIntegration => 'Pick an integration';

  @override
  String get hassSearchIntegration => 'Search...';

  @override
  String get hassDiscoveredEntities => 'Discovered devices';

  @override
  String get hassNoEntities => 'Nothing yet. Pair an integration first.';

  @override
  String get hassAddToClassroom => 'Add to classroom';

  @override
  String get hassNext => 'Next';

  @override
  String get hassAbort => 'Cancel';

  @override
  String get hassCreated => 'Done!';

  @override
  String get hassLoadingEntities => 'Loading devices...';

  @override
  String get hassCloudHint =>
      'Before you start: install the manufacturer\'s app, create an account, and add the device there.';

  @override
  String get hassLanHint =>
      'Before you start: make sure the device is on the same Wi-Fi as the server.';

  @override
  String hassVerifyOk(String state) {
    return 'Device responded. Current state: $state';
  }

  @override
  String get hassVerifyOffline => 'Device added, but it\'s offline right now.';

  @override
  String get hassOauthHint =>
      'Open the link in a new tab, sign in, and come back.';

  @override
  String get hassOauthOpen => 'Open sign-in page';

  @override
  String get hassOauthDone => 'I\'m authorized';

  @override
  String get scheduleTitle => 'Schedule';

  @override
  String get scheduleAddLesson => 'Add lesson';

  @override
  String get scheduleSubject => 'Subject';

  @override
  String get scheduleDay => 'Day';

  @override
  String get scheduleStartsAt => 'Starts';

  @override
  String get scheduleEndsAt => 'Ends';

  @override
  String get scheduleNotes => 'Notes';

  @override
  String get scheduleDayMon => 'Mon';

  @override
  String get scheduleDayTue => 'Tue';

  @override
  String get scheduleDayWed => 'Wed';

  @override
  String get scheduleDayThu => 'Thu';

  @override
  String get scheduleDayFri => 'Fri';

  @override
  String get scheduleDaySat => 'Sat';

  @override
  String get scheduleDaySun => 'Sun';

  @override
  String get scenesTitle => 'Scenes';

  @override
  String get scenesAdd => 'New scene';

  @override
  String get scenesRun => 'Run';

  @override
  String get scenesName => 'Scene name';

  @override
  String get scenesDescription => 'Description';

  @override
  String get scenesAddStep => 'Add step';

  @override
  String get scenesEmpty => 'No scenes yet';

  @override
  String scenesRunSuccess(int total) {
    String _temp0 = intl.Intl.pluralLogic(
      total,
      locale: localeName,
      other: '$total steps succeeded',
      one: '1 step succeeded',
    );
    return '$_temp0';
  }

  @override
  String scenesRunPartial(int success, int total) {
    String _temp0 = intl.Intl.pluralLogic(
      total,
      locale: localeName,
      other: '$total steps',
      one: '1 step',
    );
    return '$success/$_temp0 OK';
  }

  @override
  String get analyticsTitle => 'Analytics';

  @override
  String get analyticsSensorsSeries => 'Sensor series';

  @override
  String get analyticsDeviceUsage => 'Device usage';

  @override
  String get analyticsEnergy => 'Energy';

  @override
  String get analyticsMetric => 'Metric';

  @override
  String get analyticsBucket => 'Bucket';

  @override
  String get analyticsLastWeek => 'Last 7 days';

  @override
  String get notificationsTitle => 'Notifications';

  @override
  String get notificationsMarkAllRead => 'Mark all read';

  @override
  String get notificationsEmpty => 'No notifications';

  @override
  String get profileTitle => 'Profile';

  @override
  String get profileLanguage => 'Language';

  @override
  String get profilePhone => 'Phone';

  @override
  String get profileChangePassword => 'Change password';

  @override
  String get profileCurrentPassword => 'Current password';

  @override
  String get profileNewPassword => 'New password';

  @override
  String get profileLocalUrl => 'Local server URL';

  @override
  String get profileLocalUrlHint => 'e.g. http://192.168.1.100:8080';

  @override
  String get offlineNoInternet => 'No internet connection';

  @override
  String get offlineUnreachable => 'Server unreachable';

  @override
  String get commonCannotUndo => 'This action cannot be undone.';

  @override
  String scenesStepCount(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count steps',
      one: '1 step',
    );
    return '$_temp0';
  }

  @override
  String get analyticsNoData => 'No data';

  @override
  String get analyticsNoUsageData => 'No usage data';

  @override
  String get scenesAddAtLeastOneStep => 'Add at least one step';

  @override
  String get scenesNameRequired => 'Scene name is required';

  @override
  String get commonRequired => 'Required';

  @override
  String get profilePasswordChanged => 'Password changed successfully';

  @override
  String get profilePasswordMin6 => 'Min 6 characters';

  @override
  String get commonNotSet => 'Not set';

  @override
  String get scenesStepsLabel => 'Steps';

  @override
  String iotWizardEntityState(String state) {
    return 'State: $state';
  }

  @override
  String get cachedDataLabel => 'Offline data';
}
