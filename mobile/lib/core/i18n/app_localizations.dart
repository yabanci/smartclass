import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:intl/intl.dart' as intl;

import 'app_localizations_en.dart';
import 'app_localizations_kk.dart';
import 'app_localizations_ru.dart';

// ignore_for_file: type=lint

/// Callers can lookup localized strings with an instance of AppLocalizations
/// returned by `AppLocalizations.of(context)`.
///
/// Applications need to include `AppLocalizations.delegate()` in their app's
/// `localizationDelegates` list, and the locales they support in the app's
/// `supportedLocales` list. For example:
///
/// ```dart
/// import 'i18n/app_localizations.dart';
///
/// return MaterialApp(
///   localizationsDelegates: AppLocalizations.localizationsDelegates,
///   supportedLocales: AppLocalizations.supportedLocales,
///   home: MyApplicationHome(),
/// );
/// ```
///
/// ## Update pubspec.yaml
///
/// Please make sure to update your pubspec.yaml to include the following
/// packages:
///
/// ```yaml
/// dependencies:
///   # Internationalization support.
///   flutter_localizations:
///     sdk: flutter
///   intl: any # Use the pinned version from flutter_localizations
///
///   # Rest of dependencies
/// ```
///
/// ## iOS Applications
///
/// iOS applications define key application metadata, including supported
/// locales, in an Info.plist file that is built into the application bundle.
/// To configure the locales supported by your app, you’ll need to edit this
/// file.
///
/// First, open your project’s ios/Runner.xcworkspace Xcode workspace file.
/// Then, in the Project Navigator, open the Info.plist file under the Runner
/// project’s Runner folder.
///
/// Next, select the Information Property List item, select Add Item from the
/// Editor menu, then select Localizations from the pop-up menu.
///
/// Select and expand the newly-created Localizations item then, for each
/// locale your application supports, add a new item and select the locale
/// you wish to add from the pop-up menu in the Value field. This list should
/// be consistent with the languages listed in the AppLocalizations.supportedLocales
/// property.
abstract class AppLocalizations {
  AppLocalizations(String locale)
      : localeName = intl.Intl.canonicalizedLocale(locale.toString());

  final String localeName;

  static AppLocalizations of(BuildContext context) {
    return Localizations.of<AppLocalizations>(context, AppLocalizations)!;
  }

  static const LocalizationsDelegate<AppLocalizations> delegate =
      _AppLocalizationsDelegate();

  /// A list of this localizations delegate along with the default localizations
  /// delegates.
  ///
  /// Returns a list of localizations delegates containing this delegate along with
  /// GlobalMaterialLocalizations.delegate, GlobalCupertinoLocalizations.delegate,
  /// and GlobalWidgetsLocalizations.delegate.
  ///
  /// Additional delegates can be added by appending to this list in
  /// MaterialApp. This list does not have to be used at all if a custom list
  /// of delegates is preferred or required.
  static const List<LocalizationsDelegate<dynamic>> localizationsDelegates =
      <LocalizationsDelegate<dynamic>>[
    delegate,
    GlobalMaterialLocalizations.delegate,
    GlobalCupertinoLocalizations.delegate,
    GlobalWidgetsLocalizations.delegate,
  ];

  /// A list of this localizations delegate's supported locales.
  static const List<Locale> supportedLocales = <Locale>[
    Locale('en'),
    Locale('kk'),
    Locale('ru')
  ];

  /// No description provided for @commonLoading.
  ///
  /// In en, this message translates to:
  /// **'Loading...'**
  String get commonLoading;

  /// No description provided for @commonSave.
  ///
  /// In en, this message translates to:
  /// **'Save'**
  String get commonSave;

  /// No description provided for @commonCancel.
  ///
  /// In en, this message translates to:
  /// **'Cancel'**
  String get commonCancel;

  /// No description provided for @commonDelete.
  ///
  /// In en, this message translates to:
  /// **'Delete'**
  String get commonDelete;

  /// No description provided for @commonEdit.
  ///
  /// In en, this message translates to:
  /// **'Edit'**
  String get commonEdit;

  /// No description provided for @commonCreate.
  ///
  /// In en, this message translates to:
  /// **'Create'**
  String get commonCreate;

  /// No description provided for @commonClose.
  ///
  /// In en, this message translates to:
  /// **'Close'**
  String get commonClose;

  /// No description provided for @commonConfirm.
  ///
  /// In en, this message translates to:
  /// **'Confirm'**
  String get commonConfirm;

  /// No description provided for @commonSearch.
  ///
  /// In en, this message translates to:
  /// **'Search'**
  String get commonSearch;

  /// No description provided for @commonEmpty.
  ///
  /// In en, this message translates to:
  /// **'No items'**
  String get commonEmpty;

  /// No description provided for @commonRetry.
  ///
  /// In en, this message translates to:
  /// **'Retry'**
  String get commonRetry;

  /// No description provided for @commonOnline.
  ///
  /// In en, this message translates to:
  /// **'Online'**
  String get commonOnline;

  /// No description provided for @commonOffline.
  ///
  /// In en, this message translates to:
  /// **'Offline'**
  String get commonOffline;

  /// No description provided for @navHome.
  ///
  /// In en, this message translates to:
  /// **'Home'**
  String get navHome;

  /// No description provided for @navDevices.
  ///
  /// In en, this message translates to:
  /// **'Devices'**
  String get navDevices;

  /// No description provided for @navSchedule.
  ///
  /// In en, this message translates to:
  /// **'Schedule'**
  String get navSchedule;

  /// No description provided for @navScenes.
  ///
  /// In en, this message translates to:
  /// **'Scenes'**
  String get navScenes;

  /// No description provided for @navProfile.
  ///
  /// In en, this message translates to:
  /// **'Profile'**
  String get navProfile;

  /// No description provided for @authLogin.
  ///
  /// In en, this message translates to:
  /// **'Sign in'**
  String get authLogin;

  /// No description provided for @authRegister.
  ///
  /// In en, this message translates to:
  /// **'Register'**
  String get authRegister;

  /// No description provided for @authEmail.
  ///
  /// In en, this message translates to:
  /// **'Email'**
  String get authEmail;

  /// No description provided for @authPassword.
  ///
  /// In en, this message translates to:
  /// **'Password'**
  String get authPassword;

  /// No description provided for @authConfirmPassword.
  ///
  /// In en, this message translates to:
  /// **'Confirm password'**
  String get authConfirmPassword;

  /// No description provided for @authFullName.
  ///
  /// In en, this message translates to:
  /// **'Full name'**
  String get authFullName;

  /// No description provided for @authRole.
  ///
  /// In en, this message translates to:
  /// **'Role'**
  String get authRole;

  /// No description provided for @authRoleTeacher.
  ///
  /// In en, this message translates to:
  /// **'Teacher'**
  String get authRoleTeacher;

  /// No description provided for @authRoleAdmin.
  ///
  /// In en, this message translates to:
  /// **'Admin'**
  String get authRoleAdmin;

  /// No description provided for @authRoleTechnician.
  ///
  /// In en, this message translates to:
  /// **'Technician'**
  String get authRoleTechnician;

  /// No description provided for @authHaveAccount.
  ///
  /// In en, this message translates to:
  /// **'Already have an account?'**
  String get authHaveAccount;

  /// No description provided for @authNoAccount.
  ///
  /// In en, this message translates to:
  /// **'Don\'t have an account?'**
  String get authNoAccount;

  /// No description provided for @authLogout.
  ///
  /// In en, this message translates to:
  /// **'Log out'**
  String get authLogout;

  /// No description provided for @authPasswordMismatch.
  ///
  /// In en, this message translates to:
  /// **'Passwords do not match'**
  String get authPasswordMismatch;

  /// No description provided for @homeTitle.
  ///
  /// In en, this message translates to:
  /// **'Smart Classroom'**
  String get homeTitle;

  /// No description provided for @homeNoClassroom.
  ///
  /// In en, this message translates to:
  /// **'Create your first classroom'**
  String get homeNoClassroom;

  /// No description provided for @homeCreateClassroom.
  ///
  /// In en, this message translates to:
  /// **'New classroom'**
  String get homeCreateClassroom;

  /// No description provided for @homeClassroomName.
  ///
  /// In en, this message translates to:
  /// **'Classroom name'**
  String get homeClassroomName;

  /// No description provided for @homeActiveDevices.
  ///
  /// In en, this message translates to:
  /// **'Active devices'**
  String get homeActiveDevices;

  /// No description provided for @homeCurrentLesson.
  ///
  /// In en, this message translates to:
  /// **'Current lesson'**
  String get homeCurrentLesson;

  /// No description provided for @homeNoLesson.
  ///
  /// In en, this message translates to:
  /// **'No lesson in progress'**
  String get homeNoLesson;

  /// No description provided for @homeTemperature.
  ///
  /// In en, this message translates to:
  /// **'Temperature'**
  String get homeTemperature;

  /// No description provided for @homeHumidity.
  ///
  /// In en, this message translates to:
  /// **'Humidity'**
  String get homeHumidity;

  /// No description provided for @devicesTitle.
  ///
  /// In en, this message translates to:
  /// **'Devices'**
  String get devicesTitle;

  /// No description provided for @devicesAdd.
  ///
  /// In en, this message translates to:
  /// **'Add device'**
  String get devicesAdd;

  /// No description provided for @devicesFindIot.
  ///
  /// In en, this message translates to:
  /// **'Find IoT'**
  String get devicesFindIot;

  /// No description provided for @devicesName.
  ///
  /// In en, this message translates to:
  /// **'Name'**
  String get devicesName;

  /// No description provided for @devicesType.
  ///
  /// In en, this message translates to:
  /// **'Type'**
  String get devicesType;

  /// No description provided for @devicesBrand.
  ///
  /// In en, this message translates to:
  /// **'Brand'**
  String get devicesBrand;

  /// No description provided for @devicesDriver.
  ///
  /// In en, this message translates to:
  /// **'Driver'**
  String get devicesDriver;

  /// No description provided for @devicesConfig.
  ///
  /// In en, this message translates to:
  /// **'Config (JSON)'**
  String get devicesConfig;

  /// No description provided for @devicesOn.
  ///
  /// In en, this message translates to:
  /// **'Turn on'**
  String get devicesOn;

  /// No description provided for @devicesOff.
  ///
  /// In en, this message translates to:
  /// **'Turn off'**
  String get devicesOff;

  /// No description provided for @devicesEmpty.
  ///
  /// In en, this message translates to:
  /// **'No devices yet'**
  String get devicesEmpty;

  /// No description provided for @devicesStatus.
  ///
  /// In en, this message translates to:
  /// **'Status'**
  String get devicesStatus;

  /// No description provided for @devicesBrightness.
  ///
  /// In en, this message translates to:
  /// **'Brightness'**
  String get devicesBrightness;

  /// No description provided for @devicesTemperature.
  ///
  /// In en, this message translates to:
  /// **'Temperature'**
  String get devicesTemperature;

  /// No description provided for @devicesLevel.
  ///
  /// In en, this message translates to:
  /// **'Level'**
  String get devicesLevel;

  /// No description provided for @devicesLevelLow.
  ///
  /// In en, this message translates to:
  /// **'Low'**
  String get devicesLevelLow;

  /// No description provided for @devicesLevelMid.
  ///
  /// In en, this message translates to:
  /// **'Medium'**
  String get devicesLevelMid;

  /// No description provided for @devicesLevelHigh.
  ///
  /// In en, this message translates to:
  /// **'High'**
  String get devicesLevelHigh;

  /// No description provided for @devicesAllOn.
  ///
  /// In en, this message translates to:
  /// **'All on'**
  String get devicesAllOn;

  /// No description provided for @devicesAllOff.
  ///
  /// In en, this message translates to:
  /// **'All off'**
  String get devicesAllOff;

  /// No description provided for @devicesEco.
  ///
  /// In en, this message translates to:
  /// **'Eco'**
  String get devicesEco;

  /// No description provided for @devicesQuickControls.
  ///
  /// In en, this message translates to:
  /// **'Quick controls'**
  String get devicesQuickControls;

  /// No description provided for @devicesSending.
  ///
  /// In en, this message translates to:
  /// **'Sending...'**
  String get devicesSending;

  /// No description provided for @hassTitle.
  ///
  /// In en, this message translates to:
  /// **'Find IoT'**
  String get hassTitle;

  /// No description provided for @hassNotReady.
  ///
  /// In en, this message translates to:
  /// **'Home Assistant is still starting. Please wait...'**
  String get hassNotReady;

  /// No description provided for @hassAlreadySetup.
  ///
  /// In en, this message translates to:
  /// **'HA was set up manually. Paste a long-lived access token:'**
  String get hassAlreadySetup;

  /// No description provided for @hassSaveToken.
  ///
  /// In en, this message translates to:
  /// **'Save token'**
  String get hassSaveToken;

  /// No description provided for @hassPickBrand.
  ///
  /// In en, this message translates to:
  /// **'Pick a manufacturer'**
  String get hassPickBrand;

  /// No description provided for @hassAllBrands.
  ///
  /// In en, this message translates to:
  /// **'Show all integrations'**
  String get hassAllBrands;

  /// No description provided for @hassBrandNotAvailable.
  ///
  /// In en, this message translates to:
  /// **'This brand\'s integration isn\'t loaded yet.'**
  String get hassBrandNotAvailable;

  /// No description provided for @hassPickIntegration.
  ///
  /// In en, this message translates to:
  /// **'Pick an integration'**
  String get hassPickIntegration;

  /// No description provided for @hassSearchIntegration.
  ///
  /// In en, this message translates to:
  /// **'Search...'**
  String get hassSearchIntegration;

  /// No description provided for @hassDiscoveredEntities.
  ///
  /// In en, this message translates to:
  /// **'Discovered devices'**
  String get hassDiscoveredEntities;

  /// No description provided for @hassNoEntities.
  ///
  /// In en, this message translates to:
  /// **'Nothing yet. Pair an integration first.'**
  String get hassNoEntities;

  /// No description provided for @hassAddToClassroom.
  ///
  /// In en, this message translates to:
  /// **'Add to classroom'**
  String get hassAddToClassroom;

  /// No description provided for @hassNext.
  ///
  /// In en, this message translates to:
  /// **'Next'**
  String get hassNext;

  /// No description provided for @hassAbort.
  ///
  /// In en, this message translates to:
  /// **'Cancel'**
  String get hassAbort;

  /// No description provided for @hassCreated.
  ///
  /// In en, this message translates to:
  /// **'Done!'**
  String get hassCreated;

  /// No description provided for @hassLoadingEntities.
  ///
  /// In en, this message translates to:
  /// **'Loading devices...'**
  String get hassLoadingEntities;

  /// No description provided for @hassCloudHint.
  ///
  /// In en, this message translates to:
  /// **'Before you start: install the manufacturer\'s app, create an account, and add the device there.'**
  String get hassCloudHint;

  /// No description provided for @hassLanHint.
  ///
  /// In en, this message translates to:
  /// **'Before you start: make sure the device is on the same Wi-Fi as the server.'**
  String get hassLanHint;

  /// No description provided for @hassVerifyOk.
  ///
  /// In en, this message translates to:
  /// **'Device responded. Current state: {state}'**
  String hassVerifyOk(String state);

  /// No description provided for @hassVerifyOffline.
  ///
  /// In en, this message translates to:
  /// **'Device added, but it\'s offline right now.'**
  String get hassVerifyOffline;

  /// No description provided for @hassOauthHint.
  ///
  /// In en, this message translates to:
  /// **'Open the link in a new tab, sign in, and come back.'**
  String get hassOauthHint;

  /// No description provided for @hassOauthOpen.
  ///
  /// In en, this message translates to:
  /// **'Open sign-in page'**
  String get hassOauthOpen;

  /// No description provided for @hassOauthDone.
  ///
  /// In en, this message translates to:
  /// **'I\'m authorized'**
  String get hassOauthDone;

  /// No description provided for @scheduleTitle.
  ///
  /// In en, this message translates to:
  /// **'Schedule'**
  String get scheduleTitle;

  /// No description provided for @scheduleAddLesson.
  ///
  /// In en, this message translates to:
  /// **'Add lesson'**
  String get scheduleAddLesson;

  /// No description provided for @scheduleSubject.
  ///
  /// In en, this message translates to:
  /// **'Subject'**
  String get scheduleSubject;

  /// No description provided for @scheduleDay.
  ///
  /// In en, this message translates to:
  /// **'Day'**
  String get scheduleDay;

  /// No description provided for @scheduleStartsAt.
  ///
  /// In en, this message translates to:
  /// **'Starts'**
  String get scheduleStartsAt;

  /// No description provided for @scheduleEndsAt.
  ///
  /// In en, this message translates to:
  /// **'Ends'**
  String get scheduleEndsAt;

  /// No description provided for @scheduleNotes.
  ///
  /// In en, this message translates to:
  /// **'Notes'**
  String get scheduleNotes;

  /// No description provided for @scheduleDayMon.
  ///
  /// In en, this message translates to:
  /// **'Mon'**
  String get scheduleDayMon;

  /// No description provided for @scheduleDayTue.
  ///
  /// In en, this message translates to:
  /// **'Tue'**
  String get scheduleDayTue;

  /// No description provided for @scheduleDayWed.
  ///
  /// In en, this message translates to:
  /// **'Wed'**
  String get scheduleDayWed;

  /// No description provided for @scheduleDayThu.
  ///
  /// In en, this message translates to:
  /// **'Thu'**
  String get scheduleDayThu;

  /// No description provided for @scheduleDayFri.
  ///
  /// In en, this message translates to:
  /// **'Fri'**
  String get scheduleDayFri;

  /// No description provided for @scheduleDaySat.
  ///
  /// In en, this message translates to:
  /// **'Sat'**
  String get scheduleDaySat;

  /// No description provided for @scheduleDaySun.
  ///
  /// In en, this message translates to:
  /// **'Sun'**
  String get scheduleDaySun;

  /// No description provided for @scenesTitle.
  ///
  /// In en, this message translates to:
  /// **'Scenes'**
  String get scenesTitle;

  /// No description provided for @scenesAdd.
  ///
  /// In en, this message translates to:
  /// **'New scene'**
  String get scenesAdd;

  /// No description provided for @scenesRun.
  ///
  /// In en, this message translates to:
  /// **'Run'**
  String get scenesRun;

  /// No description provided for @scenesName.
  ///
  /// In en, this message translates to:
  /// **'Scene name'**
  String get scenesName;

  /// No description provided for @scenesDescription.
  ///
  /// In en, this message translates to:
  /// **'Description'**
  String get scenesDescription;

  /// No description provided for @scenesAddStep.
  ///
  /// In en, this message translates to:
  /// **'Add step'**
  String get scenesAddStep;

  /// No description provided for @scenesEmpty.
  ///
  /// In en, this message translates to:
  /// **'No scenes yet'**
  String get scenesEmpty;

  /// No description provided for @scenesRunSuccess.
  ///
  /// In en, this message translates to:
  /// **'{total} steps completed'**
  String scenesRunSuccess(int total);

  /// No description provided for @scenesRunPartial.
  ///
  /// In en, this message translates to:
  /// **'{success}/{total} steps OK'**
  String scenesRunPartial(int success, int total);

  /// No description provided for @analyticsTitle.
  ///
  /// In en, this message translates to:
  /// **'Analytics'**
  String get analyticsTitle;

  /// No description provided for @analyticsSensorsSeries.
  ///
  /// In en, this message translates to:
  /// **'Sensor series'**
  String get analyticsSensorsSeries;

  /// No description provided for @analyticsDeviceUsage.
  ///
  /// In en, this message translates to:
  /// **'Device usage'**
  String get analyticsDeviceUsage;

  /// No description provided for @analyticsEnergy.
  ///
  /// In en, this message translates to:
  /// **'Energy'**
  String get analyticsEnergy;

  /// No description provided for @analyticsMetric.
  ///
  /// In en, this message translates to:
  /// **'Metric'**
  String get analyticsMetric;

  /// No description provided for @analyticsBucket.
  ///
  /// In en, this message translates to:
  /// **'Bucket'**
  String get analyticsBucket;

  /// No description provided for @analyticsLastWeek.
  ///
  /// In en, this message translates to:
  /// **'Last 7 days'**
  String get analyticsLastWeek;

  /// No description provided for @notificationsTitle.
  ///
  /// In en, this message translates to:
  /// **'Notifications'**
  String get notificationsTitle;

  /// No description provided for @notificationsMarkAllRead.
  ///
  /// In en, this message translates to:
  /// **'Mark all read'**
  String get notificationsMarkAllRead;

  /// No description provided for @notificationsEmpty.
  ///
  /// In en, this message translates to:
  /// **'No notifications'**
  String get notificationsEmpty;

  /// No description provided for @profileTitle.
  ///
  /// In en, this message translates to:
  /// **'Profile'**
  String get profileTitle;

  /// No description provided for @profileLanguage.
  ///
  /// In en, this message translates to:
  /// **'Language'**
  String get profileLanguage;

  /// No description provided for @profilePhone.
  ///
  /// In en, this message translates to:
  /// **'Phone'**
  String get profilePhone;

  /// No description provided for @profileChangePassword.
  ///
  /// In en, this message translates to:
  /// **'Change password'**
  String get profileChangePassword;

  /// No description provided for @profileCurrentPassword.
  ///
  /// In en, this message translates to:
  /// **'Current password'**
  String get profileCurrentPassword;

  /// No description provided for @profileNewPassword.
  ///
  /// In en, this message translates to:
  /// **'New password'**
  String get profileNewPassword;

  /// No description provided for @profileLocalUrl.
  ///
  /// In en, this message translates to:
  /// **'Local server URL'**
  String get profileLocalUrl;

  /// No description provided for @profileLocalUrlHint.
  ///
  /// In en, this message translates to:
  /// **'e.g. http://192.168.1.100:8080'**
  String get profileLocalUrlHint;

  /// No description provided for @offlineNoInternet.
  ///
  /// In en, this message translates to:
  /// **'No internet connection'**
  String get offlineNoInternet;

  /// No description provided for @commonCannotUndo.
  ///
  /// In en, this message translates to:
  /// **'This action cannot be undone.'**
  String get commonCannotUndo;

  /// No description provided for @scenesStepCount.
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{1 step} other{{count} steps}}'**
  String scenesStepCount(int count);

  /// No description provided for @analyticsNoData.
  ///
  /// In en, this message translates to:
  /// **'No data'**
  String get analyticsNoData;

  /// No description provided for @analyticsNoUsageData.
  ///
  /// In en, this message translates to:
  /// **'No usage data'**
  String get analyticsNoUsageData;

  /// No description provided for @scenesAddAtLeastOneStep.
  ///
  /// In en, this message translates to:
  /// **'Add at least one step'**
  String get scenesAddAtLeastOneStep;

  /// No description provided for @scenesNameRequired.
  ///
  /// In en, this message translates to:
  /// **'Scene name is required'**
  String get scenesNameRequired;

  /// No description provided for @commonRequired.
  ///
  /// In en, this message translates to:
  /// **'Required'**
  String get commonRequired;

  /// No description provided for @profilePasswordChanged.
  ///
  /// In en, this message translates to:
  /// **'Password changed successfully'**
  String get profilePasswordChanged;

  /// No description provided for @profilePasswordMin6.
  ///
  /// In en, this message translates to:
  /// **'Min 6 characters'**
  String get profilePasswordMin6;

  /// No description provided for @commonNotSet.
  ///
  /// In en, this message translates to:
  /// **'Not set'**
  String get commonNotSet;

  /// No description provided for @scenesStepsLabel.
  ///
  /// In en, this message translates to:
  /// **'Steps'**
  String get scenesStepsLabel;

  /// No description provided for @iotWizardEntityState.
  ///
  /// In en, this message translates to:
  /// **'State: {state}'**
  String iotWizardEntityState(String state);
}

class _AppLocalizationsDelegate
    extends LocalizationsDelegate<AppLocalizations> {
  const _AppLocalizationsDelegate();

  @override
  Future<AppLocalizations> load(Locale locale) {
    return SynchronousFuture<AppLocalizations>(lookupAppLocalizations(locale));
  }

  @override
  bool isSupported(Locale locale) =>
      <String>['en', 'kk', 'ru'].contains(locale.languageCode);

  @override
  bool shouldReload(_AppLocalizationsDelegate old) => false;
}

AppLocalizations lookupAppLocalizations(Locale locale) {
  // Lookup logic when only language code is specified.
  switch (locale.languageCode) {
    case 'en':
      return AppLocalizationsEn();
    case 'kk':
      return AppLocalizationsKk();
    case 'ru':
      return AppLocalizationsRu();
  }

  throw FlutterError(
      'AppLocalizations.delegate failed to load unsupported locale "$locale". This is likely '
      'an issue with the localizations generation tool. Please file an issue '
      'on GitHub with a reproducible sample app and the gen-l10n configuration '
      'that was used.');
}
