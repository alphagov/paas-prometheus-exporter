package util

import (
    "net/http"
    "fmt"
)

func BasicAuthHandler(user, pass, realm string, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if checkBasicAuth(r, user, pass) {
            next(w, r)
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
