#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>
#import <dispatch/dispatch.h>
#import <stdlib.h>
#import <string.h>

#import "notifications_bridge.h"

extern void opentamer_menu_command(const char *command);
extern void OpenTamerShowPreferencesMenu(void);

static NSString *const OpenTamerHighCPUActionContinue = @"opentamer-high-cpu-continue";
static NSString *const OpenTamerHighCPUActionLimit = @"opentamer-high-cpu-limit";
static NSString *const OpenTamerHighCPUActionMute = @"opentamer-high-cpu-mute";
static NSString *const OpenTamerHighCPUActionSettings = @"opentamer-high-cpu-settings";

@interface OpenTamerNotificationDelegate : NSObject <UNUserNotificationCenterDelegate>
+ (instancetype)sharedDelegate;
@end

@implementation OpenTamerNotificationDelegate

+ (instancetype)sharedDelegate {
    static OpenTamerNotificationDelegate *delegate = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        delegate = [[OpenTamerNotificationDelegate alloc] init];
    });
    return delegate;
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
         willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions options))completionHandler {
    UNNotificationPresentationOptions options = UNNotificationPresentationOptionSound |
        UNNotificationPresentationOptionBanner |
        UNNotificationPresentationOptionList;
    completionHandler(options);
}

// Routes a warning action back into Go through the same menu-command
// path the menu bar menus use. "Let it continue" needs no command:
// the detector's cooldown already spaces out repeat warnings, and a
// fresh duration is required before the next one. Tapping the warning
// itself opens the Preferences menu, same as "Change notification
// settings".
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
didReceiveNotificationResponse:(UNNotificationResponse *)response
         withCompletionHandler:(void (^)(void))completionHandler {
    NSString *actionIdentifier = response.actionIdentifier;
    NSString *appKey = response.notification.request.content.userInfo[@"appKey"];
    if ([appKey isKindOfClass:NSString.class] && appKey.length > 0) {
        if ([actionIdentifier isEqualToString:OpenTamerHighCPUActionLimit]) {
            // Background-only 25% cap: the same default the Limit CPU
            // custom prompt offers, applied as a managed rule.
            NSString *command = [NSString stringWithFormat:@"rule|limit-background|25|%@", appKey];
            opentamer_menu_command(command.UTF8String);
        } else if ([actionIdentifier isEqualToString:OpenTamerHighCPUActionMute]) {
            NSString *command = [NSString stringWithFormat:@"mute-high-cpu|%@", appKey];
            opentamer_menu_command(command.UTF8String);
        }
    }
    if ([actionIdentifier isEqualToString:OpenTamerHighCPUActionSettings] ||
        [actionIdentifier isEqualToString:UNNotificationDefaultActionIdentifier]) {
        dispatch_async(dispatch_get_main_queue(), ^{
            OpenTamerShowPreferencesMenu();
        });
    }
    completionHandler();
}

@end

// One warning category per app, so the mute button can carry the app's
// name ("Don't warn about Aftershoot"). Categories accumulate in a
// static set: replacing the center's categories outright would drop
// the actions on warnings already sitting in Notification Center.
static NSMutableSet<UNNotificationCategory *> *OpenTamerWarningCategories(void) {
    static NSMutableSet<UNNotificationCategory *> *categories = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        categories = [NSMutableSet set];
    });
    return categories;
}

static NSString *OpenTamerSanitizedCategorySuffix(NSString *appKey) {
    NSCharacterSet *allowed = [NSCharacterSet characterSetWithCharactersInString:@"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"];
    NSMutableString *sanitized = [NSMutableString string];
    for (NSUInteger i = 0; i < appKey.length; i++) {
        unichar c = [appKey characterAtIndex:i];
        if ([allowed characterIsMember:c]) {
            [sanitized appendFormat:@"%C", c];
        } else {
            [sanitized appendString:@"-"];
        }
    }
    if (sanitized.length == 0) {
        return @"app";
    }
    return sanitized;
}

static void OpenTamerRegisterWarningCategory(NSString *categoryIdentifier, NSString *appName) {
    UNNotificationAction *continueAction = [UNNotificationAction actionWithIdentifier:OpenTamerHighCPUActionContinue
                                                                                  title:@"Let it continue"
                                                                                options:0];
    UNNotificationAction *limitAction = [UNNotificationAction actionWithIdentifier:OpenTamerHighCPUActionLimit
                                                                               title:@"Limit its CPU usage"
                                                                             options:0];
    NSString *muteTitle = [NSString stringWithFormat:@"Don't warn about %@", appName];
    UNNotificationAction *muteAction = [UNNotificationAction actionWithIdentifier:OpenTamerHighCPUActionMute
                                                                              title:muteTitle
                                                                            options:0];
    UNNotificationAction *settingsAction = [UNNotificationAction actionWithIdentifier:OpenTamerHighCPUActionSettings
                                                                                  title:@"Change notification settings"
                                                                                options:0];
    UNNotificationCategory *category = [UNNotificationCategory categoryWithIdentifier:categoryIdentifier
                                                                                actions:@[continueAction, limitAction, muteAction, settingsAction]
                                                                      intentIdentifiers:@[]
                                                                                options:UNNotificationCategoryOptionNone];
    NSMutableSet<UNNotificationCategory *> *categories = OpenTamerWarningCategories();
    [categories addObject:category];
    [UNUserNotificationCenter.currentNotificationCenter setNotificationCategories:categories];
}

static char *OpenTamerCopyCString(NSString *message) {
    if (message == nil) {
        return strdup("OpenTamer notification failed");
    }
    const char *utf8 = [message UTF8String];
    if (utf8 == NULL) {
        return strdup("OpenTamer notification failed");
    }
    return strdup(utf8);
}

static char *OpenTamerCopyNSError(NSString *prefix, NSError *error) {
    if (error == nil) {
        return OpenTamerCopyCString(prefix);
    }
    NSString *details = error.localizedDescription ?: error.description;
    return OpenTamerCopyCString([NSString stringWithFormat:@"%@: %@", prefix, details]);
}

static void OpenTamerAssignError(char **error_out, char *message) {
    if (message == NULL) {
        return;
    }
    if (error_out != NULL && *error_out == NULL) {
        *error_out = message;
        return;
    }
    free(message);
}

static int OpenTamerNotificationAuthorized(UNUserNotificationCenter *center, char **error_out) {
    __block BOOL authorized = NO;
    __block char *authorizationError = NULL;
    dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);

    [center getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings) {
        switch (settings.authorizationStatus) {
            case UNAuthorizationStatusAuthorized:
            case UNAuthorizationStatusProvisional:
                authorized = YES;
                dispatch_semaphore_signal(semaphore);
                break;
            case UNAuthorizationStatusNotDetermined: {
                [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                                      completionHandler:^(BOOL granted, NSError * _Nullable error) {
                    authorized = granted && error == nil;
                    if (error != nil) {
                        authorizationError = OpenTamerCopyNSError(@"OpenTamer notification authorization failed", error);
                    } else if (!granted) {
                        authorizationError = OpenTamerCopyCString(@"OpenTamer notification authorization was not granted");
                    }
                    dispatch_semaphore_signal(semaphore);
                }];
                break;
            }
            case UNAuthorizationStatusDenied:
                authorizationError = OpenTamerCopyCString(@"OpenTamer notification authorization denied");
                dispatch_semaphore_signal(semaphore);
                break;
            default:
                authorizationError = OpenTamerCopyCString(@"OpenTamer notification authorization unavailable");
                dispatch_semaphore_signal(semaphore);
                break;
        }
    }];

    long result = dispatch_semaphore_wait(semaphore, dispatch_time(DISPATCH_TIME_NOW, 30 * NSEC_PER_SEC));
    if (result != 0) {
        free(authorizationError);
        OpenTamerAssignError(error_out, OpenTamerCopyCString(@"OpenTamer notification authorization timed out"));
        return 0;
    }
    if (!authorized) {
        OpenTamerAssignError(error_out, authorizationError != NULL ? authorizationError : OpenTamerCopyCString(@"OpenTamer notification authorization failed"));
        return 0;
    }
    free(authorizationError);
    return 1;
}

int OpenTamerDeliverHighCPUWarning(const char *title, const char *body, const char *appKey, const char *appName, char **error_out) {
    @autoreleasepool {
        if (error_out != NULL) {
            *error_out = NULL;
        }
        NSString *titleString = title == NULL ? @"OpenTamer" : [NSString stringWithUTF8String:title];
        NSString *bodyString = body == NULL ? @"" : [NSString stringWithUTF8String:body];
        NSString *appKeyString = appKey == NULL ? @"" : [NSString stringWithUTF8String:appKey];
        NSString *appNameString = appName == NULL ? @"" : [NSString stringWithUTF8String:appName];
        if (titleString == nil || bodyString == nil || appKeyString == nil || appNameString == nil) {
            OpenTamerAssignError(error_out, OpenTamerCopyCString(@"OpenTamer notification text is not valid UTF-8"));
            return 0;
        }

        UNUserNotificationCenter *center = UNUserNotificationCenter.currentNotificationCenter;
        center.delegate = [OpenTamerNotificationDelegate sharedDelegate];
        if (OpenTamerNotificationAuthorized(center, error_out) != 1) {
            return 0;
        }

        NSString *categoryIdentifier = [NSString stringWithFormat:@"opentamer-high-cpu-%@", OpenTamerSanitizedCategorySuffix(appKeyString)];
        OpenTamerRegisterWarningCategory(categoryIdentifier, appNameString);

        UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
        content.title = titleString;
        content.body = bodyString;
        content.categoryIdentifier = categoryIdentifier;
        content.userInfo = @{@"appKey": appKeyString};

        NSString *identifier = [NSString stringWithFormat:@"opentamer-high-cpu-%@", NSUUID.UUID.UUIDString];
        UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier
                                                                              content:content
                                                                              trigger:nil];

        __block BOOL delivered = NO;
        __block char *deliveryError = NULL;
        dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
        [center addNotificationRequest:request withCompletionHandler:^(NSError * _Nullable error) {
            delivered = error == nil;
            if (error != nil) {
                deliveryError = OpenTamerCopyNSError(@"OpenTamer notification delivery failed", error);
            }
            dispatch_semaphore_signal(semaphore);
        }];

        long result = dispatch_semaphore_wait(semaphore, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));
        if (result != 0) {
            free(deliveryError);
            OpenTamerAssignError(error_out, OpenTamerCopyCString(@"OpenTamer notification delivery timed out"));
            return 0;
        }
        if (!delivered) {
            OpenTamerAssignError(error_out, deliveryError != NULL ? deliveryError : OpenTamerCopyCString(@"OpenTamer notification delivery failed"));
            return 0;
        }
        free(deliveryError);
        return 1;
    }
}
