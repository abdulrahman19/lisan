package lisan

import "context"

// localeKey is the private context key carrying the active locale. A package
// private type keeps it from colliding with any other package's keys.
type localeKey struct{}

// WithLocale returns a copy of parent carrying code as the active locale.
// Scoping the locale to the context is what makes concurrent requests in
// different languages safe.
func WithLocale(parent context.Context, code string) context.Context {
	return context.WithValue(parent, localeKey{}, code)
}

// FromContext reports the locale code carried by ctx, or an empty string when
// it carries none.
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	code, carried := ctx.Value(localeKey{}).(string)
	if !carried {
		return ""
	}

	return code
}

// From returns a Localizer for the locale carried by ctx, falling back to the
// default locale when ctx carries none.
func (i *I18n) From(ctx context.Context) *Localizer {
	code := FromContext(ctx)
	if code == "" {
		code = i.fallback.code
	}

	return i.Locale(code)
}
