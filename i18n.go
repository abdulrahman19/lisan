package lisan

import (
	"fmt"
	"io/fs"

	"golang.org/x/text/message"
)

// Option configures an I18n at construction time.
type Option func(*settings)

// settings holds the resolved effect of every Option.
type settings struct {
	onMissing     func(MissingEvent)
	defaultLocale string
}

// WithDefaultLocale sets the locale used when a context carries none. It
// defaults to settings.base_region from config.json.
func WithDefaultLocale(code string) Option {
	return func(s *settings) { s.defaultLocale = code }
}

// WithOnMissing registers a hook fired on every failed lookup, which is useful
// for surfacing typo'd identifiers in staging.
func WithOnMissing(hook func(MissingEvent)) Option {
	return func(s *settings) { s.onMissing = hook }
}

// I18n is a compiled translation tree. It is immutable and safe for concurrent
// use once New returns.
type I18n struct {
	catalog   *catalog
	printers  map[string]*message.Printer
	onMissing func(MissingEvent)
	fallback  localeBinding
}

// New compiles a translation tree read from fsys. Every translation file is
// validated before New returns; any defect yields a *CompileError listing all
// of them. Nothing is written to disk.
func New(fsys fs.FS, opts ...Option) (*I18n, error) {
	if fsys == nil {
		return nil, fmt.Errorf("lisan: %w", fs.ErrInvalid)
	}

	compiled, err := compile(fsys)
	if err != nil {
		return nil, err
	}

	resolved := settings{defaultLocale: compiled.baseCode}
	for _, apply := range opts {
		apply(&resolved)
	}

	fallback, known := compiled.bind(resolved.defaultLocale)
	if !known {
		return nil, &CompileError{Problems: []Problem{{
			File:    configName,
			Message: fmt.Sprintf("default locale %q is not served by any region", resolved.defaultLocale),
		}}}
	}

	instance := &I18n{
		catalog:   compiled,
		printers:  buildPrinters(compiled),
		onMissing: resolved.onMissing,
		fallback:  fallback,
	}

	return instance, nil
}

// T renders a non-plural translation in the default locale.
func (i *I18n) T(id string, args ...Args) Translation {
	return i.Locale(i.fallback.code).T(id, args...)
}

// TN renders a plural translation in the default locale, choosing the CLDR
// category from count.
func (i *I18n) TN(id string, count Number, args ...Args) Translation {
	return i.Locale(i.fallback.code).TN(id, count, args...)
}

// TryT renders a non-plural translation in the default locale, reporting a
// missing identifier as an error.
func (i *I18n) TryT(id string, args ...Args) (string, error) {
	return i.Locale(i.fallback.code).TryT(id, args...)
}

// TryTN renders a plural translation in the default locale, reporting a
// missing identifier as an error.
func (i *I18n) TryTN(id string, count Number, args ...Args) (string, error) {
	return i.Locale(i.fallback.code).TryTN(id, count, args...)
}

// Locale returns a Localizer bound to one locale. It accepts any locale listed
// under a region's locales, or a region code directly. An unknown code falls
// back to the default locale.
func (i *I18n) Locale(code string) *Localizer {
	binding, known := i.catalog.bind(code)
	if !known {
		binding = i.fallback
	}

	return &Localizer{
		owner:   i,
		region:  binding.region,
		printer: i.printers[binding.code],
		code:    binding.code,
		tag:     binding.tag,
	}
}

// DefaultLocale reports the locale used when no other is selected.
func (i *I18n) DefaultLocale() string {
	return i.fallback.code
}

// Regions lists every configured region code, in sorted order.
func (i *I18n) Regions() []string {
	return append([]string(nil), i.catalog.order...)
}

// GetRegionName returns a region's name in its own language, or an empty
// string when the region is not configured.
func (i *I18n) GetRegionName(code string) string {
	region, known := i.catalog.regions[code]
	if !known {
		return ""
	}

	return region.name
}

// GetRegionLocales lists the locales a region serves, or nil when the region
// is not configured.
func (i *I18n) GetRegionLocales(code string) []string {
	region, known := i.catalog.regions[code]
	if !known {
		return nil
	}

	return append([]string(nil), region.locales...)
}

// buildPrinters creates one printer per bindable code up front, so that
// rendering needs no locking.
func buildPrinters(compiled *catalog) map[string]*message.Printer {
	printers := make(map[string]*message.Printer, len(compiled.byLocale))

	for code, binding := range compiled.byLocale {
		printers[code] = message.NewPrinter(binding.tag)
	}

	return printers
}
