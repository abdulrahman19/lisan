package lisan

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing/fstest"
)

// ExampleNew builds a translation tree in memory and renders from it. A real
// program would pass os.DirFS("locales") or an embedded fs.FS instead.
func ExampleNew() {
	locales := fstest.MapFS{
		"config.json": &fstest.MapFile{Data: []byte(`{
            "settings": { "base_region": "en-US" },
            "globals":  { "app_name": "Acme" },
            "regions":  { "en-US": { "name": "English (US)", "locales": ["en-US"] } }
        }`)},
		"en-US/app.json": &fstest.MapFile{Data: []byte(`[
            { "id": "greeting", "txt": "Hello {{name}}, welcome to {{%app_name%}}!" },
            {
              "id": "inbox.unread",
              "one":   "You have one unread message.",
              "other": "You have {{count}} unread messages."
            }
        ]`)},
	}

	i18n, err := New(locales)
	if err != nil {
		panic(err)
	}

	fmt.Println(i18n.T("greeting", Args{"name": "Sara"}).Text)
	fmt.Println(i18n.TN("inbox.unread", N(1)).Text)
	fmt.Println(i18n.TN("inbox.unread", N(5)).Text)

	// Output:
	// Hello Sara, welcome to Acme!
	// You have one unread message.
	// You have 5 unread messages.
}

// ExampleI18n_TN shows one call site rendering correctly in three languages,
// with no plural category named anywhere in the Go code.
func ExampleI18n_TN() {
	i18n, err := New(sampleTree())
	if err != nil {
		panic(err)
	}

	for _, locale := range []string{"en-US", "ar", "pl-PL"} {
		localizer := i18n.Locale(locale)

		for _, count := range []int{1, 2, 5} {
			fmt.Printf("%-6s %d %s\n", locale, count, localizer.TN("user.label", N(count)).Text)
		}
	}

	// Output:
	// en-US  1 user
	// en-US  2 users
	// en-US  5 users
	// ar     1 مستخدم
	// ar     2 مستخدمان
	// ar     5 مستخدمين
	// pl-PL  1 użytkownik
	// pl-PL  2 użytkownicy
	// pl-PL  5 użytkowników
}

// ExampleI18n_Locale renders in a locale chosen at runtime, such as a
// recipient's stored preference.
func ExampleI18n_Locale() {
	i18n, err := New(sampleTree())
	if err != nil {
		panic(err)
	}

	fmt.Println(i18n.Locale("pl-PL").T("user.msg.welcome", Args{"name": "Jan"}).Text)

	// Output:
	// Witaj Jan, miło Cię widzieć w MyApp!
}

// ExampleI18n_Middleware negotiates Accept-Language and renders in the
// language the request asked for.
func ExampleI18n_Middleware() {
	i18n, err := New(sampleTree())
	if err != nil {
		panic(err)
	}

	handler := i18n.Middleware(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		localizer := i18n.From(request.Context())
		fmt.Fprintln(writer, localizer.T("user.msg.hi").Text)
	}))

	// ar-MA has no folder of its own; it is served by the ar region.
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept-Language", "ar-MA,ar;q=0.9,en;q=0.5")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	fmt.Print(recorder.Body.String())

	// Output:
	// مرحبًا
}

// ExampleWithLocale scopes the active locale to a context, which is what keeps
// concurrent requests from rendering each other's language.
func ExampleWithLocale() {
	i18n, err := New(sampleTree())
	if err != nil {
		panic(err)
	}

	ctx := WithLocale(context.Background(), "ar-EG")

	fmt.Println(FromContext(ctx))
	fmt.Println(i18n.From(ctx).T("user.msg.hi").Text)

	// Output:
	// ar-EG
	// مرحبًا
}

// ExampleI18n_TryT aborts the operation rather than rendering a fallback.
func ExampleI18n_TryT() {
	i18n, err := New(sampleTree())
	if err != nil {
		panic(err)
	}

	text, err := i18n.TryT("user.msg.hi")
	fmt.Printf("%q %v\n", text, err)

	text, err = i18n.TryT("typo.in.this.id")
	fmt.Printf("%q %v\n", text, err)

	// Output:
	// "Hello" <nil>
	// "missing_key" lisan: unknown translation id (id "typo.in.this.id", locale "en-US")
}

// ExampleCompileError shows the compiler refusing a tree and naming every
// defect, with a file, a line and an identifier.
func ExampleCompileError() {
	broken := fstest.MapFS{
		"config.json": &fstest.MapFile{Data: []byte(`{
            "settings": { "base_region": "en-US" },
            "regions":  { "en-US": { "name": "English", "locales": ["en-US"] } }
        }`)},
		"en-US/app.json": &fstest.MapFile{Data: []byte("[\n" +
			`  { "id": "user.label", "one": "user", "many": "users" }` + "\n]")},
	}

	_, err := New(broken)

	var compileErr *CompileError
	if errors.As(err, &compileErr) {
		for _, problem := range compileErr.Problems {
			fmt.Println(problem)
		}
	}

	// Output:
	// en-US/app.json:2:3: id "user.label" declares categories [one many] but en-US requires exactly [one other]; missing: other; unexpected: many
}

// ExampleMoney formats a monetary value for the target locale.
func ExampleMoney() {
	locales := fstest.MapFS{
		"config.json": &fstest.MapFile{Data: []byte(`{
            "settings": { "base_region": "en-US" },
            "regions":  {
              "en-US": { "name": "English", "locales": ["en-US"] },
              "pl-PL": { "name": "Polski",  "locales": ["pl-PL"] }
            }
        }`)},
		"en-US/app.json": &fstest.MapFile{Data: []byte(
			`[{ "id": "total", "txt": "Your total is {{amount:currency}} ({{tax:percent}} tax)." }]`)},
		"pl-PL/app.json": &fstest.MapFile{Data: []byte(
			`[{ "id": "total", "txt": "Razem {{amount:currency}} ({{tax:percent}} podatku)." }]`)},
	}

	i18n, err := New(locales)
	if err != nil {
		panic(err)
	}

	args := Args{"amount": Money(99.50, "USD"), "tax": 0.075}

	fmt.Println(i18n.Locale("en-US").T("total", args).Text)
	fmt.Println(i18n.Locale("pl-PL").T("total", args).Text)

	// Output:
	// Your total is $ 99.50 (7.5% tax).
	// Razem USD 99,50 (7,5% podatku).
}
