package handlers

import (
	"encoding/base64"
	"net/http"
	"strings"
)

// BasicAuth 返回一个 HTTP Basic Auth 中间件，使用给定的用户名密码保护处理器。
func BasicAuth(user, pass string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Basic ") {
				w.Header().Set("WWW-Authenticate", `Basic realm="Admin Area"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			payload, err := base64.StdEncoding.DecodeString(auth[6:])
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Basic realm="Admin Area"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			pair := strings.SplitN(string(payload), ":", 2)
			if len(pair) != 2 || pair[0] != user || pair[1] != pass {
				w.Header().Set("WWW-Authenticate", `Basic realm="Admin Area"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next(w, r)
		}
	}
}
