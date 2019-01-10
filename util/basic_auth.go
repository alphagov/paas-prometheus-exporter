package util

import (
	"fmt"
	"net/http"
)

type basicAuth struct {
	username string
	password string
	realm    string
	next     http.Handler
}

func BasicAuthHandler(user, pass, realm string, next http.Handler) http.Handler {
	return &basicAuth{
		username: user,
		password: pass,
		realm:    realm,
		next:     next,
	}
}

func (b *basicAuth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if b.checkBasicAuth(r) {
		b.next.ServeHTTP(w, r)
		return
	}

	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, b.realm))
	http.Error(w, "401 Unauthorized.", http.StatusUnauthorized)
}

func (b *basicAuth) checkBasicAuth(r *http.Request) bool {
	u, p, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return u == b.username && p == b.password
}
