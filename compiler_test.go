package lisan

import (
	"strings"
	"testing"
)

func TestCompileAcceptsValidTree(t *testing.T) {
	t.Parallel()

	instance := mustSample(t)

	if got := instance.DefaultLocale(); got != "en-US" {
		t.Errorf("DefaultLocale() = %q, want %q", got, "en-US")
	}

	if got, want := len(instance.Regions()), 3; got != want {
		t.Errorf("Regions() has %d entries, want %d", got, want)
	}
}

// minimalConfig builds a one-region configuration around a base region.
func minimalConfig(region string) string {
	return `{
      "settings": { "base_region": "` + region + `" },
      "regions": { "` + region + `": { "name": "N", "locales": ["` + region + `"] } }
    }`
}

func TestCompileRejectsBadTrees(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name:  "missing config",
			files: map[string]string{"en-US/a.json": `[]`},
			want:  "config.json: is missing or unreadable",
		},
		{
			name: "config is not valid json",
			files: map[string]string{
				"config.json": `{ "settings": `,
			},
			want: "is not valid",
		},
		{
			name: "unknown key in config",
			files: map[string]string{
				"config.json": `{"settings":{"base_region":"en-US"},"regionz":{}}`,
			},
			want: "unknown field",
		},
		{
			name: "base region not declared",
			files: map[string]string{
				"config.json":  `{"settings":{"base_region":"fr"},"regions":{"en-US":{"name":"N","locales":["en-US"]}}}`,
				"en-US/a.json": `[]`,
			},
			want: `sets settings.base_region to "fr", which is not declared in regions`,
		},
		{
			name: "region folder name is not a bcp47 tag",
			files: map[string]string{
				"config.json":    minimalConfig("english"),
				"english/a.json": `[]`,
			},
			want: `region "english" is not a valid BCP 47 tag`,
		},
		{
			name: "region has no folder",
			files: map[string]string{
				"config.json": minimalConfig("en-US"),
			},
			want: `region "en-US" has no folder`,
		},
		{
			name: "folder is not declared",
			files: map[string]string{
				"config.json":  minimalConfig("en-US"),
				"en-US/a.json": `[]`,
				"de-DE/a.json": `[]`,
			},
			want: `folder "de-DE" is not declared under regions`,
		},
		{
			name: "locale claimed by two regions",
			files: map[string]string{
				"config.json": `{"settings":{"base_region":"en-US"},"regions":{
                    "en-US":{"name":"A","locales":["en-US","en-IE"]},
                    "en-GB":{"name":"B","locales":["en-GB","en-IE"]}}}`,
				"en-US/a.json": `[]`,
				"en-GB/a.json": `[]`,
			},
			want: `locale "en-IE" is claimed by both`,
		},
		{
			name: "unset environment global",
			files: map[string]string{
				"config.json": `{"settings":{"base_region":"en-US"},
                    "globals":{"base_url":"${LISAN_DEFINITELY_UNSET}"},
                    "regions":{"en-US":{"name":"N","locales":["en-US"]}}}`,
				"en-US/a.json": `[]`,
			},
			want: `global "base_url" references ${LISAN_DEFINITELY_UNSET}, which is not set`,
		},
		{
			name: "entry without id",
			files: map[string]string{
				"config.json":  minimalConfig("en-US"),
				"en-US/a.json": `[{"txt":"Hello"}]`,
			},
			want: `has an entry with no "id"`,
		},
		{
			name: "duplicate id in one region",
			files: map[string]string{
				"config.json":  minimalConfig("en-US"),
				"en-US/a.json": `[{"id":"x","txt":"A"},{"id":"x","txt":"B"}]`,
			},
			want: `is already declared in region "en-US"`,
		},
		{
			name: "entry mixes txt with plural categories",
			files: map[string]string{
				"config.json":  minimalConfig("en-US"),
				"en-US/a.json": `[{"id":"x","txt":"A","one":"B","other":"C"}]`,
			},
			want: `declares both "txt" and plural categories`,
		},
		{
			name: "entry has neither txt nor categories",
			files: map[string]string{
				"config.json":  minimalConfig("en-US"),
				"en-US/a.json": `[{"id":"x"}]`,
			},
			want: `has neither "txt" nor any plural category`,
		},
		{
			name: "unrecognised key",
			files: map[string]string{
				"config.json":  minimalConfig("en-US"),
				"en-US/a.json": `[{"id":"x","text":"A"}]`,
			},
			want: `has unrecognised key(s) "text"`,
		},
		{
			name: "english declares a category it does not have",
			files: map[string]string{
				"config.json":  minimalConfig("en-US"),
				"en-US/a.json": `[{"id":"x","one":"a","many":"b"}]`,
			},
			want: `declares categories [one many] but en-US requires exactly [one other]; missing: other; unexpected: many`,
		},
		{
			name: "arabic is missing categories",
			files: map[string]string{
				"config.json": minimalConfig("ar"),
				"ar/a.json":   `[{"id":"x","one":"a","other":"b"}]`,
			},
			want: `requires exactly [zero one two few many other]; missing: zero, two, few, many`,
		},
		{
			name: "polish declares arabic categories",
			files: map[string]string{
				"config.json": minimalConfig("pl-PL"),
				"pl-PL/a.json": `[{"id":"x","zero":"a","one":"b","two":"c",
                    "few":"d","many":"e","other":"f"}]`,
			},
			want: `unexpected: zero, two`,
		},
		{
			name: "unknown global reference",
			files: map[string]string{
				"config.json":  minimalConfig("en-US"),
				"en-US/a.json": `[{"id":"x","txt":"Hi {{%nope%}}"}]`,
			},
			want: `references global "nope", which is not defined in globals`,
		},
		{
			name: "unknown format directive",
			files: map[string]string{
				"config.json":  minimalConfig("en-US"),
				"en-US/a.json": `[{"id":"x","txt":"{{n:bogus}}"}]`,
			},
			want: `uses unknown format directive "bogus"`,
		},
		{
			name: "currency takes no scale",
			files: map[string]string{
				"config.json":  minimalConfig("en-US"),
				"en-US/a.json": `[{"id":"x","txt":"{{n:currency:2}}"}]`,
			},
			want: `does not take a scale`,
		},
		{
			name: "unterminated placeholder",
			files: map[string]string{
				"config.json":  minimalConfig("en-US"),
				"en-US/a.json": `[{"id":"x","txt":"Hi {{name"}]`,
			},
			want: "has an unterminated placeholder",
		},
		{
			name: "count outside a plural entry",
			files: map[string]string{
				"config.json":  minimalConfig("en-US"),
				"en-US/a.json": `[{"id":"x","txt":"{{count}} items"}]`,
			},
			want: "only available on plural entries",
		},
		{
			name: "id missing from a non-base region",
			files: map[string]string{
				"config.json": `{"settings":{"base_region":"en-US"},"regions":{
                    "en-US":{"name":"A","locales":["en-US"]},
                    "pl-PL":{"name":"B","locales":["pl-PL"]}}}`,
				"en-US/a.json": `[{"id":"x","txt":"A"},{"id":"y","txt":"B"}]`,
				"pl-PL/a.json": `[{"id":"x","txt":"A"}]`,
			},
			want: `id "y" is declared in base region "en-US" but missing here`,
		},
		{
			name: "id foreign to the base region",
			files: map[string]string{
				"config.json": `{"settings":{"base_region":"en-US"},"regions":{
                    "en-US":{"name":"A","locales":["en-US"]},
                    "pl-PL":{"name":"B","locales":["pl-PL"]}}}`,
				"en-US/a.json": `[{"id":"x","txt":"A"}]`,
				"pl-PL/a.json": `[{"id":"x","txt":"A"},{"id":"z","txt":"B"}]`,
			},
			want: `id "z" is not declared in base region "en-US"`,
		},
		{
			name: "id changes kind between regions",
			files: map[string]string{
				"config.json": `{"settings":{"base_region":"en-US"},"regions":{
                    "en-US":{"name":"A","locales":["en-US"]},
                    "pl-PL":{"name":"B","locales":["pl-PL"]}}}`,
				"en-US/a.json": `[{"id":"x","txt":"A"}]`,
				"pl-PL/a.json": `[{"id":"x","one":"a","few":"b","many":"c","other":"d"}]`,
			},
			want: `is a plural entry here but a non-plural entry in base region "en-US"`,
		},
		{
			name: "placeholder not defined in the base region",
			files: map[string]string{
				"config.json": `{"settings":{"base_region":"en-US"},"regions":{
                    "en-US":{"name":"A","locales":["en-US"]},
                    "pl-PL":{"name":"B","locales":["pl-PL"]}}}`,
				"en-US/a.json": `[{"id":"x","txt":"Hi {{name}}"}]`,
				"pl-PL/a.json": `[{"id":"x","txt":"Hej {{username}}"}]`,
			},
			want: `did you mean {{name}}?`,
		},
		{
			name: "translation file is not an array",
			files: map[string]string{
				"config.json":  minimalConfig("en-US"),
				"en-US/a.json": `{"id":"x"}`,
			},
			want: "must contain a JSON array",
		},
		{
			name: "translation value is not a string",
			files: map[string]string{
				"config.json":  minimalConfig("en-US"),
				"en-US/a.json": `[{"id":"x","txt":42}]`,
			},
			want: "not an object of strings",
		},
		{
			name: "region groups locales with different plural rules",
			files: map[string]string{
				"config.json": `{"settings":{"base_region":"en-US"},"regions":{
                    "en-US":{"name":"A","locales":["en-US","ar-EG"]}}}`,
				"en-US/a.json": `[]`,
			},
			want: "cannot share one translation set",
		},
		{
			name: "locale is not a bcp47 tag",
			files: map[string]string{
				"config.json": `{"settings":{"base_region":"en-US"},"regions":{
                    "en-US":{"name":"A","locales":["en-US","!!!"]}}}`,
				"en-US/a.json": `[]`,
			},
			want: `locale "!!!", which is not a valid BCP 47 tag`,
		},
		{
			name: "a plural form has a bad directive",
			files: map[string]string{
				"config.json":  minimalConfig("en-US"),
				"en-US/a.json": `[{"id":"x","one":"one","other":"{{n:bogus}}"}]`,
			},
			want: `category "other" uses unknown format directive "bogus"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			problems := compileProblems(t, test.files)

			joined := joinProblems(problems)
			if !strings.Contains(joined, test.want) {
				t.Errorf("compile problems:\n%s\nwant a problem containing:\n%s", joined, test.want)
			}
		})
	}
}

// joinProblems renders problems one per line for assertion messages.
func joinProblems(problems []Problem) string {
	lines := make([]string, 0, len(problems))
	for _, problem := range problems {
		lines = append(lines, "  "+problem.String())
	}

	return strings.Join(lines, "\n")
}

func TestCompileReportsEveryProblem(t *testing.T) {
	t.Parallel()

	problems := compileProblems(t, map[string]string{
		"config.json": minimalConfig("en-US"),
		"en-US/a.json": `[
            {"id":"a","one":"x","many":"y"},
            {"id":"b","txt":"{{%missing%}}"},
            {"id":"c"}
        ]`,
	})

	if len(problems) < 3 {
		t.Fatalf("got %d problems, want at least 3:\n%s", len(problems), joinProblems(problems))
	}
}

func TestCompileReportsLineNumbers(t *testing.T) {
	t.Parallel()

	problems := compileProblems(t, map[string]string{
		"config.json": minimalConfig("en-US"),
		"en-US/a.json": "[\n" +
			`  {"id":"ok","txt":"fine"},` + "\n" +
			`  {"id":"bad","one":"x","many":"y"}` + "\n]",
	})

	if len(problems) != 1 {
		t.Fatalf("got %d problems, want 1:\n%s", len(problems), joinProblems(problems))
	}

	if problems[0].Line != 3 {
		t.Errorf("Line = %d, want 3 (%s)", problems[0].Line, problems[0].String())
	}

	if problems[0].ID != "bad" {
		t.Errorf("ID = %q, want %q", problems[0].ID, "bad")
	}
}

// twoRegionConfig pairs en-US as base with a second region.
func twoRegionConfig(other string) string {
	return `{"settings":{"base_region":"en-US"},"regions":{
        "en-US":{"name":"A","locales":["en-US"]},
        "` + other + `":{"name":"B","locales":["` + other + `"]}}}`
}

func TestCompileDoesNotReportAnInvalidEntryAsMissing(t *testing.T) {
	t.Parallel()

	problems := compileProblems(t, map[string]string{
		"config.json":  twoRegionConfig("ar"),
		"en-US/a.json": `[{"id":"x","one":"one user","other":"{{count}} users"}]`,
		"ar/a.json":    "[\n" + `  {"id":"x","one":"مستخدم","other":"{{count}} مستخدم"}` + "\n]",
	})

	if len(problems) != 1 {
		t.Fatalf("got %d problems, want 1:\n%s", len(problems), joinProblems(problems))
	}

	if !strings.Contains(problems[0].String(), "missing: zero, two, few, many") {
		t.Errorf("problem = %s, want the incomplete category set", problems[0].String())
	}

	if strings.Contains(problems[0].String(), "missing here") {
		t.Errorf("problem = %s, want no missing-id report for a declared id", problems[0].String())
	}
}

func TestCompileDoesNotReportAnEntryForeignWhenTheBaseEntryIsInvalid(t *testing.T) {
	t.Parallel()

	problems := compileProblems(t, map[string]string{
		"config.json":  twoRegionConfig("pl-PL"),
		"en-US/a.json": `[{"id":"x","one":"a","many":"b"}]`,
		"pl-PL/a.json": `[{"id":"x","one":"a","few":"b","many":"c","other":"d"}]`,
	})

	if len(problems) != 1 {
		t.Fatalf("got %d problems, want 1:\n%s", len(problems), joinProblems(problems))
	}

	if got := joinProblems(problems); strings.Contains(got, "is not declared in base region") {
		t.Errorf("problems:\n%s\nwant no foreign-id report while the base entry is invalid", got)
	}
}

func TestCompileStillReportsDuplicatesOfAnInvalidEntry(t *testing.T) {
	t.Parallel()

	problems := compileProblems(t, map[string]string{
		"config.json":  minimalConfig("en-US"),
		"en-US/a.json": `[{"id":"x","one":"a","many":"b"},{"id":"x","txt":"A"}]`,
	})

	if got := joinProblems(problems); !strings.Contains(got, `is already declared in region "en-US"`) {
		t.Errorf("problems:\n%s\nwant a duplicate-id report", got)
	}
}

func TestCompileResolvesEnvironmentGlobals(t *testing.T) {
	t.Setenv("LISAN_TEST_URL", "https://example.test")

	instance := mustCompile(t, map[string]string{
		"config.json": `{"settings":{"base_region":"en-US"},
            "globals":{"base_url":"${LISAN_TEST_URL}"},
            "regions":{"en-US":{"name":"N","locales":["en-US"]}}}`,
		"en-US/a.json": `[{"id":"x","txt":"Visit {{%base_url%}}/help"}]`,
	})

	if got, want := instance.T("x").Text, "Visit https://example.test/help"; got != want {
		t.Errorf("T() = %q, want %q", got, want)
	}
}

func TestNewRejectsUnknownDefaultLocale(t *testing.T) {
	t.Parallel()

	_, err := New(sampleTree(), WithDefaultLocale("fr-FR"))
	if err == nil {
		t.Fatal("New() succeeded, want an error for an unserved default locale")
	}

	if !strings.Contains(err.Error(), "not served by any region") {
		t.Errorf("error = %v, want it to mention an unserved locale", err)
	}
}

func TestNewRejectsNilFS(t *testing.T) {
	t.Parallel()

	if _, err := New(nil); err == nil {
		t.Fatal("New(nil) succeeded, want an error")
	}
}
