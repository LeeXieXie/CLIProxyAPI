package tui

import "testing"

func TestUsageTabIsRegistered(t *testing.T) {
	prevLocale := CurrentLocale()
	t.Cleanup(func() {
		SetLocale(prevLocale)
	})
	SetLocale("en")

	names := TabNames()
	if tabUsage >= len(names) {
		t.Fatalf("tabUsage index %d outside tab names length %d", tabUsage, len(names))
	}
	if names[tabUsage] != "Usage" {
		t.Fatalf("TabNames()[tabUsage] = %q, want Usage", names[tabUsage])
	}

	app := NewApp(1234, "", nil)
	if tabUsage >= len(app.tabs) {
		t.Fatalf("tabUsage index %d outside app tabs length %d", tabUsage, len(app.tabs))
	}
	if app.tabs[tabUsage] != "Usage" {
		t.Fatalf("app.tabs[tabUsage] = %q, want Usage", app.tabs[tabUsage])
	}
}
