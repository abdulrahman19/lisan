package lisan

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContextRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := WithLocale(t.Context(), "ar-EG")

	if got := FromContext(ctx); got != "ar-EG" {
		t.Errorf("FromContext() = %q, want %q", got, "ar-EG")
	}
}

func TestFromContextWithoutLocale(t *testing.T) {
	t.Parallel()

	if got := FromContext(t.Context()); got != "" {
		t.Errorf("FromContext() = %q, want empty", got)
	}
}

func TestInstanceFrom(t *testing.T) {
	t.Parallel()

	instance := mustSample(t)

	tests := []struct {
		name   string
		locale string
		want   string
	}{
		{name: "carried locale wins", locale: "pl-PL", want: "Cześć"},
		{name: "arabic", locale: "ar-SA", want: "مرحبًا"},
		{name: "empty context falls back to the default", locale: "", want: "Hello"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			if test.locale != "" {
				ctx = WithLocale(ctx, test.locale)
			}

			if got := instance.From(ctx).T("user.msg.hi").Text; got != test.want {
				t.Errorf("From(ctx).T() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNegotiate(t *testing.T) {
	t.Parallel()

	instance := mustSample(t)

	tests := []struct {
		name   string
		header string
		want   string
	}{
		{name: "exact locale", header: "pl-PL", want: "pl-PL"},
		{name: "sibling locale resolves to its region", header: "ar-MA,ar;q=0.9,en;q=0.5", want: "ar-MA"},
		{name: "region code", header: "ar", want: "ar"},
		{name: "unknown falls back to the base", header: "ja-JP", want: "en-US"},
		{name: "empty falls back to the base", header: "", want: "en-US"},
		{name: "malformed falls back to the base", header: "!!!", want: "en-US"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := instance.Negotiate(test.header); got != test.want {
				t.Errorf("Negotiate(%q) = %q, want %q", test.header, got, test.want)
			}
		})
	}
}

func TestMiddlewareSetsLocale(t *testing.T) {
	t.Parallel()

	instance := mustSample(t)

	tests := []struct {
		name   string
		header string
		want   string
	}{
		{name: "arabic", header: "ar-MA,ar;q=0.9", want: "مرحبًا"},
		{name: "polish", header: "pl-PL", want: "Cześć"},
		{name: "unmatched", header: "ja-JP", want: "Hello"},
		{name: "absent", header: "", want: "Hello"},
	}

	handler := instance.Middleware(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		text := instance.From(request.Context()).T("user.msg.hi").Text

		if _, err := writer.Write([]byte(text)); err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if test.header != "" {
				request.Header.Set(acceptLanguageHeader, test.header)
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if got := recorder.Body.String(); got != test.want {
				t.Errorf("body = %q, want %q", got, test.want)
			}
		})
	}
}

func TestMiddlewareKeepsAnExistingLocale(t *testing.T) {
	t.Parallel()

	instance := mustSample(t)

	handler := instance.Middleware(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if _, err := writer.Write([]byte(FromContext(request.Context()))); err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(acceptLanguageHeader, "pl-PL")
	request = request.WithContext(WithLocale(request.Context(), "ar-EG"))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Body.String(); got != "ar-EG" {
		t.Errorf("locale = %q, want %q; a stored preference must beat the header", got, "ar-EG")
	}
}

func TestLocalizersAreIndependentAcrossGoroutines(t *testing.T) {
	t.Parallel()

	instance := mustSample(t)

	cases := map[string]string{
		"en-US": "Hello",
		"ar-EG": "مرحبًا",
		"pl-PL": "Cześć",
	}

	for locale, want := range cases {
		t.Run(locale, func(t *testing.T) {
			t.Parallel()

			for range 200 {
				if got := instance.Locale(locale).T("user.msg.hi").Text; got != want {
					t.Fatalf("T() = %q, want %q", got, want)
				}
			}
		})
	}
}
