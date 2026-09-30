package lisan

import (
	"maps"
	"slices"

	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
)

// entry is one compiled translation identifier within a region. It is either a
// non-plural entry carrying text, or a plural entry carrying one phrase per
// CLDR category, never both.
type entry struct {
	forms  map[plural.Form]phrase
	id     string
	file   string
	text   phrase
	pos    textPosition
	plural bool
}

// argNames returns every runtime placeholder this entry can use, across all of
// its plural forms, in sorted order.
func (e *entry) argNames() []string {
	if !e.plural {
		return e.text.args
	}

	union := make(map[string]struct{})

	for _, form := range e.forms {
		for _, name := range form.args {
			union[name] = struct{}{}
		}
	}

	return slices.Sorted(maps.Keys(union))
}

// regionData is one compiled region: the translation set shared by a group of
// locales, together with the plural categories its language requires.
type regionData struct {
	entries map[string]*entry
	// rejected holds identifiers that were declared but failed validation, so
	// later checks do not also report them as missing.
	rejected map[string]*entry
	code     string
	name     string
	locales  []string
	forms    []plural.Form
	tag      language.Tag
}

// declaredEntry returns the entry declared under id, valid or not.
func (r *regionData) declaredEntry(id string) (*entry, bool) {
	if existing, present := r.entries[id]; present {
		return existing, true
	}

	existing, present := r.rejected[id]

	return existing, present
}

// localeBinding ties a requested locale to the region that serves it. Plural
// categories follow the region, number formatting follows the locale.
type localeBinding struct {
	region *regionData
	code   string
	tag    language.Tag
}

// catalog is a fully compiled translation tree.
type catalog struct {
	regions    map[string]*regionData
	byLocale   map[string]localeBinding
	order      []string
	baseCode   string
	matcher    language.Matcher
	matchCodes []string
}

// bind resolves a locale code, or a region code, to the region serving it.
func (c *catalog) bind(code string) (localeBinding, bool) {
	binding, known := c.byLocale[code]

	return binding, known
}

// match negotiates an Accept-Language header against the configured locales,
// falling back to the default locale when nothing matches.
func (c *catalog) match(header string) localeBinding {
	desired, _, err := language.ParseAcceptLanguage(header)
	if err != nil || len(desired) == 0 {
		return c.byLocale[c.baseCode]
	}

	_, index, _ := c.matcher.Match(desired...)
	if index < 0 || index >= len(c.matchCodes) {
		return c.byLocale[c.baseCode]
	}

	return c.byLocale[c.matchCodes[index]]
}

// sortedKeys returns a map's keys in sorted order, for deterministic output.
func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}
