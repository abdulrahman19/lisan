# Lisan

**لسان** — compile-time-validated internationalization for Go.

Lisan reads JSON translation files, validates the whole tree before your program serves a request, and renders with CLDR-correct pluralization from `golang.org/x/text`.

## Features

- **Validated at startup.** Missing IDs, wrong plural categories, unknown placeholders and undefined globals fail `New`/`Configure` with a file, line and ID.
- **Plural categories are derived, never chosen.** You pass a number; Lisan asks CLDR which form the target locale needs.
- **Locale lives in `context.Context`.** No package-level mutable state, so concurrent requests can't cross languages.
- **Any `fs.FS`.** `os.DirFS` in development, `go:embed` in production.
- **In-memory compilation.** Nothing is written to disk; works in read-only containers.
- **Locale-aware number, percent and currency formatting.**
- **One dependency:** `golang.org/x/text`.

## Installation

```bash
go get github.com/abdulrahman19/lisan
```

Requires Go 1.26 or newer, as pinned by `golang.org/x/text`.

## Quick Start

```
locales/
├── config.json
└── en-US/
    └── app.json
```

`locales/config.json`

```json
{
  "settings": { "base_region": "en-US" },
  "globals":  { "app_name": "Acme" },
  "regions": {
    "en-US": { "name": "English (US)", "locales": ["en-US"] }
  }
}
```

`locales/en-US/app.json`

```json
[
  { "id": "greeting", "txt": "Hello {{name}}, welcome to {{%app_name%}}!" },
  {
    "id": "inbox.unread",
    "one":   "You have one unread message.",
    "other": "You have {{count}} unread messages."
  }
]
```

`main.go`

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/abdulrahman19/lisan"
)

func main() {
	if err := lisan.Configure(os.DirFS("locales")); err != nil {
		log.Fatal(err)
	}

	fmt.Println(lisan.T("greeting", lisan.Args{"name": "Sara"}).Text)
	// Hello Sara, welcome to Acme!

	fmt.Println(lisan.TN("inbox.unread", lisan.N(1)).Text)
	// You have one unread message.

	fmt.Println(lisan.TN("inbox.unread", lisan.N(5)).Text)
	// You have 5 unread messages.
}
```

## Translation Tree

Each **region** gets a folder. A region is a group of locales that share one set of files.

```
locales/
├── config.json
├── ar/            serves ar-EG, ar-SA, ar-MA
│   ├── منتجات.json
│   └── مستخدمين.json
├── en-US/
│   └── users.json
└── pl-PL/
    └── users.json
```

- The folder name must be a valid **BCP 47 tag** (`ar`, `en-US`, `pl-PL`, `zh-Hant`).
- Every `.json` file under a region folder is merged into one namespace, keyed by ID. File names and subfolders are free.
- Every locale a region serves must share that region's plural categories.

### config.json

Must sit at the root of the tree.

```json
{
  "settings": {
    "base_region": "en-US"
  },
  "globals": {
    "app_name":      "MyApp",
    "support_email": "help@myapp.com",
    "base_url":      "${BASE_URL}"
  },
  "regions": {
    "ar":    { "name": "عربي",         "locales": ["ar-EG", "ar-SA", "ar-MA"] },
    "en-US": { "name": "English (US)", "locales": ["en-US"] },
    "pl-PL": { "name": "Polski",       "locales": ["pl-PL"] }
  }
}
```

| Key | Meaning |
|---|---|
| `settings.base_region` | Defines the canonical set of IDs and placeholders. Every other region is validated against it. Also the default locale. |
| `globals` | Values injected via `{{%key%}}`, resolved once at compile time. |
| `regions[code].name` | Region name in its own language, for language pickers. |
| `regions[code].locales` | Locales served by that region's folder. |

Unknown keys are rejected. A locale may belong to only one region.

A global whose value is exactly `${VAR}` is replaced with `os.Getenv("VAR")` at compile time, so **load your environment before calling `Configure` or `New`**. An unset variable fails the build.

### Translation Files

A JSON array of entries. Each entry has an `id` plus **either** `txt` **or** a set of plural forms — never both.

| Key | Meaning |
|---|---|
| `id` | Unique identifier within the region. Required. |
| `txt` | Text for a non-plural entry. Read with `T`. |
| `zero` `one` `two` `few` `many` `other` | Plural forms. Read with `TN`. |

```json
[
  { "id": "user.msg.hi", "txt": "Hello" },
  { "id": "user.label", "one": "user", "other": "users" }
]
```

Any other key is a compile error, which catches `"text"` for `"txt"` or `"onee"` for `"one"`.

## Plural Categories

You never name a category in Go code. Lisan derives it from the number and the target locale.

Each language requires **exactly** its CLDR category set — no more, no fewer:

| Language | Required keys |
|---|---|
| English (`en`, `en-US`, `en-GB`), Turkish (`tr`) | `one`, `other` |
| Arabic (`ar`) | `zero`, `one`, `two`, `few`, `many`, `other` |
| Polish (`pl`), Russian (`ru`), Czech (`cs`) | `one`, `few`, `many`, `other` |
| Japanese (`ja`) | `other` |

How numbers map differs per language:

| n | `en` | `ar` | `pl` | `ru` |
|---|---|---|---|---|
| 0 | `other` | `zero` | `many` | `many` |
| 1 | `one` | `one` | `one` | `one` |
| 2 | `other` | `two` | `few` | `few` |
| 5 | `other` | `few` | `many` | `many` |
| 21 | `other` | `many` | `many` | `one` |
| 22 | `other` | `many` | `few` | `few` |

You don't need to memorise these — the compiler names the exact set:

```
compile error: locales/pl-PL/users.json:14:3: id "user.session.active" declares
  categories [zero one two few many other] but pl-PL requires exactly
  [one few many other]; unexpected: zero, two
```

`N` accepts any Go numeric type. Integers and floats differ on purpose:

```go
lisan.N(1)    // integer → "one" in en
lisan.N(1.0)  // float   → "other" in en, renders "1.0"
lisan.N(0.5)  // → "other"
```

## Placeholders

| Syntax | Filled from | When |
|---|---|---|
| `{{name}}` | the `Args` map | call time |
| `{{%app_name%}}` | `globals` in `config.json` | compile time |
| `{{count}}` | the `N()` value passed to `TN` | call time, plural entries only |
| `{{{{` | — | renders a literal `{{` |

```json
{ "id": "user.msg.welcome", "txt": "Welcome {{name}} to {{%app_name%}}!" }
```

```go
lisan.T("user.msg.welcome", lisan.Args{"name": "John"}).Text
// Welcome John to MyApp!
```

Unused keys in `Args` are ignored, so one map can serve several calls. Never put `count` in `Args`; `TN` binds it.

Which placeholders an ID may use is defined by the base region. A placeholder used elsewhere must exist there. Individual plural forms may use a subset — English's `one` form legitimately omits `{{count}}`.

### Format Directives

Append `:directive` to format a value for the target locale. `lisan.Args` is a `map[string]any`, so values carry their real type.

| Directive | Go value | `en-US` | `ar` | `pl-PL` |
|---|---|---|---|---|
| `{{n:number}}` | `1234567` | `1,234,567` | `١٬٢٣٤٬٥٦٧` | `1 234 567` |
| `{{n:number:2}}` | `3.14159` | `3.14` | `٣٫١٤` | `3,14` |
| `{{r:percent}}` | `0.075` | `7.5%` | `٧٫٥٪` | `7,5%` |
| `{{v:currency}}` | `lisan.Money(1234.5, "USD")` | `$ 1,234.50` | `US$ ١٬٢٣٤٫٥٠` | `USD 1 234,50` |

```json
{ "id": "invoice", "txt": "Total {{total:currency}} ({{tax:percent}} tax on {{items:number}} items)." }
```

```go
lisan.T("invoice", lisan.Args{
	"total": lisan.Money(1234.50, "USD"),
	"tax":   0.075,
	"items": 12000,
}).Text
// Total $ 1,234.50 (7.5% tax on 12,000 items).
```

`number` and `percent` take an optional scale. Without one, `percent` keeps up to three fraction digits so `0.075` is not rounded to `7%`.

Directive names are checked at compile time; a value of the wrong type surfaces on `.Err` at runtime.

> Digits follow the locale: `lisan.Locale("ar")` gives Arabic-Indic digits, `lisan.Locale("ar-EG")` gives Latin. Use `ar-EG-u-nu-arab` to force Arabic-Indic on a specific locale.

## Setup

### Package-level

```go
if err := lisan.Configure(os.DirFS("locales")); err != nil {
	log.Fatal(err)
}
```

### Instance

```go
ui, err := lisan.New(os.DirFS("locales"))
mail, err := lisan.New(os.DirFS("mail-locales"))

ui.T("user.msg.hi")
mail.Locale("ar-EG").T("mail.receipt.subject")
```

`*I18n` has every method the package-level API has.

### Embedding

```go
//go:embed locales
var embedded embed.FS

locales, err := fs.Sub(embedded, "locales")
if err != nil {
	log.Fatal(err)
}

if err := lisan.Configure(locales); err != nil {
	log.Fatal(err)
}
```

### Options

```go
i18n, err := lisan.New(
	os.DirFS("locales"),
	lisan.WithDefaultLocale("ar-EG"),
	lisan.WithOnMissing(func(e lisan.MissingEvent) {
		metrics.Inc("i18n.missing", e.ID, e.Locale)
	}),
)
```

| Option | Effect |
|---|---|
| `WithDefaultLocale(code)` | Locale used when the context carries none. Defaults to `settings.base_region`. |
| `WithOnMissing(fn)` | Hook fired on an unknown ID or a bad `Args` value. |

## Choosing a Locale

### From a context

```go
ctx := lisan.WithLocale(ctx, "ar-EG")

lisan.From(ctx).T("user.msg.hi").Text  // مرحبًا
lisan.FromContext(ctx)                 // "ar-EG"
```

### From a code

```go
loc := lisan.Locale(user.PreferredLocale)
loc.T("mail.receipt.subject").Text
```

`Locale` accepts any locale listed under `regions[*].locales`, or a region code directly. An unknown code falls back to the default.

### HTTP middleware

Negotiates `Accept-Language` against your configured locales and stores the result in the request context.

```go
http.ListenAndServe(":8080", lisan.Middleware(mux))
```

```go
func dashboard(w http.ResponseWriter, r *http.Request) {
	loc := lisan.From(r.Context())

	fmt.Fprintln(w, loc.T("user.msg.hi").Text)
	fmt.Fprintln(w, loc.TN("user.session.active",
		lisan.N(count), lisan.Args{"name": name},
	).Text)
}
```

`Accept-Language: ar-MA,ar;q=0.9` matches the `ar` region even though `ar-MA` has no folder, because `config.json` lists it. To let a stored preference win, set the locale before the middleware runs:

```go
r = r.WithContext(lisan.WithLocale(r.Context(), user.Locale))
```

Use `lisan.Negotiate(header)` to resolve a locale without `net/http`.

## Translation Methods

```go
type Translation struct {
	Text string
	Err  error
}

type Args map[string]any
```

All four exist on the package, on `*I18n`, and on `*Localizer`.

### T — non-plural

```go
lisan.T("user.msg.hi").Text
// Hello

lisan.T("user.msg.welcome", lisan.Args{"name": "John"}).Text
// Welcome John to MyApp!
```

### TN — plural

```go
lisan.TN("user.label", lisan.N(1)).Text   // user
lisan.TN("user.label", lisan.N(5)).Text   // users

ar := lisan.Locale("ar")
ar.TN("user.label", lisan.N(1)).Text      // مستخدم    (one)
ar.TN("user.label", lisan.N(2)).Text      // مستخدمان  (two)
ar.TN("user.label", lisan.N(5)).Text      // مستخدمين  (few)
ar.TN("user.label", lisan.N(25)).Text     // مستخدمًا   (many)

lisan.Locale("pl-PL").TN("user.session.active",
	lisan.N(3), lisan.Args{"name": "Jan"}).Text
// Jan, masz 3 aktywne sesje.
```

### TryT / TryTN — error returns

```go
subject, err := loc.TryT("mail.receipt.subject", lisan.Args{"order": ref})
if err != nil {
	return fmt.Errorf("render subject: %w", err)
}
```

| | Returns | On an unknown ID |
|---|---|---|
| `T` / `TN` | `Translation` | `.Text` is the ID verbatim, `.Err` is set |
| `TryT` / `TryTN` | `(string, error)` | `("missing_key", err)` |

## Region Information

```go
lisan.Regions()               // [ar en-US pl-PL]
lisan.GetRegionName("ar")     // عربي
lisan.GetRegionLocales("ar")  // [ar-EG ar-SA ar-MA]
lisan.DefaultLocale()         // en-US
```

Language picker:

```go
for _, code := range lisan.Regions() {
	fmt.Printf("<option value=%q>%s</option>\n", code, lisan.GetRegionName(code))
}
```

## Errors

### Compile time

```go
if err := lisan.Configure(os.DirFS("locales")); err != nil {
	var ce *lisan.CompileError
	if errors.As(err, &ce) {
		for _, p := range ce.Problems {
			fmt.Printf("%s:%d:%d: %s\n", p.File, p.Line, p.Column, p.Message)
		}
	}
	log.Fatal(err)
}
```

Every problem is reported, not just the first:

```
lisan: compile error: 3 problems in the translation tree

  locales/ar/مستخدمين.json:18:3: id "user.label" declares categories [one many]
    but ar requires exactly [zero one two few many other]; missing: zero, two, few, other
  locales/pl-PL/users.json:7:3: id "user.msg.welcome" uses placeholder {{username}},
    which is not defined for this id in base region "en-US"; did you mean {{name}}?
  locales/en-GB: id "user.session.active" is declared in base region "en-US" but missing here
```

### Runtime

After a successful compile the only failures left are a typo'd ID or an `Args` value whose type doesn't match its directive. Both produce a `*MissingTranslationError` and fire `WithOnMissing`.

```go
t := lisan.T("user.msg.hi")
if t.Err != nil {
	log.Warn("i18n", "err", t.Err)
}
fmt.Fprint(w, t.Text) // renders either way
```

## Compile-Time Guarantees

Checked across every region before `Configure`/`New` returns:

1. Every `id` is unique within a region.
2. A plural entry declares exactly the CLDR categories for its language.
3. A non-plural entry has `txt` and no category keys.
4. Every region declares the same IDs as `base_region`.
5. An ID is the same kind — plural or non-plural — in every region.
6. Every placeholder exists in the base region's entry for that ID.
7. Every `{{%global%}}` resolves, and every `${VAR}` is set in the environment.
8. Every `{{key:directive}}` names a known directive.

## Recipes

### html/template

```go
loc := lisan.From(r.Context())

tmpl := template.New(name).Funcs(template.FuncMap{
	"t": func(id string, kv ...any) string {
		return loc.T(id, pairs(kv...)).Text
	},
	"tn": func(id string, n any, kv ...any) string {
		return loc.TN(id, lisan.N(n), pairs(kv...)).Text
	},
})

// pairs turns "name", value, ... into lisan.Args.
func pairs(kv ...any) lisan.Args {
	a := make(lisan.Args, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		a[kv[i].(string)] = kv[i+1]
	}
	return a
}
```

```html
<h1>{{ t "user.msg.hi" }}</h1>
<p>{{ tn "user.session.active" .Count "name" .User.Name }}</p>
```

### Testing

Compilation takes an `fs.FS`, so tests need no fixtures on disk:

```go
fsys := fstest.MapFS{
	"config.json": &fstest.MapFile{Data: []byte(`{
  "settings": { "base_region": "pl-PL" },
  "regions":  { "pl-PL": { "name": "Polski", "locales": ["pl-PL"] } }
}`)},
	"pl-PL/app.json": &fstest.MapFile{Data: []byte(`[
  {
    "id": "items",
    "one":   "{{count}} element",
    "few":   "{{count}} elementy",
    "many":  "{{count}} elementów",
    "other": "{{count}} elementu"
  }
]`)},
}

i18n, err := lisan.New(fsys)
```

Assert your real tree compiles — one line that catches every broken translation in CI:

```go
func TestLocalesCompile(t *testing.T) {
	if _, err := lisan.New(os.DirFS("../locales")); err != nil {
		t.Fatal(err)
	}
}
```

### Adding a language

1. Create the region folder, named with a valid BCP 47 tag: `locales/ru/`.
2. Add it to `regions` in `config.json`.
3. Run the test above — the compile error lists every ID you owe and the exact categories Russian requires.
4. Fill them in until it compiles.

## Not in v1

- **Date and time formatting.** `golang.org/x/text` has no CLDR date formatter. Format dates in Go and pass the result as a string.
- **Gender and select contexts.** Plural categories only.
- **Message extraction** from Go source.
- **Comments in translation files.** JSON has none.

## License

See [LICENSE](LICENSE).
