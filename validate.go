package lisan

import (
	"fmt"
	"slices"
	"strings"

	"golang.org/x/text/feature/plural"
)

// Bounds on how far a misspelling may sit from a known placeholder before the
// compiler stops guessing. The allowance scales with name length, so "username"
// still suggests "name" while "xyz" suggests nothing.
const (
	minSuggestionDistance = 2
	maxSuggestionDistance = 5
	suggestionHalving     = 2
)

// addEntry validates one raw entry and files it under its region.
func (c *compiler) addEntry(region *regionData, file *sourceFile, raw rawEntry) {
	pos := file.position(raw.offset)

	id := raw.fields[entryKeyID]
	if id == "" {
		c.problemf(file.path, pos, "", "has an entry with no %q", entryKeyID)

		return
	}

	if existing, duplicate := region.entries[id]; duplicate {
		c.problemf(file.path, pos, id,
			"is already declared in region %q at %s:%d", region.code, existing.file, existing.pos.line)

		return
	}

	built := &entry{id: id, file: file.path, pos: pos}

	if c.fillEntry(region, built, raw) {
		region.entries[id] = built
	}
}

// fillEntry classifies an entry as plural or non-plural and compiles its text.
func (c *compiler) fillEntry(region *regionData, built *entry, raw rawEntry) bool {
	if unknown := unknownKeys(raw.fields); len(unknown) > 0 {
		c.problemAt(built, "has unrecognised key(s) %s", strings.Join(unknown, ", "))

		return false
	}

	text, hasText := raw.fields[entryKeyText]
	declared := declaredForms(raw.fields)

	switch {
	case hasText && len(declared) > 0:
		c.problemAt(built, "declares both %q and plural categories [%s]; an entry is one or the other",
			entryKeyText, strings.Join(formNames(declared), " "))

		return false
	case hasText:
		return c.fillText(built, text)
	case len(declared) > 0:
		return c.fillPlural(region, built, raw, declared)
	default:
		c.problemAt(built, "has neither %q nor any plural category", entryKeyText)

		return false
	}
}

// fillText compiles a non-plural entry.
func (c *compiler) fillText(built *entry, text string) bool {
	parsed, err := parsePhrase(text, c.config.Globals)
	if err != nil {
		c.problemAt(built, "%v", err)

		return false
	}

	if parsed.usesCount {
		c.problemAt(built, "uses %scount%s, which is only available on plural entries", openDelim, closeDelim)

		return false
	}

	built.text = parsed

	return true
}

// fillPlural compiles a plural entry after checking it declares exactly the
// categories its language requires.
func (c *compiler) fillPlural(region *regionData, built *entry, raw rawEntry, declared []plural.Form) bool {
	if !c.checkCategorySet(region, built, declared) {
		return false
	}

	built.plural = true
	built.forms = make(map[plural.Form]phrase, len(declared))

	complete := true

	for _, form := range declared {
		parsed, err := parsePhrase(raw.fields[formName(form)], c.config.Globals)
		if err != nil {
			c.problemAt(built, "category %q %v", formName(form), err)

			complete = false

			continue
		}

		built.forms[form] = parsed
	}

	return complete
}

// checkCategorySet enforces that a plural entry declares exactly the CLDR
// categories of its language, naming what is missing and what is extra.
func (c *compiler) checkCategorySet(region *regionData, built *entry, declared []plural.Form) bool {
	want := formNames(region.forms)
	got := formNames(declared)

	if slices.Equal(got, want) {
		return true
	}

	detail := ""

	if missing := difference(want, got); len(missing) > 0 {
		detail += "; missing: " + strings.Join(missing, ", ")
	}

	if extra := difference(got, want); len(extra) > 0 {
		detail += "; unexpected: " + strings.Join(extra, ", ")
	}

	c.problemAt(built, "declares categories [%s] but %s requires exactly [%s]%s",
		strings.Join(got, " "), region.code, strings.Join(want, " "), detail)

	return false
}

// validateAgainstBase compares every region to the base region, which defines
// the canonical set of identifiers and placeholders.
func (c *compiler) validateAgainstBase() {
	base, known := c.regions[c.config.Settings.BaseRegion]
	if !known {
		return
	}

	for _, code := range sortedKeys(c.regions) {
		if code != base.code {
			c.compareRegion(base, c.regions[code])
		}
	}
}

// compareRegion reports identifiers that are missing from, or foreign to, a
// non-base region.
func (c *compiler) compareRegion(base, region *regionData) {
	for _, id := range sortedKeys(base.entries) {
		if _, present := region.entries[id]; !present {
			c.problemf(region.code, textPosition{}, id, "is declared in base region %q but missing here", base.code)
		}
	}

	for _, id := range sortedKeys(region.entries) {
		current := region.entries[id]

		reference, present := base.entries[id]
		if !present {
			c.problemAt(current, "is not declared in base region %q", base.code)

			continue
		}

		c.compareEntry(base.code, reference, current)
	}
}

// compareEntry checks that an identifier keeps the same kind and stays within
// the placeholders the base region defines for it.
func (c *compiler) compareEntry(baseCode string, reference, current *entry) {
	if reference.plural != current.plural {
		c.problemAt(current, "is %s here but %s in base region %q",
			entryKind(current), entryKind(reference), baseCode)

		return
	}

	allowed := reference.argNames()

	for _, name := range current.argNames() {
		if slices.Contains(allowed, name) {
			continue
		}

		detail := ""
		if suggestion, found := closestName(name, allowed); found {
			detail = fmt.Sprintf("; did you mean %s%s%s?", openDelim, suggestion, closeDelim)
		}

		c.problemAt(current, "uses placeholder %s%s%s, which is not defined for this id in base region %q%s",
			openDelim, name, closeDelim, baseCode, detail)
	}
}

// problemAt records a defect located at an entry.
func (c *compiler) problemAt(built *entry, format string, args ...any) {
	c.problemf(built.file, built.pos, built.id, format, args...)
}

// declaredForms lists the plural categories an entry declares, in canonical
// order.
func declaredForms(fields map[string]string) []plural.Form {
	var declared []plural.Form

	for _, form := range canonicalForms {
		if _, present := fields[formName(form)]; present {
			declared = append(declared, form)
		}
	}

	return declared
}

// unknownKeys lists entry keys that are neither the identifier, the text, nor
// a plural category.
func unknownKeys(fields map[string]string) []string {
	var unknown []string

	for _, key := range sortedKeys(fields) {
		if key == entryKeyID || key == entryKeyText {
			continue
		}

		if _, isForm := formByName(key); !isForm {
			unknown = append(unknown, fmt.Sprintf("%q", key))
		}
	}

	return unknown
}

// entryKind describes an entry for use in an error message.
func entryKind(built *entry) string {
	if built.plural {
		return "a plural entry"
	}

	return "a non-plural entry"
}

// difference returns the elements of want that are absent from got.
func difference(want, got []string) []string {
	var missing []string

	for _, item := range want {
		if !slices.Contains(got, item) {
			missing = append(missing, item)
		}
	}

	return missing
}

// closestName finds the candidate nearest to name, so a misspelled placeholder
// can be reported with a suggestion.
func closestName(name string, candidates []string) (string, bool) {
	best := ""
	bestDistance := 0
	found := false

	for _, candidate := range candidates {
		distance := editDistance(name, candidate)
		if distance > suggestionThreshold(name, candidate) {
			continue
		}

		if !found || distance < bestDistance {
			best, bestDistance, found = candidate, distance, true
		}
	}

	return best, found
}

// suggestionThreshold allows a larger edit distance between longer names.
func suggestionThreshold(from, to string) int {
	longer := max(len(from), len(to))

	return min(max(longer/suggestionHalving, minSuggestionDistance), maxSuggestionDistance)
}

// editDistance computes the Levenshtein distance between two strings.
func editDistance(from, to string) int {
	previous := make([]int, len(to)+1)
	current := make([]int, len(to)+1)

	for column := range previous {
		previous[column] = column
	}

	for row := 1; row <= len(from); row++ {
		current[0] = row

		for column := 1; column <= len(to); column++ {
			substitution := previous[column-1]
			if from[row-1] != to[column-1] {
				substitution++
			}

			current[column] = min(substitution, previous[column]+1, current[column-1]+1)
		}

		previous, current = current, previous
	}

	return previous[len(to)]
}
