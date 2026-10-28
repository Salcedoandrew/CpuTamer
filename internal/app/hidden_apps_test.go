package app

import (
	"testing"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestHiddenAppsByKey(t *testing.T) {
	hidden := hiddenAppsByKey([]core.AppProcessHint{
		{AppID: core.AppID{BundleID: "com.example.app", Name: "Example"}, Hidden: true},
		{AppID: core.AppID{BundleID: "com.example.visible", Name: "Visible"}, Hidden: false},
		{AppID: core.AppID{BundleID: "com.example.shared", Name: "Shared"}, Hidden: true},
		{AppID: core.AppID{BundleID: "com.example.other", Name: "Shared"}, Hidden: false},
	})

	if !hidden["bundle:com.example.app"] {
		t.Fatal("want bundle key for hidden app")
	}
	if !hidden["name:example"] {
		t.Fatal("want name fallback for hidden app")
	}
	if hidden["bundle:com.example.visible"] {
		t.Fatal("visible app must not appear in hidden map")
	}
	if !hidden["name:shared"] {
		t.Fatal("a visible app sharing a name must not clear the hidden entry")
	}
}
