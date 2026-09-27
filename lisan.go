// Package lisan provides compile-time-validated internationalization for Go.
//
// A translation tree is read from any fs.FS, validated in full before the first
// lookup, and rendered with CLDR-correct pluralization from golang.org/x/text.
// Compilation happens entirely in memory, so nothing is written to disk.
//
// The package-level API wraps a single hidden instance installed by Configure.
// Use New instead when a program needs several independent translation sets.
//
// The active locale travels in a context.Context rather than in package state,
// so concurrent requests in different languages cannot interfere.
package lisan

import (
	"context"
	"io/fs"
	"net/http"
	"sync/atomic"
)

// configured holds the instance backing the package-level API. An atomic
// pointer keeps Configure safe to call while lookups are in flight.
var configured atomic.Pointer[I18n]

// Configure compiles a translation tree and installs it for the package-level
// API. It returns a *CompileError describing every defect in the tree.
//
// Load environment variables first: globals written as ${VAR} are resolved
// during compilation.
func Configure(fsys fs.FS, opts ...Option) error {
	instance, err := New(fsys, opts...)
	if err != nil {
		return err
	}

	configured.Store(instance)

	return nil
}

// Default returns the instance installed by Configure, or nil.
func Default() *I18n {
	return configured.Load()
}

// T renders a non-plural translation in the default locale.
func T(id string, args ...Args) Translation {
	instance := configured.Load()
	if instance == nil {
		return Translation{Text: id, Err: ErrNotConfigured}
	}

	return instance.T(id, args...)
}

// TN renders a plural translation in the default locale, choosing the CLDR
// category from count.
func TN(id string, count Number, args ...Args) Translation {
	instance := configured.Load()
	if instance == nil {
		return Translation{Text: id, Err: ErrNotConfigured}
	}

	return instance.TN(id, count, args...)
}

// TryT renders a non-plural translation in the default locale, reporting
// failure as an error.
func TryT(id string, args ...Args) (string, error) {
	instance := configured.Load()
	if instance == nil {
		return missingKeyText, ErrNotConfigured
	}

	return instance.TryT(id, args...)
}

// TryTN renders a plural translation in the default locale, reporting failure
// as an error.
func TryTN(id string, count Number, args ...Args) (string, error) {
	instance := configured.Load()
	if instance == nil {
		return missingKeyText, ErrNotConfigured
	}

	return instance.TryTN(id, count, args...)
}

// Locale returns a Localizer bound to one locale or region code. Every method
// on the result reports ErrNotConfigured until Configure has succeeded.
func Locale(code string) *Localizer {
	instance := configured.Load()
	if instance == nil {
		return nil
	}

	return instance.Locale(code)
}

// From returns a Localizer for the locale carried by ctx, falling back to the
// default locale when ctx carries none.
func From(ctx context.Context) *Localizer {
	instance := configured.Load()
	if instance == nil {
		return nil
	}

	return instance.From(ctx)
}

// Middleware negotiates Accept-Language against the configured locales and
// stores the winner in the request context. Before Configure succeeds it
// passes requests through untouched.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		instance := configured.Load()
		if instance == nil {
			next.ServeHTTP(writer, request)

			return
		}

		instance.Middleware(next).ServeHTTP(writer, request)
	})
}

// Negotiate reports the best configured locale for an Accept-Language header.
func Negotiate(header string) string {
	instance := configured.Load()
	if instance == nil {
		return ""
	}

	return instance.Negotiate(header)
}

// DefaultLocale reports the locale used when no other is selected.
func DefaultLocale() string {
	instance := configured.Load()
	if instance == nil {
		return ""
	}

	return instance.DefaultLocale()
}

// Regions lists every configured region code, in sorted order.
func Regions() []string {
	instance := configured.Load()
	if instance == nil {
		return nil
	}

	return instance.Regions()
}

// GetRegionName returns a region's name in its own language.
func GetRegionName(code string) string {
	instance := configured.Load()
	if instance == nil {
		return ""
	}

	return instance.GetRegionName(code)
}

// GetRegionLocales lists the locales a region serves.
func GetRegionLocales(code string) []string {
	instance := configured.Load()
	if instance == nil {
		return nil
	}

	return instance.GetRegionLocales(code)
}
