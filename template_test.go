package lisan

import (
	"strings"
	"testing"
)

func TestParsePhrase(t *testing.T) {
	t.Parallel()

	globals := map[string]string{"app": "Acme", "url": "https://acme.test"}

	tests := []struct {
		name      string
		raw       string
		wantArgs  []string
		wantCount bool
		wantText  string
	}{
		{name: "literal only", raw: "Hello", wantText: "Hello"},
		{name: "single arg", raw: "Hi {{name}}", wantArgs: []string{"name"}, wantText: "Hi "},
		{
			name: "two args", raw: "{{greeting}} {{name}}!",
			wantArgs: []string{"greeting", "name"},
		},
		{
			name: "repeated arg counts once", raw: "{{name}} and {{name}}",
			wantArgs: []string{"name"},
		},
		{name: "global is inlined", raw: "Welcome to {{%app%}}", wantText: "Welcome to Acme"},
		{name: "two globals", raw: "{{%app%}} at {{%url%}}", wantText: "Acme at https://acme.test"},
		{name: "count", raw: "{{count}} items", wantCount: true},
		{
			name: "directives do not change the arg name", raw: "{{total:currency}}",
			wantArgs: []string{"total"},
		},
		{name: "escaped braces", raw: "{{{{literal}}", wantText: "{{literal}}"},
		{name: "unmatched closing braces are literal", raw: "a}}b", wantText: "a}}b"},
		{name: "empty string", raw: "", wantText: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := parsePhrase(test.raw, globals)
			if err != nil {
				t.Fatalf("parsePhrase(%q) failed: %v", test.raw, err)
			}

			if test.wantArgs != nil && !equalStrings(got.args, test.wantArgs) {
				t.Errorf("args = %v, want %v", got.args, test.wantArgs)
			}

			if got.usesCount != test.wantCount {
				t.Errorf("usesCount = %v, want %v", got.usesCount, test.wantCount)
			}

			if test.wantText != "" && literalOf(got) != test.wantText {
				t.Errorf("literal text = %q, want %q", literalOf(got), test.wantText)
			}
		})
	}
}

// literalOf concatenates a phrase's literal segments.
func literalOf(parsed phrase) string {
	var builder strings.Builder

	for _, seg := range parsed.segments {
		if seg.kind == segmentLiteral {
			builder.WriteString(seg.text)
		}
	}

	return builder.String()
}

func TestParsePhraseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "unterminated", raw: "Hi {{name", wantErr: "unterminated placeholder"},
		{name: "empty name", raw: "{{}}", wantErr: "malformed placeholder"},
		{name: "name with a space", raw: "{{first name}}", wantErr: "malformed placeholder"},
		{name: "leading digit", raw: "{{1name}}", wantErr: "malformed placeholder"},
		{name: "unknown global", raw: "{{%nope%}}", wantErr: `global "nope"`},
		{name: "unknown directive", raw: "{{n:bogus}}", wantErr: `unknown format directive "bogus"`},
		{name: "scale is not a number", raw: "{{n:number:x}}", wantErr: "invalid scale"},
		{name: "scale is negative", raw: "{{n:number:-1}}", wantErr: "invalid scale"},
		{name: "scale is too large", raw: "{{n:number:99}}", wantErr: "invalid scale"},
		{name: "currency with a scale", raw: "{{n:currency:2}}", wantErr: "does not take a scale"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := parsePhrase(test.raw, map[string]string{"app": "Acme"})
			if err == nil {
				t.Fatalf("parsePhrase(%q) succeeded, want an error", test.raw)
			}

			if !strings.Contains(err.Error(), test.wantErr) {
				t.Errorf("error = %v, want it to contain %q", err, test.wantErr)
			}
		})
	}
}

func TestValidPlaceholderName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "simple", input: "name", want: true},
		{name: "underscore prefix", input: "_name", want: true},
		{name: "digits after a letter", input: "arg1", want: true},
		{name: "non-latin letters", input: "اسم", want: true},
		{name: "empty", input: "", want: false},
		{name: "leading digit", input: "1arg", want: false},
		{name: "space", input: "a b", want: false},
		{name: "dash", input: "a-b", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := validPlaceholderName(test.input); got != test.want {
				t.Errorf("validPlaceholderName(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestEditDistance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		from string
		to   string
		want int
	}{
		{from: "", to: "", want: 0},
		{from: "name", to: "name", want: 0},
		{from: "name", to: "nam", want: 1},
		{from: "username", to: "name", want: 4},
		{from: "kitten", to: "sitting", want: 3},
		{from: "abc", to: "", want: 3},
	}

	for _, test := range tests {
		t.Run(test.from+"/"+test.to, func(t *testing.T) {
			t.Parallel()

			if got := editDistance(test.from, test.to); got != test.want {
				t.Errorf("editDistance(%q, %q) = %d, want %d", test.from, test.to, got, test.want)
			}
		})
	}
}

func TestClosestName(t *testing.T) {
	t.Parallel()

	candidates := []string{"name", "email", "count"}

	tests := []struct {
		input     string
		want      string
		wantFound bool
	}{
		{input: "username", want: "name", wantFound: true},
		{input: "nam", want: "name", wantFound: true},
		{input: "emial", want: "email", wantFound: true},
		{input: "zzzzzzzzzzzz", wantFound: false},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()

			got, found := closestName(test.input, candidates)
			if found != test.wantFound {
				t.Fatalf("closestName(%q) found = %v, want %v", test.input, found, test.wantFound)
			}

			if found && got != test.want {
				t.Errorf("closestName(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestSourcePosition(t *testing.T) {
	t.Parallel()

	file := newSourceFile("x.json", []byte("[\n  {\"id\":\"a\"},\n  {\"id\":\"b\"}\n]"))

	tests := []struct {
		name       string
		offset     int
		wantLine   int
		wantColumn int
	}{
		{name: "start", offset: 0, wantLine: 1, wantColumn: 1},
		{name: "second line skips indent", offset: 2, wantLine: 2, wantColumn: 3},
		{name: "third line", offset: 16, wantLine: 3, wantColumn: 3},
		{name: "negative clamps to start", offset: -5, wantLine: 1, wantColumn: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := file.position(test.offset)
			if got.line != test.wantLine || got.column != test.wantColumn {
				t.Errorf("position(%d) = (%d, %d), want (%d, %d)",
					test.offset, got.line, got.column, test.wantLine, test.wantColumn)
			}
		})
	}
}
