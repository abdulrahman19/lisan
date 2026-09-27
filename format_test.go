package lisan

import (
	"strings"
	"testing"
)

// formatTree is a two-region tree exercising every format directive.
func formatTree() map[string]string {
	return map[string]string{
		"config.json": `{
          "settings": { "base_region": "en-US" },
          "regions": {
            "en-US": { "name": "English", "locales": ["en-US"] },
            "pl-PL": { "name": "Polski",  "locales": ["pl-PL"] }
          }
        }`,
		"en-US/a.json": `[
          { "id": "plain",    "txt": "{{v}}" },
          { "id": "number",   "txt": "{{v:number}}" },
          { "id": "scaled",   "txt": "{{v:number:2}}" },
          { "id": "percent",  "txt": "{{v:percent}}" },
          { "id": "currency", "txt": "{{v:currency}}" },
          { "id": "braces",   "txt": "{{{{v}} stays" }
        ]`,
		"pl-PL/a.json": `[
          { "id": "plain",    "txt": "{{v}}" },
          { "id": "number",   "txt": "{{v:number}}" },
          { "id": "scaled",   "txt": "{{v:number:2}}" },
          { "id": "percent",  "txt": "{{v:percent}}" },
          { "id": "currency", "txt": "{{v:currency}}" },
          { "id": "braces",   "txt": "{{{{v}} zostaje" }
        ]`,
	}
}

func TestFormatDirectives(t *testing.T) {
	t.Parallel()

	instance := mustCompile(t, formatTree())

	tests := []struct {
		name   string
		locale string
		id     string
		value  any
		want   string
	}{
		{name: "plain string", locale: "en-US", id: "plain", value: "raw", want: "raw"},
		{name: "number en", locale: "en-US", id: "number", value: 1234567, want: "1,234,567"},
		{name: "number pl", locale: "pl-PL", id: "number", value: 1234567, want: "1\u00a0234\u00a0567"},
		{name: "scaled en", locale: "en-US", id: "scaled", value: 3.14159, want: "3.14"},
		{name: "scaled pl", locale: "pl-PL", id: "scaled", value: 3.14159, want: "3,14"},
		{name: "percent en", locale: "en-US", id: "percent", value: 0.075, want: "7.5%"},
		{name: "percent pl", locale: "pl-PL", id: "percent", value: 0.075, want: "7,5%"},
		{name: "percent whole", locale: "en-US", id: "percent", value: 0.12, want: "12%"},
		{
			name: "currency en", locale: "en-US", id: "currency",
			value: Money(1234.50, "USD"), want: "$ 1,234.50",
		},
		{
			name: "currency pl", locale: "pl-PL", id: "currency",
			value: Money(1234.50, "USD"), want: "USD 1\u00a0234,50",
		},
		{name: "escaped braces", locale: "en-US", id: "braces", value: "ignored", want: "{{v}} stays"},
		{name: "non-string value without a directive", locale: "en-US", id: "plain", value: 42, want: "42"},
		{name: "bool value without a directive", locale: "en-US", id: "plain", value: true, want: "true"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := instance.Locale(test.locale).T(test.id, Args{"v": test.value})
			if got.Err != nil {
				t.Fatalf("T(%q) returned error: %v", test.id, got.Err)
			}

			if got.Text != test.want {
				t.Errorf("T(%q) in %s = %q, want %q", test.id, test.locale, got.Text, test.want)
			}
		})
	}
}

func TestFormatDirectiveTypeErrors(t *testing.T) {
	t.Parallel()

	instance := mustCompile(t, formatTree())

	tests := []struct {
		name    string
		id      string
		value   any
		wantErr string
	}{
		{name: "string to number", id: "number", value: "nope", wantErr: "expects a number for :number"},
		{name: "string to percent", id: "percent", value: "nope", wantErr: "expects a number for :percent"},
		{name: "string to currency", id: "currency", value: "nope", wantErr: "expects lisan.Money for :currency"},
		{
			name: "unknown currency code", id: "currency",
			value: Money(1, "ZZZ"), wantErr: "unknown ISO 4217 currency code",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := instance.T(test.id, Args{"v": test.value})
			if got.Err == nil {
				t.Fatalf("T(%q) succeeded with %q, want an error", test.id, got.Text)
			}

			if !strings.Contains(got.Err.Error(), test.wantErr) {
				t.Errorf("error = %v, want it to contain %q", got.Err, test.wantErr)
			}
		})
	}
}

func TestCountFormatting(t *testing.T) {
	t.Parallel()

	instance := mustCompile(t, map[string]string{
		"config.json": `{
          "settings": { "base_region": "en-US" },
          "regions": { "en-US": { "name": "English", "locales": ["en-US"] } }
        }`,
		"en-US/a.json": `[
          { "id": "cart", "one": "One item.", "other": "{{count}} items." }
        ]`,
	})

	tests := []struct {
		name  string
		count any
		want  string
	}{
		{name: "grouped thousands", count: 1200, want: "1,200 items."},
		{name: "singular", count: 1, want: "One item."},
		{name: "float keeps its fraction", count: 1.0, want: "1.0 items."},
		{name: "float half", count: 0.5, want: "0.5 items."},
		{name: "millions", count: 2500000, want: "2,500,000 items."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := instance.TN("cart", N(test.count)).Text; got != test.want {
				t.Errorf("TN(cart, %v) = %q, want %q", test.count, got, test.want)
			}
		})
	}
}

func TestCountHonoursItsDirective(t *testing.T) {
	t.Parallel()

	instance := mustCompile(t, map[string]string{
		"config.json": `{
          "settings": { "base_region": "en-US" },
          "regions": { "en-US": { "name": "English", "locales": ["en-US"] } }
        }`,
		"en-US/a.json": `[
          { "id": "cart", "one": "One item.", "other": "{{count}} items." },
          { "id": "scaled", "one": "One.", "other": "{{count:number:2}} items." }
        ]`,
	})

	if got, want := instance.TN("scaled", N(1200)).Text, "1,200.00 items."; got != want {
		t.Errorf("TN(scaled, 1200) = %q, want %q", got, want)
	}
}
