package httpapi

import "net/http"

// CORS lets the browser app (a different origin, like http://localhost:3000)
// call this API. Only origins in the allowed list get the CORS headers.
//
// We use a Bearer token in a header, not cookies, so credentials are not allowed
// and there is no Access-Control-Allow-Credentials.
func CORS(allowed []string) func(http.Handler) http.Handler {
	set := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		set[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r) // not a browser cross-origin call
				return
			}

			w.Header().Add("Vary", "Origin")

			if set[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)

				// Preflight: the browser asks "may I send this?" before the real call.
				if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
					w.Header().Set("Access-Control-Max-Age", "600")
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
