package lisan

import (
	"maps"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// missingKeyText is what TryT and TryTN return alongside their error.
const missingKeyText = "missing_key"

// Translation is the result of a lookup. Text always holds something
// renderable; Err is set when that something is a fallback.
type Translation struct {
	// Err is non-nil when the lookup failed.
	Err error
	// Text is the rendered translation, or the requested id when it failed.
	Text string
}

// Localizer renders translations for exactly one locale. It is immutable, so
// concurrent requests in different languages cannot interfere.
type Localizer struct {
	owner   *I18n
	region  *regionData
	printer *message.Printer
	code    string
	tag     language.Tag
}

// T renders a non-plural translation. On failure Text holds the identifier and
// Err explains what went wrong, so display paths can render regardless.
func (l *Localizer) T(id string, args ...Args) Translation {
	text, err := l.resolveText(id, mergeArgs(args))
	if err != nil {
		return Translation{Text: id, Err: err}
	}

	return Translation{Text: text}
}

// TN renders a plural translation, choosing the CLDR category from count and
// binding {{count}} to it.
func (l *Localizer) TN(id string, count Number, args ...Args) Translation {
	text, err := l.resolvePlural(id, count, mergeArgs(args))
	if err != nil {
		return Translation{Text: id, Err: err}
	}

	return Translation{Text: text}
}

// TryT renders a non-plural translation, returning an error instead of a
// fallback string.
func (l *Localizer) TryT(id string, args ...Args) (string, error) {
	text, err := l.resolveText(id, mergeArgs(args))
	if err != nil {
		return missingKeyText, err
	}

	return text, nil
}

// TryTN renders a plural translation, returning an error instead of a fallback
// string.
func (l *Localizer) TryTN(id string, count Number, args ...Args) (string, error) {
	text, err := l.resolvePlural(id, count, mergeArgs(args))
	if err != nil {
		return missingKeyText, err
	}

	return text, nil
}

// Code reports the locale this Localizer is bound to.
func (l *Localizer) Code() string {
	if l == nil {
		return ""
	}

	return l.code
}

// Tag reports the BCP 47 tag this Localizer formats numbers with.
func (l *Localizer) Tag() language.Tag {
	if l == nil {
		return language.Und
	}

	return l.tag
}

// resolveText looks up and renders a non-plural entry.
func (l *Localizer) resolveText(id string, args Args) (string, error) {
	found, err := l.entry(id)
	if err != nil {
		return "", err
	}

	if found.plural {
		return "", l.missf(id, "is a plural entry; use TN instead of T")
	}

	text, renderErr := found.text.render(renderContext{printer: l.printer, args: args})
	if renderErr != nil {
		return "", l.missf(id, "%v", renderErr)
	}

	return text, nil
}

// resolvePlural looks up a plural entry, picks its CLDR category, and renders.
func (l *Localizer) resolvePlural(id string, count Number, args Args) (string, error) {
	found, err := l.entry(id)
	if err != nil {
		return "", err
	}

	if !found.plural {
		return "", l.missf(id, "is not a plural entry; use T instead of TN")
	}

	if !count.ok {
		return "", l.missf(id, "was given a count that is not a number: %T", count.value)
	}

	// The region tag drives category selection because the compiler validated
	// the declared categories against it.
	form := resolveForm(l.region.tag, count)

	chosen, declared := found.forms[form]
	if !declared {
		return "", l.missf(id, "has no %q form", formName(form))
	}

	text, renderErr := chosen.render(renderContext{
		printer:  l.printer,
		args:     args,
		count:    count,
		hasCount: true,
	})
	if renderErr != nil {
		return "", l.missf(id, "%v", renderErr)
	}

	return text, nil
}

// entry resolves an identifier within this locale's region.
func (l *Localizer) entry(id string) (*entry, error) {
	if l == nil || l.region == nil {
		return nil, ErrNotConfigured
	}

	found, known := l.region.entries[id]
	if !known {
		return nil, l.missf(id, "unknown translation id")
	}

	return found, nil
}

// missf reports a failed lookup to the hook and returns the error.
func (l *Localizer) missf(id, format string, args ...any) error {
	var hook func(MissingEvent)
	if l.owner != nil {
		hook = l.owner.onMissing
	}

	return missingf(hook, id, l.code, format, args...)
}

// mergeArgs flattens the optional variadic Args into one map.
func mergeArgs(args []Args) Args {
	switch len(args) {
	case 0:
		return nil
	case 1:
		return args[0]
	}

	merged := make(Args)
	for _, one := range args {
		maps.Copy(merged, one)
	}

	return merged
}
