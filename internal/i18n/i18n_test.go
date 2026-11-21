package i18n

import (
	"context"
	"reflect"
	"regexp"
	"testing"
)

func TestLanguageAndText(t *testing.T) {
	for _, tc := range []struct{ language, want string }{
		{"", "en"}, {"en", "en"}, {"plt", "plt"}, {"mg-MG", "plt"},
		{"fr,plt;q=0.8,en;q=0.5", "fr"}, {"fr-FR", "fr"}, {"plt;q=0,en", "en"}, {"unknown", "en"},
	} {
		ctx := WithLanguage(context.Background(), tc.language)
		if got := Language(ctx); got != tc.want {
			t.Errorf("%q: %q", tc.language, got)
		}
	}
	if got := Text("plt", "Group %d / track %d: %s", 1, 2, "100%.wav"); got != "Vondrona 1 / raki-peo 2: 100%.wav" {
		t.Fatal(got)
	}
	if got := Text("en", "100% complete"); got != "100% complete" {
		t.Fatal(got)
	}
	if got := Text("plt", "external diagnostic"); got != "external diagnostic" {
		t.Fatal(got)
	}
}

func TestCatalogCoverageAndFormats(t *testing.T) {
	format := regexp.MustCompile(`%[-+# 0]*[0-9]*(?:\.[0-9]+)?[a-zA-Z%]`)
	for key := range malagasy {
		for name, catalog := range map[string]map[string]string{"plt": malagasy, "fr": french} {
			value, ok := catalog[key]
			if !ok || value == "" {
				t.Errorf("%s missing %q", name, key)
			}
			if !reflect.DeepEqual(format.FindAllString(key, -1), format.FindAllString(value, -1)) {
				t.Errorf("%s changed format placeholders: %q", name, key)
			}
		}
	}
	if got := Text("fr", "Group %d / track %d: %s", 1, 2, "/tmp/100%.wav"); got != "Groupe 1 / piste 2 : /tmp/100%.wav" {
		t.Fatal(got)
	}
	if Language(context.Background()) != "en" || Text("fr", "raw external diagnostic") != "raw external diagnostic" {
		t.Fatal("internal default or raw diagnostics changed")
	}
}

func TestVCDMessages(t *testing.T) {
	for _, key := range []string{
		"VCD video standard must be pal or ntsc",
		"VCD capacity must be cd74 or cd80",
		"VCD requires one group with 1-98 sequence tracks",
		"VCD exceeds selected CD capacity",
		"Explicit confirmation is required even for a simulated burn; cancellation may ruin the disc",
		"Select a real Linux optical recorder; files and device aliases are not allowed",
		"Burn cancelled; the disc may be unusable. Image files were retained.",
	} {
		if got := Text("en", key); got != key {
			t.Fatal(got)
		}
		if got := Text("plt", key); got == key || got == "" {
			t.Fatalf("missing translation: %s", key)
		}
	}
}
