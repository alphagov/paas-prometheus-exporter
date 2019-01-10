package util

import (
	"fmt"
	"net/http"
)

func BasicAuthHandler(user, pass, realm string, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if checkBasicAuth(r, user, pass) {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, realm))
		w.WriteHeader(401)
		w.Write([]byte("401 Unauthorized\n"))
	}
}

func checkBasicAuth(r *http.Request, user, pass string) bool {
	u, p, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return u == user && p == pass
}
