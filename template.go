package lisan

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// Placeholder syntax. A doubled opening delimiter escapes to a literal one.
const (
	openDelim        = "{{"
	closeDelim       = "}}"
	escapedOpen      = "{{{{"
	globalMarker     = "%"
	directiveSep     = ":"
	countPlaceholder = "count"
	maxScale         = 20
)

// segmentKind distinguishes the three things a compiled phrase is made of.
type segmentKind uint8

const (
	segmentLiteral segmentKind = iota
	segmentArg
	segmentCount
)

// directiveKind names a locale-aware formatting directive.
type directiveKind uint8

const (
	directiveNone directiveKind = iota
	directiveNumber
	directivePercent
	directiveCurrency
)

// directive is the optional `:name` or `:name:scale` suffix of a placeholder.
type directive struct {
	kind     directiveKind
	scale    int
	hasScale bool
}

// segment is one piece of a compiled phrase.
type segment struct {
	text      string
	directive directive
	kind      segmentKind
}

// phrase is a translation string with its placeholders already parsed and its
// global references already substituted.
type phrase struct {
	segments  []segment
	args      []string
	usesCount bool
	width     int
}

// parsePhrase compiles a raw translation string, resolving global references
// against globals. The returned error describes the first defect found.
func parsePhrase(raw string, globals map[string]string) (phrase, error) {
	parser := &phraseParser{globals: globals, argSet: make(map[string]struct{})}

	if err := parser.run(raw); err != nil {
		return phrase{}, err
	}

	return parser.finish(), nil
}

// phraseParser accumulates segments while scanning a translation string.
type phraseParser struct {
	globals map[string]string
	argSet  map[string]struct{}
	built   phrase
	literal []byte
}

// run scans raw and fills the parser's segment list.
func (p *phraseParser) run(raw string) error {
	for index := 0; index < len(raw); {
		rest := raw[index:]

		switch {
		case strings.HasPrefix(rest, escapedOpen):
			p.literal = append(p.literal, openDelim...)
			index += len(escapedOpen)
		case strings.HasPrefix(rest, openDelim):
			width, err := p.placeholder(rest)
			if err != nil {
				return err
			}

			index += width
		default:
			p.literal = append(p.literal, raw[index])
			index++
		}
	}

	return nil
}

// placeholder consumes one {{...}} span and reports how many bytes it spanned.
func (p *phraseParser) placeholder(rest string) (int, error) {
	closeAt := strings.Index(rest, closeDelim)
	if closeAt < 0 {
		return 0, fmt.Errorf("has an unterminated placeholder %q", rest)
	}

	if err := p.emit(rest[len(openDelim):closeAt]); err != nil {
		return 0, err
	}

	return closeAt + len(closeDelim), nil
}

// emit turns one placeholder body into a segment, or into literal text when it
// is a global reference.
func (p *phraseParser) emit(body string) error {
	if isGlobalRef(body) {
		return p.emitGlobal(body)
	}

	return p.emitArg(body)
}

// emitGlobal substitutes a {{%name%}} reference at compile time.
func (p *phraseParser) emitGlobal(body string) error {
	name := body[len(globalMarker) : len(body)-len(globalMarker)]

	value, defined := p.globals[name]
	if !defined {
		return fmt.Errorf("references global %q, which is not defined in globals", name)
	}

	p.literal = append(p.literal, value...)

	return nil
}

// emitArg records a runtime placeholder, with its optional format directive.
func (p *phraseParser) emitArg(body string) error {
	name, spec, _ := strings.Cut(body, directiveSep)

	if !validPlaceholderName(name) {
		return fmt.Errorf("has a malformed placeholder %s%s%s", openDelim, body, closeDelim)
	}

	parsed, err := parseDirective(spec)
	if err != nil {
		return err
	}

	p.flushLiteral()

	if name == countPlaceholder {
		p.built.usesCount = true
		p.built.segments = append(p.built.segments, segment{kind: segmentCount, text: name, directive: parsed})

		return nil
	}

	p.argSet[name] = struct{}{}
	p.built.segments = append(p.built.segments, segment{kind: segmentArg, text: name, directive: parsed})

	return nil
}

// flushLiteral closes off any pending literal run.
func (p *phraseParser) flushLiteral() {
	if len(p.literal) == 0 {
		return
	}

	p.built.segments = append(p.built.segments, segment{kind: segmentLiteral, text: string(p.literal)})
	p.literal = p.literal[:0]
}

// finish closes the parser and returns the compiled phrase.
func (p *phraseParser) finish() phrase {
	p.flushLiteral()

	p.built.args = slices.Sorted(maps.Keys(p.argSet))
	for _, seg := range p.built.segments {
		p.built.width += len(seg.text)
	}

	return p.built
}

// isGlobalRef reports whether a placeholder body is a {{%name%}} reference.
func isGlobalRef(body string) bool {
	return len(body) > 2*len(globalMarker) &&
		strings.HasPrefix(body, globalMarker) &&
		strings.HasSuffix(body, globalMarker)
}

// validPlaceholderName reports whether name is usable as a placeholder key.
func validPlaceholderName(name string) bool {
	if name == "" {
		return false
	}

	for index, char := range name {
		switch {
		case unicode.IsLetter(char) || char == '_':
		case index > 0 && unicode.IsDigit(char):
		default:
			return false
		}
	}

	return true
}

// parseDirective parses the part of a placeholder body after the first colon.
func parseDirective(spec string) (directive, error) {
	if spec == "" {
		return directive{}, nil
	}

	name, scaleText, scaled := strings.Cut(spec, directiveSep)

	kind, known := directiveByName(name)
	if !known {
		return directive{}, fmt.Errorf("uses unknown format directive %q", name)
	}

	if !scaled {
		return directive{kind: kind}, nil
	}

	if kind == directiveCurrency {
		return directive{}, fmt.Errorf("format directive %q does not take a scale", name)
	}

	scale, err := strconv.Atoi(scaleText)
	if err != nil || scale < 0 || scale > maxScale {
		return directive{}, fmt.Errorf("format directive %q has an invalid scale %q", name, scaleText)
	}

	return directive{kind: kind, scale: scale, hasScale: true}, nil
}

// directiveByName maps a directive name to its kind.
func directiveByName(name string) (directiveKind, bool) {
	switch name {
	case "number":
		return directiveNumber, true
	case "percent":
		return directivePercent, true
	case "currency":
		return directiveCurrency, true
	default:
		return directiveNone, false
	}
}
