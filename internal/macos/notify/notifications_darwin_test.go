//go:build darwin && cgo

package notify

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/app"
	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestNotifyHighCPURejectsNilNotifier(t *testing.T) {
	var notifier *UserNotifier
	err := notifier.NotifyHighCPU(app.HighCPUNotice{AppName: "Preview"})
	if err == nil || !strings.Contains(err.Error(), "notifier is not configured") {
		t.Fatalf("nil notifier error = %v", err)
	}
}

func TestNotifyHighCPUReturnsQueueFullWhenNativeDeliveryBacksUp(t *testing.T) {
	notifier := &UserNotifier{queue: make(chan app.HighCPUNotice)}

	err := notifier.NotifyHighCPU(app.HighCPUNotice{AppName: "Preview"})
	if err == nil || !strings.Contains(err.Error(), "notification queue full") {
		t.Fatalf("queue full error = %v", err)
	}
}

func TestDeliverHighCPUNotificationFormatsNativePayload(t *testing.T) {
	old := deliverWarningNotification
	t.Cleanup(func() {
		deliverWarningNotification = old
	})

	var gotTitle, gotBody, gotAppKey string
	deliverWarningNotification = func(title, body, appKey, appName string) error {
		gotTitle = title
		gotBody = body
		gotAppKey = appKey
		return nil
	}

	notice := app.HighCPUNotice{AppID: core.AppID{Name: "Preview"}, AppName: "Preview", CPUPercent: 87.6, ThresholdPercent: 75, AlertAfter: 30 * time.Second}
	if err := deliverHighCPUNotification(notice); err != nil {
		t.Fatalf("deliver high CPU notification: %v", err)
	}
	if gotTitle != "Preview has been using more than 75% CPU for over 30 seconds." {
		t.Fatalf("title = %q", gotTitle)
	}
	if gotBody != "What would you like to do about it?" {
		t.Fatalf("body = %q", gotBody)
	}
	if gotAppKey == "" {
		t.Fatal("app key was not passed to native delivery")
	}
}

func TestDeliverHighCPUNotificationPropagatesNativeError(t *testing.T) {
	old := deliverWarningNotification
	t.Cleanup(func() {
		deliverWarningNotification = old
	})

	want := errors.New("authorization denied")
	deliverWarningNotification = func(title, body, appKey, appName string) error {
		return want
	}

	err := deliverHighCPUNotification(app.HighCPUNotice{AppName: "Preview", CPUPercent: 87.6})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestUserNotifierDeliversQueuedHighCPUNotice(t *testing.T) {
	old := deliverWarningNotification
	t.Cleanup(func() {
		deliverWarningNotification = old
	})

	delivered := make(chan string, 1)
	deliverWarningNotification = func(title, body, appKey, appName string) error {
		delivered <- title + "\n" + body
		return nil
	}

	notifier := NewUserNotifier()
	t.Cleanup(func() {
		close(notifier.queue)
	})

	notice := app.HighCPUNotice{AppID: core.AppID{Name: "Preview"}, AppName: "Preview", CPUPercent: 87.6, ThresholdPercent: 75, AlertAfter: 30 * time.Second}
	if err := notifier.NotifyHighCPU(notice); err != nil {
		t.Fatalf("notify high CPU: %v", err)
	}

	select {
	case payload := <-delivered:
		if payload != "Preview has been using more than 75% CPU for over 30 seconds.\nWhat would you like to do about it?" {
			t.Fatalf("payload = %q", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("queued notification was not delivered")
	}
}
