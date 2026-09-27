package lisan

import (
	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
)

// Probe bounds used to discover which plural categories a language actually
// uses. golang.org/x/text exposes CLDR rules but not the category set they can
// produce, so the set is derived by evaluating the rules over a range of
// operands wide enough to reach every category in every CLDR locale.
const (
	probeMaxInteger    = 1000
	probeMaxFractional = 120
	probeOneDigitMax   = 9
	probeTwoDigitMax   = 99
	oneFractionDigit   = 1
	twoFractionDigits  = 2
)

// largeProbes covers magnitude-sensitive rules, such as the French and Spanish
// "many" category, which only triggers at a million and above.
var largeProbes = []int{
	10_000, 100_000,
	1_000_000, 1_100_000, 2_000_000,
	10_000_000, 100_000_000, 1_000_000_000,
}

// canonicalForms lists every CLDR plural form in the order translation files
// and error messages present them.
var canonicalForms = []plural.Form{
	plural.Zero,
	plural.One,
	plural.Two,
	plural.Few,
	plural.Many,
	plural.Other,
}

// formName returns the translation-file key for a plural form.
func formName(form plural.Form) string {
	switch form {
	case plural.Zero:
		return "zero"
	case plural.One:
		return "one"
	case plural.Two:
		return "two"
	case plural.Few:
		return "few"
	case plural.Many:
		return "many"
	case plural.Other:
		return "other"
	default:
		return ""
	}
}

// formByName maps a translation-file key back to a plural form.
func formByName(name string) (plural.Form, bool) {
	for _, form := range canonicalForms {
		if formName(form) == name {
			return form, true
		}
	}

	return plural.Other, false
}

// formNames renders a set of forms as their translation-file keys.
func formNames(forms []plural.Form) []string {
	names := make([]string, 0, len(forms))
	for _, form := range forms {
		names = append(names, formName(form))
	}

	return names
}

// categoriesFor reports the plural categories the given language can produce,
// in canonical order. Every language has at least "other".
func categoriesFor(tag language.Tag) []plural.Form {
	seen := make(map[plural.Form]bool, len(canonicalForms))
	seen[plural.Other] = true

	probeIntegers(tag, seen)
	probeFractions(tag, seen)

	forms := make([]plural.Form, 0, len(seen))

	for _, form := range canonicalForms {
		if seen[form] {
			forms = append(forms, form)
		}
	}

	return forms
}

// probeIntegers records the categories reachable by whole numbers.
func probeIntegers(tag language.Tag, seen map[plural.Form]bool) {
	for value := 0; value <= probeMaxInteger; value++ {
		seen[plural.Cardinal.MatchPlural(tag, value, 0, 0, 0, 0)] = true
	}

	for _, value := range largeProbes {
		seen[plural.Cardinal.MatchPlural(tag, value, 0, 0, 0, 0)] = true
	}
}

// probeFractions records the categories reachable by numbers with one or two
// visible fraction digits, which is where "other" splits off in many locales.
func probeFractions(tag language.Tag, seen map[plural.Form]bool) {
	for whole := 0; whole <= probeMaxFractional; whole++ {
		probeFractionRange(tag, seen, whole, oneFractionDigit, probeOneDigitMax)
		probeFractionRange(tag, seen, whole, twoFractionDigits, probeTwoDigitMax)
	}
}

// probeFractionRange probes every fraction of the given visible digit count.
func probeFractionRange(tag language.Tag, seen map[plural.Form]bool, whole, visible, maxFraction int) {
	for value := 0; value <= maxFraction; value++ {
		trimmed := trimFraction(value, visible)
		seen[plural.Cardinal.MatchPlural(tag, whole, visible, trimmed.digits, value, trimmed.value)] = true
	}
}

// resolveForm picks the plural category for a count in the given language.
func resolveForm(tag language.Tag, count Number) plural.Form {
	return plural.Cardinal.MatchPlural(
		tag,
		count.digits,
		count.visible,
		count.visibleTrimmed,
		count.frac,
		count.fracTrimmed,
	)
}
