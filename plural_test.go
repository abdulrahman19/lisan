package lisan

import (
	"math"
	"slices"
	"testing"

	"golang.org/x/text/language"
)

func TestCategoriesFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tag  string
		want []string
	}{
		{name: "english", tag: "en", want: []string{"one", "other"}},
		{name: "english US", tag: "en-US", want: []string{"one", "other"}},
		{name: "arabic", tag: "ar", want: []string{"zero", "one", "two", "few", "many", "other"}},
		{name: "polish", tag: "pl", want: []string{"one", "few", "many", "other"}},
		{name: "russian", tag: "ru", want: []string{"one", "few", "many", "other"}},
		{name: "japanese", tag: "ja", want: []string{"other"}},
		{name: "turkish", tag: "tr", want: []string{"one", "other"}},
		{name: "czech", tag: "cs", want: []string{"one", "few", "many", "other"}},
		{name: "welsh", tag: "cy", want: []string{"zero", "one", "two", "few", "many", "other"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			tag := language.MustParse(test.tag)

			got := formNames(categoriesFor(tag))
			if !slices.Equal(got, test.want) {
				t.Errorf("categoriesFor(%q) = %v, want %v", test.tag, got, test.want)
			}
		})
	}
}

func TestResolveForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		tag   string
		count any
		want  string
	}{
		{name: "en zero", tag: "en", count: 0, want: "other"},
		{name: "en one", tag: "en", count: 1, want: "one"},
		{name: "en two", tag: "en", count: 2, want: "other"},
		{name: "en hundred", tag: "en", count: 100, want: "other"},
		{name: "en one point zero", tag: "en", count: 1.0, want: "other"},
		{name: "en half", tag: "en", count: 0.5, want: "other"},

		{name: "ar zero", tag: "ar", count: 0, want: "zero"},
		{name: "ar one", tag: "ar", count: 1, want: "one"},
		{name: "ar two", tag: "ar", count: 2, want: "two"},
		{name: "ar five", tag: "ar", count: 5, want: "few"},
		{name: "ar twenty one", tag: "ar", count: 21, want: "many"},
		{name: "ar twenty two", tag: "ar", count: 22, want: "many"},
		{name: "ar hundred", tag: "ar", count: 100, want: "other"},

		{name: "pl zero", tag: "pl", count: 0, want: "many"},
		{name: "pl one", tag: "pl", count: 1, want: "one"},
		{name: "pl two", tag: "pl", count: 2, want: "few"},
		{name: "pl five", tag: "pl", count: 5, want: "many"},
		{name: "pl twenty two", tag: "pl", count: 22, want: "few"},

		{name: "ru twenty one", tag: "ru", count: 21, want: "one"},
		{name: "ru twenty two", tag: "ru", count: 22, want: "few"},
		{name: "ru five", tag: "ru", count: 5, want: "many"},

		{name: "ja anything", tag: "ja", count: 7, want: "other"},

		{name: "negative counts by magnitude", tag: "en", count: -1, want: "one"},
		{name: "unsigned", tag: "en", count: uint8(1), want: "one"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			tag := language.MustParse(test.tag)

			got := formName(resolveForm(tag, N(test.count)))
			if got != test.want {
				t.Errorf("resolveForm(%q, %v) = %q, want %q", test.tag, test.count, got, test.want)
			}
		})
	}
}

func TestNOperands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		value          any
		wantDigits     int
		wantVisible    int
		wantFrac       int
		wantFracTrimmd int
		wantOK         bool
	}{
		{name: "int", value: 12, wantDigits: 12, wantOK: true},
		{name: "negative int", value: -12, wantDigits: 12, wantOK: true},
		{name: "int64", value: int64(7), wantDigits: 7, wantOK: true},
		{name: "uint", value: uint(3), wantDigits: 3, wantOK: true},
		{name: "float whole", value: 1.0, wantDigits: 1, wantVisible: 1, wantOK: true},
		{name: "float half", value: 1.5, wantDigits: 1, wantVisible: 1, wantFrac: 5, wantFracTrimmd: 5, wantOK: true},
		{name: "float trailing zero", value: 1.50, wantDigits: 1, wantVisible: 1, wantFrac: 5, wantFracTrimmd: 5, wantOK: true},
		{name: "float many digits", value: 3.14159, wantDigits: 3, wantVisible: 5, wantFrac: 14159, wantFracTrimmd: 14159, wantOK: true},
		{name: "string is not a number", value: "nope", wantOK: false},
		{name: "nil is not a number", value: nil, wantOK: false},
		{name: "NaN is not usable", value: math.NaN(), wantOK: false},
		{name: "infinity is not usable", value: math.Inf(1), wantOK: false},
		{name: "huge int clamps", value: int64(math.MaxInt64), wantDigits: math.MaxInt32, wantOK: true},
		{name: "most negative int clamps", value: int64(math.MinInt64), wantDigits: math.MaxInt32, wantOK: true},
		{name: "huge uint clamps", value: uint64(math.MaxUint64), wantDigits: math.MaxInt32, wantOK: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := N(test.value)

			if got.ok != test.wantOK {
				t.Fatalf("N(%v).ok = %v, want %v", test.value, got.ok, test.wantOK)
			}

			if got.digits != test.wantDigits {
				t.Errorf("digits = %d, want %d", got.digits, test.wantDigits)
			}

			if got.visible != test.wantVisible {
				t.Errorf("visible = %d, want %d", got.visible, test.wantVisible)
			}

			if got.frac != test.wantFrac {
				t.Errorf("frac = %d, want %d", got.frac, test.wantFrac)
			}

			if got.fracTrimmed != test.wantFracTrimmd {
				t.Errorf("fracTrimmed = %d, want %d", got.fracTrimmed, test.wantFracTrimmd)
			}
		})
	}
}

func TestNIsIdempotent(t *testing.T) {
	t.Parallel()

	once := N(5)
	if twice := N(once); twice != once {
		t.Errorf("N(N(5)) = %+v, want %+v", twice, once)
	}
}
