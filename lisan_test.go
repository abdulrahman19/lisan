package lisan

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// resetPackageState clears the instance behind the package-level API so that
// each test starts from a known state.
func resetPackageState(t *testing.T) {
	t.Helper()

	configured.Store(nil)
	t.Cleanup(func() { configured.Store(nil) })
}

func TestPackageAPIBeforeConfigure(t *testing.T) {
	resetPackageState(t)

	if got := T("x"); got.Text != "x" || !errors.Is(got.Err, ErrNotConfigured) {
		t.Errorf("T() = %+v, want the id with ErrNotConfigured", got)
	}

	if got := TN("x", N(1)); !errors.Is(got.Err, ErrNotConfigured) {
		t.Errorf("TN() err = %v, want ErrNotConfigured", got.Err)
	}

	if text, err := TryT("x"); text != missingKeyText || !errors.Is(err, ErrNotConfigured) {
		t.Errorf("TryT() = (%q, %v), want (%q, ErrNotConfigured)", text, err, missingKeyText)
	}

	if text, err := TryTN("x", N(1)); text != missingKeyText || !errors.Is(err, ErrNotConfigured) {
		t.Errorf("TryTN() = (%q, %v), want (%q, ErrNotConfigured)", text, err, missingKeyText)
	}

	assertEmptyPackageState(t)
}

// assertEmptyPackageState checks every accessor returns a zero value while the
// package is unconfigured.
func assertEmptyPackageState(t *testing.T) {
	t.Helper()

	if Default() != nil {
		t.Error("Default() is not nil before Configure")
	}

	if Locale("en-US") != nil {
		t.Error("Locale() is not nil before Configure")
	}

	if From(t.Context()) != nil {
		t.Error("From() is not nil before Configure")
	}

	if got := Regions(); got != nil {
		t.Errorf("Regions() = %v, want nil", got)
	}

	if got := GetRegionName("ar"); got != "" {
		t.Errorf("GetRegionName() = %q, want empty", got)
	}

	if got := GetRegionLocales("ar"); got != nil {
		t.Errorf("GetRegionLocales() = %v, want nil", got)
	}

	if got := DefaultLocale(); got != "" {
		t.Errorf("DefaultLocale() = %q, want empty", got)
	}

	if got := Negotiate("pl-PL"); got != "" {
		t.Errorf("Negotiate() = %q, want empty", got)
	}
}

func TestPackageAPIAfterConfigure(t *testing.T) {
	resetPackageState(t)

	if err := Configure(sampleTree()); err != nil {
		t.Fatalf("Configure() failed: %v", err)
	}

	if got := T("user.msg.hi").Text; got != "Hello" {
		t.Errorf("T() = %q, want %q", got, "Hello")
	}

	if got := TN("user.label", N(5)).Text; got != "users" {
		t.Errorf("TN() = %q, want %q", got, "users")
	}

	if got := Locale("pl-PL").T("user.msg.hi").Text; got != "Cześć" {
		t.Errorf("Locale().T() = %q, want %q", got, "Cześć")
	}

	ctx := WithLocale(t.Context(), "ar-EG")
	if got := From(ctx).T("user.msg.hi").Text; got != "مرحبًا" {
		t.Errorf("From().T() = %q, want %q", got, "مرحبًا")
	}

	if got := DefaultLocale(); got != "en-US" {
		t.Errorf("DefaultLocale() = %q, want %q", got, "en-US")
	}

	if got := GetRegionName("ar"); got != "عربي" {
		t.Errorf("GetRegionName() = %q, want %q", got, "عربي")
	}

	if got := Negotiate("pl-PL"); got != "pl-PL" {
		t.Errorf("Negotiate() = %q, want %q", got, "pl-PL")
	}

	if Default() == nil {
		t.Error("Default() is nil after Configure")
	}
}

func TestConfigureReportsCompileErrors(t *testing.T) {
	resetPackageState(t)

	err := Configure(tree(map[string]string{
		"config.json":  minimalConfig("en-US"),
		"en-US/a.json": `[{"id":"x","one":"a","many":"b"}]`,
	}))

	var compileErr *CompileError
	if !errors.As(err, &compileErr) {
		t.Fatalf("Configure() returned %T, want *CompileError", err)
	}

	if Default() != nil {
		t.Error("a failed Configure installed an instance")
	}
}

func TestPackageMiddleware(t *testing.T) {
	resetPackageState(t)

	handler := Middleware(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if _, err := writer.Write([]byte(From(request.Context()).T("user.msg.hi").Text)); err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))

	if err := Configure(sampleTree()); err != nil {
		t.Fatalf("Configure() failed: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(acceptLanguageHeader, "pl-PL")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Body.String(); got != "Cześć" {
		t.Errorf("body = %q, want %q", got, "Cześć")
	}
}

func TestPackageMiddlewarePassesThroughWhenUnconfigured(t *testing.T) {
	resetPackageState(t)

	called := false
	handler := Middleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !called {
		t.Error("the wrapped handler was not called while unconfigured")
	}
}

func TestCompileErrorMessage(t *testing.T) {
	t.Parallel()

	single := &CompileError{Problems: []Problem{
		{File: "a.json", Line: 3, Column: 5, ID: "x", Message: "is wrong"},
	}}

	if got, want := single.Error(), `lisan: compile error: a.json:3:5: id "x" is wrong`; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	many := &CompileError{Root: "locales/", Problems: []Problem{
		{File: "a.json", Message: "one"},
		{File: "b.json", Message: "two"},
	}}

	if got := many.Error(); got == "" || !errors.As(error(many), new(*CompileError)) {
		t.Errorf("Error() = %q, want a multi-problem summary", got)
	}
}

func TestMissingTranslationErrorMessage(t *testing.T) {
	t.Parallel()

	err := &MissingTranslationError{ID: "a.b", Locale: "pl-PL", Reason: "unknown translation id"}

	want := `lisan: unknown translation id (id "a.b", locale "pl-PL")`
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
