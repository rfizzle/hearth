package rpc

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// unauthenticatedBody is the Connect error body for a refused request;
// Connect clients read the code from it and from the 401.
const unauthenticatedBody = `{"code":"unauthenticated","message":"missing or wrong launch token"}`

// requireToken refuses any request that does not carry the launch token as a
// bearer token. It is HTTP middleware rather than a Connect interceptor
// because an interceptor runs only for registered procedures; this refuses
// unknown paths too. The token stands in for the peer-uid check a Unix
// socket would have given.
// See docs/architecture.md#5-security-model
func requireToken(token string, next http.Handler) http.Handler {
	want := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(got), want) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(unauthenticatedBody))
			return
		}
		next.ServeHTTP(w, r)
	})
}
