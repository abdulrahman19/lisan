package lisan

import (
	"errors"
	"testing"
	"testing/fstest"
)

// tree builds an in-memory translation tree from a name-to-contents map.
func tree(files map[string]string) fstest.MapFS {
	fsys := make(fstest.MapFS, len(files))

	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}

	return fsys
}

// sampleConfig is the three-region configuration shared by most tests.
const sampleConfig = `{
  "settings": { "base_region": "en-US" },
  "globals": {
    "app_name": "MyApp",
    "support_email": "help@myapp.com"
  },
  "regions": {
    "ar":    { "name": "عربي",         "locales": ["ar-EG", "ar-SA", "ar-MA"] },
    "en-US": { "name": "English (US)", "locales": ["en-US"] },
    "pl-PL": { "name": "Polski",       "locales": ["pl-PL"] }
  }
}`

const sampleEnglish = `[
  { "id": "user.msg.hi", "txt": "Hello" },
  { "id": "user.msg.welcome", "txt": "Welcome {{name}}, nice to have you in {{%app_name%}}!" },
  { "id": "user.msg.footer", "txt": "Email {{%support_email%}}." },
  { "id": "user.label", "one": "user", "other": "users" },
  {
    "id": "user.session.active",
    "one": "{{name}}, you have one active session.",
    "other": "{{name}}, you have {{count}} active sessions."
  }
]`

const sampleArabic = `[
  { "id": "user.msg.hi", "txt": "مرحبًا" },
  { "id": "user.msg.welcome", "txt": "مرحبًا {{name}}، سعيد بوجودك في {{%app_name%}}!" },
  { "id": "user.msg.footer", "txt": "راسلنا على {{%support_email%}}." },
  {
    "id": "user.label",
    "zero": "مستخدم", "one": "مستخدم", "two": "مستخدمان",
    "few": "مستخدمين", "many": "مستخدمًا", "other": "مستخدم"
  },
  {
    "id": "user.session.active",
    "zero": "{{name}}، ليس لديك جلسات نشطة.",
    "one": "{{name}}، لديك جلسة نشطة واحدة.",
    "two": "{{name}}، لديك جلستان نشطتان.",
    "few": "{{name}}، لديك {{count}} جلسات نشطة.",
    "many": "{{name}}، لديك {{count}} جلسةً نشطة.",
    "other": "{{name}}، لديك {{count}} جلسة نشطة."
  }
]`

const samplePolish = `[
  { "id": "user.msg.hi", "txt": "Cześć" },
  { "id": "user.msg.welcome", "txt": "Witaj {{name}}, miło Cię widzieć w {{%app_name%}}!" },
  { "id": "user.msg.footer", "txt": "Napisz na {{%support_email%}}." },
  {
    "id": "user.label",
    "one": "użytkownik", "few": "użytkownicy",
    "many": "użytkowników", "other": "użytkownika"
  },
  {
    "id": "user.session.active",
    "one": "{{name}}, masz jedną aktywną sesję.",
    "few": "{{name}}, masz {{count}} aktywne sesje.",
    "many": "{{name}}, masz {{count}} aktywnych sesji.",
    "other": "{{name}}, masz {{count}} sesji."
  }
]`

// sampleTree returns the three-region tree used across the test suite. The
// Arabic file is named in Arabic on purpose: files are matched by the ids they
// contain, never by name.
func sampleTree() fstest.MapFS {
	return tree(map[string]string{
		"config.json":      sampleConfig,
		"en-US/users.json": sampleEnglish,
		"ar/مستخدمين.json": sampleArabic,
		"pl-PL/users.json": samplePolish,
	})
}

// mustCompile compiles a tree and fails the test if it does not.
func mustCompile(t *testing.T, files map[string]string, opts ...Option) *I18n {
	t.Helper()

	instance, err := New(tree(files), opts...)
	if err != nil {
		t.Fatalf("New() failed unexpectedly: %v", err)
	}

	return instance
}

// mustSample compiles the shared three-region tree.
func mustSample(t *testing.T, opts ...Option) *I18n {
	t.Helper()

	instance, err := New(sampleTree(), opts...)
	if err != nil {
		t.Fatalf("New() on the sample tree failed: %v", err)
	}

	return instance
}

// compileProblems compiles a tree expecting failure and returns the problems.
func compileProblems(t *testing.T, files map[string]string) []Problem {
	t.Helper()

	_, err := New(tree(files))
	if err == nil {
		t.Fatal("New() succeeded, want a compile error")
	}

	var compileErr *CompileError
	if !errors.As(err, &compileErr) {
		t.Fatalf("New() returned %T, want *CompileError", err)
	}

	return compileErr.Problems
}
