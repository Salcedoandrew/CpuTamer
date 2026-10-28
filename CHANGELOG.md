# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Release Policy

- `CHANGELOG.md` is the source of truth for release versions.
- `packaging/Info.plist` `CFBundleShortVersionString` must match the latest released changelog version without the leading `v`.
- `packaging/Info.plist` `CFBundleVersion` is the monotonically increasing Apple build number for distributable app bundles.

## Unreleased

## [v1.6.0] - 2026-10-27
### Added
- Actionable high-CPU warning (AppTamer-style): when an app stays above the threshold for the configured duration, the notification now reads "<App> has been using more than <threshold>% CPU for over <duration>." with "What would you like to do about it?" and four buttons: "Let it continue" (dismiss; the cooldown still spaces out repeat warnings), "Limit its CPU usage" (applies a background-only 25% CPU-limit rule to that app), "Don't warn about <App>" (mutes that app's warnings until unmuted from Preferences > High CPU Alerts > Muted Apps), and "Change notification settings" (opens the Preferences menu). Tapping the warning itself also opens Preferences.

## [v1.5.0] - 2026-10-27
### Added
- Per-app "Only stop or slow when hidden" rule option (AppTamer-style): pause, lower-priority, and CPU-limit rules now stay idle while the app is merely in the background and only apply once the app is hidden; unhiding restores full speed on the next tick.
- Per-app "Don't stop or slow when sound is playing" toggle in the per-app menu, alongside the hidden-only option. Audio protection was already default-on for new rules; it can now be switched off per app.
- Multi-core CPU-limit presets (AppPolice-style): 200% and 100% now sit above the existing presets in the Limit CPU submenu, and the custom prompt notes that multi-core apps can exceed 100%. The duty-cycle engine already handled targets above 100%; only the exposure was missing.
- "Show System Processes" preference (Preferences > General): system processes are now hidden from the process menus by default, with their CPU folded into the "System / Other" row. Turning it on restores the previous listing.

## [v1.4.1] - 2026-09-12
### Fixed
- Set the build deployment target to macOS 14.0.

## [v1.4.0] - 2026-09-06
### Added
- Background only CPU limiting, with `In Background` and `Always` modes for preset and custom limits.

## [v1.3.0] - 2026-06-30
### Added
- Persisted theme preference with default/backfill behavior (Thanks @kerem0x !)
### Updated
- git ignore to exclude built opentamer binaries.

## [v1.2.0] - 2026-06-20
### Added
- Ability to launch OpenTamer on login.


## [v1.1.0] - 2026-06-19
### Added
- Preference to swap between per-process and system normalized CPU usage display.


## [v1.0.0] - 2026-06-19
### Released
- My open source macOS menu bar utility for controlling app CPU usage.
