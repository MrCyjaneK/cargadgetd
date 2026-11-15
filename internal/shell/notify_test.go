package shell

import "testing"

const sampleGetNotifications = `([('ssh-test', uint32 20, '/usr/share/icons/hicolor/86x86/apps/sailfish-browser.png', 'Title here', 'Description body', @as [], {'x-nemo-preview-summary': <'Title here'>, 'x-nemo-icon': <'icon-lock-information'>, 'x-nemo-preview-body': <'Description body'>, 'x-nemo-priority': <50>, 'x-nemo-timestamp': <'2026-08-14T12:51:14Z'>}, 4000), ('ssh-test', 19, 'icon-lock-information', 'Cargadgetd test', 'Body from SSH', [], {'x-nemo-preview-summary': <'Cargadgetd test'>, 'x-nemo-preview-body': <'Body from SSH'>, 'x-nemo-priority': <50>, 'x-nemo-timestamp': <'2026-08-14T12:50:55Z'>}, 3000)],)`

func TestParseNotificationList(t *testing.T) {
	notes := parseNotificationList(sampleGetNotifications)
	if len(notes) != 2 {
		t.Fatalf("len=%d %#v", len(notes), notes)
	}
	if notes[0].ID != 20 || notes[0].Title != "Title here" || notes[0].Body != "Description body" {
		t.Fatalf("first %+v", notes[0])
	}
	if notes[0].Icon != "/usr/share/icons/hicolor/86x86/apps/sailfish-browser.png" {
		t.Fatalf("icon %q", notes[0].Icon)
	}
	if notes[1].ID != 19 || notes[1].Title != "Cargadgetd test" || notes[1].App != "ssh-test" {
		t.Fatalf("second %+v", notes[1])
	}
}

func TestParseNotificationPreviewAndEscape(t *testing.T) {
	out := `([('app', uint32 3, '', 'raw', 'raw-body', [], {'x-nemo-preview-summary': <'Don\'t miss'>, 'x-nemo-preview-body': <'Hello'>, 'x-nemo-icon': <'icon-lock-sms'>}, -1)],)`
	notes := parseNotificationList(out)
	if len(notes) != 1 {
		t.Fatalf("len=%d", len(notes))
	}
	if notes[0].Title != "Don't miss" || notes[0].Body != "Hello" || notes[0].Icon != "icon-lock-sms" {
		t.Fatalf("%+v", notes[0])
	}
}

func TestParseNotificationEmpty(t *testing.T) {
	if notes := parseNotificationList(`(@a(sussasa{sv}i) [],)`); len(notes) != 0 {
		t.Fatalf("%#v", notes)
	}
}
