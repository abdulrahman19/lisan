package lisan

import "net/http"

// acceptLanguageHeader is the request header used to negotiate a locale.
const acceptLanguageHeader = "Accept-Language"

// Middleware negotiates Accept-Language against the configured locales and
// stores the winner in the request context, where From can pick it up.
//
// A request asking for a locale that has no folder of its own still matches the
// region serving it, because negotiation runs over every locale listed under
// regions.
func (i *I18n) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if FromContext(request.Context()) == "" {
			binding := i.catalog.match(request.Header.Get(acceptLanguageHeader))
			request = request.WithContext(WithLocale(request.Context(), binding.code))
		}

		next.ServeHTTP(writer, request)
	})
}

// Negotiate reports the best configured locale for an Accept-Language header,
// for callers that are not using net/http handlers.
func (i *I18n) Negotiate(header string) string {
	return i.catalog.match(header).code
}
