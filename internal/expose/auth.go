package expose

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// BasicAuth wraps next with HTTP Basic authentication. userpass is "user:pass".
// When empty, next is returned unchanged. Use this on exposed routes so a public
// URL is not wide open.
func BasicAuth(next http.Handler, userpass string) http.Handler {
	if userpass == "" {
		return next
	}
	wantUser, wantPass, _ := strings.Cut(userpass, ":")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || !constEqual(user, wantUser) || !constEqual(pass, wantPass) {
			w.Header().Set("WWW-Authenticate", `Basic realm="prx"`)
			http.Error(w, "401 Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func constEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
