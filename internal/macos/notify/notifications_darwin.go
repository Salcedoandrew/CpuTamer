//go:build darwin && cgo

package notify

/*
#cgo darwin LDFLAGS: -framework UserNotifications -framework Foundation
#include <stdlib.h>
#include "notifications_bridge.h"
*/
import "C"

import (
	"fmt"
	"log"
	"time"
	"unsafe"

	"github.com/Digital-Shane/open-tamer/internal/app"
)

const notificationQueueSize = 16

// deliverWarningNotification delivers the actionable high-CPU warning.
// It is a variable so darwin tests can stub the native delivery.
var deliverWarningNotification = deliverNativeWarningNotification

type UserNotifier struct {
	queue chan app.HighCPUNotice
}

func NewUserNotifier() *UserNotifier {
	notifier := &UserNotifier{queue: make(chan app.HighCPUNotice, notificationQueueSize)}
	go notifier.run()
	return notifier
}

func (notifier *UserNotifier) NotifyHighCPU(notice app.HighCPUNotice) error {
	if notifier == nil {
		return fmt.Errorf("notifier is not configured")
	}
	select {
	case notifier.queue <- notice:
		return nil
	default:
		return fmt.Errorf("notification queue full")
	}
}

func (notifier *UserNotifier) run() {
	for notice := range notifier.queue {
		if err := deliverHighCPUNotification(notice); err != nil {
			log.Printf("OpenTamer notification warning: %v", err)
		}
	}
}

// deliverHighCPUNotification formats the warning in the style of the
// AppTamer high-CPU popup: "<App> has been using more than <threshold>%
// CPU for over <duration>." with "What would you like to do about it?"
// as the body. The native side adds the action buttons.
func deliverHighCPUNotification(notice app.HighCPUNotice) error {
	threshold := notice.ThresholdPercent
	if threshold <= 0 {
		threshold = notice.CPUPercent
	}
	title := fmt.Sprintf("%s has been using more than %.0f%% CPU for over %s.", notice.AppName, threshold, formatAlertDuration(notice.AlertAfter))
	return deliverWarningNotification(title, "What would you like to do about it?", notice.AppID.Key(), notice.AppName)
}

// formatAlertDuration renders a configured alert-after duration the way
// the warning title reads it: seconds below a minute, whole minutes at
// or above it.
func formatAlertDuration(duration time.Duration) string {
	if duration >= time.Minute {
		minutes := int(duration / time.Minute)
		if minutes == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", minutes)
	}
	seconds := int(duration / time.Second)
	if seconds <= 1 {
		return "1 second"
	}
	return fmt.Sprintf("%d seconds", seconds)
}

func deliverNativeWarningNotification(titleText, bodyText, appKey, appName string) error {
	title := C.CString(titleText)
	body := C.CString(bodyText)
	key := C.CString(appKey)
	name := C.CString(appName)
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(body))
	defer C.free(unsafe.Pointer(key))
	defer C.free(unsafe.Pointer(name))

	var nativeError *C.char
	if C.OpenTamerDeliverHighCPUWarning(title, body, key, name, &nativeError) != 1 {
		if nativeError != nil {
			defer C.free(unsafe.Pointer(nativeError))
			message := C.GoString(nativeError)
			if message != "" {
				return fmt.Errorf("%s", message)
			}
		}
		return fmt.Errorf("deliver notification")
	}
	return nil
}
