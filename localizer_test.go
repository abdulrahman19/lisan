package lisan

import (
	"errors"
	"strings"
	"testing"
)

func TestTranslateText(t *testing.T) {
	t.Parallel()

	instance := mustSample(t)

	tests := []struct {
		name   string
		locale string
		id     string
		args   Args
		want   string
	}{
		{name: "default locale", locale: "en-US", id: "user.msg.hi", want: "Hello"},
		{name: "arabic", locale: "ar-EG", id: "user.msg.hi", want: "مرحبًا"},
		{name: "polish", locale: "pl-PL", id: "user.msg.hi", want: "Cześć"},
		{
			name: "region code selects the region", locale: "ar",
			id: "user.msg.hi", want: "مرحبًا",
		},
		{
			name: "sibling locale shares the region", locale: "ar-MA",
			id: "user.msg.hi", want: "مرحبًا",
		},
		{
			name: "placeholder and global", locale: "en-US", id: "user.msg.welcome",
			args: Args{"name": "John"},
			want: "Welcome John, nice to have you in MyApp!",
		},
		{
			name: "global only", locale: "en-US", id: "user.msg.footer",
			want: "Email help@myapp.com.",
		},
		{
			name: "extra args are ignored", locale: "en-US", id: "user.msg.welcome",
			args: Args{"name": "John", "unused": "x"},
			want: "Welcome John, nice to have you in MyApp!",
		},
		{
			name: "unknown locale falls back to the default", locale: "fr-FR",
			id: "user.msg.hi", want: "Hello",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := instance.Locale(test.locale).T(test.id, test.args)
			if got.Err != nil {
				t.Fatalf("T(%q) returned error: %v", test.id, got.Err)
			}

			if got.Text != test.want {
				t.Errorf("T(%q) = %q, want %q", test.id, got.Text, test.want)
			}
		})
	}
}

func TestTranslatePlural(t *testing.T) {
	t.Parallel()

	instance := mustSample(t)

	tests := []struct {
		name   string
		locale string
		count  any
		want   string
	}{
		{name: "en zero", locale: "en-US", count: 0, want: "Sara, you have 0 active sessions."},
		{name: "en one", locale: "en-US", count: 1, want: "Sara, you have one active session."},
		{name: "en two", locale: "en-US", count: 2, want: "Sara, you have 2 active sessions."},
		{name: "en five", locale: "en-US", count: 5, want: "Sara, you have 5 active sessions."},
		{name: "en twenty two", locale: "en-US", count: 22, want: "Sara, you have 22 active sessions."},

		{name: "ar zero", locale: "ar", count: 0, want: "Sara، ليس لديك جلسات نشطة."},
		{name: "ar one", locale: "ar", count: 1, want: "Sara، لديك جلسة نشطة واحدة."},
		{name: "ar two", locale: "ar", count: 2, want: "Sara، لديك جلستان نشطتان."},
		{name: "ar few", locale: "ar", count: 5, want: "Sara، لديك ٥ جلسات نشطة."},
		{name: "ar many", locale: "ar", count: 22, want: "Sara، لديك ٢٢ جلسةً نشطة."},
		{name: "ar other", locale: "ar", count: 100, want: "Sara، لديك ١٠٠ جلسة نشطة."},

		{name: "pl zero is many", locale: "pl-PL", count: 0, want: "Sara, masz 0 aktywnych sesji."},
		{name: "pl one", locale: "pl-PL", count: 1, want: "Sara, masz jedną aktywną sesję."},
		{name: "pl two is few", locale: "pl-PL", count: 2, want: "Sara, masz 2 aktywne sesje."},
		{name: "pl five is many", locale: "pl-PL", count: 5, want: "Sara, masz 5 aktywnych sesji."},
		{name: "pl twenty two is few", locale: "pl-PL", count: 22, want: "Sara, masz 22 aktywne sesje."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := instance.Locale(test.locale).TN("user.session.active", N(test.count), Args{"name": "Sara"})
			if got.Err != nil {
				t.Fatalf("TN() returned error: %v", got.Err)
			}

			if got.Text != test.want {
				t.Errorf("TN(%v) in %s = %q, want %q", test.count, test.locale, got.Text, test.want)
			}
		})
	}
}

func TestPluralUsesLocaleDigits(t *testing.T) {
	t.Parallel()

	instance := mustSample(t)

	// The region tag decides plural category; the locale tag decides digits.
	// golang.org/x/text renders ar with Arabic-Indic digits and ar-EG without.
	tests := []struct {
		locale string
		want   string
	}{
		{locale: "ar", want: "Sara، لديك ٥ جلسات نشطة."},
		{locale: "ar-EG", want: "Sara، لديك 5 جلسات نشطة."},
	}

	for _, test := range tests {
		t.Run(test.locale, func(t *testing.T) {
			t.Parallel()

			got := instance.Locale(test.locale).TN("user.session.active", N(5), Args{"name": "Sara"})
			if got.Text != test.want {
				t.Errorf("TN() in %s = %q, want %q", test.locale, got.Text, test.want)
			}
		})
	}
}

func TestPluralLabels(t *testing.T) {
	t.Parallel()

	instance := mustSample(t)

	tests := []struct {
		locale string
		count  int
		want   string
	}{
		{locale: "en-US", count: 1, want: "user"},
		{locale: "en-US", count: 5, want: "users"},
		{locale: "ar", count: 1, want: "مستخدم"},
		{locale: "ar", count: 2, want: "مستخدمان"},
		{locale: "ar", count: 5, want: "مستخدمين"},
		{locale: "ar", count: 25, want: "مستخدمًا"},
		{locale: "pl-PL", count: 1, want: "użytkownik"},
		{locale: "pl-PL", count: 2, want: "użytkownicy"},
		{locale: "pl-PL", count: 5, want: "użytkowników"},
	}

	for _, test := range tests {
		t.Run(test.locale+"/"+test.want, func(t *testing.T) {
			t.Parallel()

			if got := instance.Locale(test.locale).TN("user.label", N(test.count)).Text; got != test.want {
				t.Errorf("TN(user.label, %d) in %s = %q, want %q", test.count, test.locale, got, test.want)
			}
		})
	}
}

func TestLookupFailures(t *testing.T) {
	t.Parallel()

	instance := mustSample(t)
	english := instance.Locale("en-US")

	tests := []struct {
		name    string
		call    func() Translation
		wantErr string
	}{
		{
			name:    "unknown id",
			call:    func() Translation { return english.T("nope") },
			wantErr: "unknown translation id",
		},
		{
			name:    "T on a plural entry",
			call:    func() Translation { return english.T("user.label") },
			wantErr: "use TN instead of T",
		},
		{
			name:    "TN on a non-plural entry",
			call:    func() Translation { return english.TN("user.msg.hi", N(1)) },
			wantErr: "use T instead of TN",
		},
		{
			name:    "missing placeholder value",
			call:    func() Translation { return english.T("user.msg.welcome") },
			wantErr: "is missing a value for placeholder {{name}}",
		},
		{
			name:    "count is not a number",
			call:    func() Translation { return english.TN("user.label", N("five")) },
			wantErr: "was given a count that is not a number",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := test.call()
			if got.Err == nil {
				t.Fatalf("call succeeded with %q, want an error", got.Text)
			}

			if !strings.Contains(got.Err.Error(), test.wantErr) {
				t.Errorf("error = %v, want it to contain %q", got.Err, test.wantErr)
			}
		})
	}
}

func TestTReturnsIdentifierOnMiss(t *testing.T) {
	t.Parallel()

	got := mustSample(t).T("does.not.exist")

	if got.Text != "does.not.exist" {
		t.Errorf("Text = %q, want the identifier verbatim", got.Text)
	}

	var missing *MissingTranslationError
	if !errors.As(got.Err, &missing) {
		t.Fatalf("Err = %T, want *MissingTranslationError", got.Err)
	}

	if missing.Locale != "en-US" {
		t.Errorf("Locale = %q, want %q", missing.Locale, "en-US")
	}
}

func TestTryVariants(t *testing.T) {
	t.Parallel()

	instance := mustSample(t)

	text, err := instance.TryT("user.msg.hi")
	if err != nil || text != "Hello" {
		t.Errorf("TryT() = (%q, %v), want (\"Hello\", nil)", text, err)
	}

	text, err = instance.TryT("nope")
	if err == nil || text != missingKeyText {
		t.Errorf("TryT(nope) = (%q, %v), want (%q, error)", text, err, missingKeyText)
	}

	text, err = instance.TryTN("user.label", N(5))
	if err != nil || text != "users" {
		t.Errorf("TryTN() = (%q, %v), want (\"users\", nil)", text, err)
	}

	text, err = instance.TryTN("nope", N(5))
	if err == nil || text != missingKeyText {
		t.Errorf("TryTN(nope) = (%q, %v), want (%q, error)", text, err, missingKeyText)
	}
}

func TestOnMissingHook(t *testing.T) {
	t.Parallel()

	var events []MissingEvent

	instance := mustSample(t, WithOnMissing(func(event MissingEvent) {
		events = append(events, event)
	}))

	instance.Locale("pl-PL").T("nope")

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	if events[0].ID != "nope" || events[0].Locale != "pl-PL" {
		t.Errorf("event = %+v, want ID=nope Locale=pl-PL", events[0])
	}
}

func TestWithDefaultLocale(t *testing.T) {
	t.Parallel()

	instance := mustSample(t, WithDefaultLocale("pl-PL"))

	if got := instance.DefaultLocale(); got != "pl-PL" {
		t.Errorf("DefaultLocale() = %q, want %q", got, "pl-PL")
	}

	if got := instance.T("user.msg.hi").Text; got != "Cześć" {
		t.Errorf("T() = %q, want %q", got, "Cześć")
	}
}

func TestRegionInformation(t *testing.T) {
	t.Parallel()

	instance := mustSample(t)

	if got, want := instance.Regions(), []string{"ar", "en-US", "pl-PL"}; !equalStrings(got, want) {
		t.Errorf("Regions() = %v, want %v", got, want)
	}

	if got := instance.GetRegionName("ar"); got != "عربي" {
		t.Errorf("GetRegionName(ar) = %q, want %q", got, "عربي")
	}

	if got, want := instance.GetRegionLocales("ar"), []string{"ar-EG", "ar-SA", "ar-MA"}; !equalStrings(got, want) {
		t.Errorf("GetRegionLocales(ar) = %v, want %v", got, want)
	}

	if got := instance.GetRegionName("nope"); got != "" {
		t.Errorf("GetRegionName(nope) = %q, want empty", got)
	}

	if got := instance.GetRegionLocales("nope"); got != nil {
		t.Errorf("GetRegionLocales(nope) = %v, want nil", got)
	}
}

func TestLocalizerCodeAndTag(t *testing.T) {
	t.Parallel()

	localizer := mustSample(t).Locale("ar-SA")

	if got := localizer.Code(); got != "ar-SA" {
		t.Errorf("Code() = %q, want %q", got, "ar-SA")
	}

	if got := localizer.Tag().String(); got != "ar-SA" {
		t.Errorf("Tag() = %q, want %q", got, "ar-SA")
	}
}

func TestNilLocalizerIsSafe(t *testing.T) {
	t.Parallel()

	var localizer *Localizer

	if got := localizer.Code(); got != "" {
		t.Errorf("Code() on nil = %q, want empty", got)
	}

	if got := localizer.Tag().String(); got != "und" {
		t.Errorf("Tag() on nil = %q, want %q", got, "und")
	}

	result := localizer.T("anything")
	if !errors.Is(result.Err, ErrNotConfigured) {
		t.Errorf("Err = %v, want ErrNotConfigured", result.Err)
	}

	if _, err := localizer.TryTN("anything", N(1)); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("TryTN err = %v, want ErrNotConfigured", err)
	}
}

func TestMergedArgs(t *testing.T) {
	t.Parallel()

	instance := mustSample(t)

	got := instance.T("user.msg.welcome", Args{"unused": 1}, Args{"name": "Ann"})
	if got.Text != "Welcome Ann, nice to have you in MyApp!" {
		t.Errorf("T() = %q with merged args", got.Text)
	}
}

// equalStrings compares two string slices element by element.
func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}

	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}

	return true
}
