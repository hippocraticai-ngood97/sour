package authz

import (
	"context"
	"net/http"

	"github.com/golang-jwt/jwt/v5/request"
	"github.com/smw1218/sour/response"
	"github.com/smw1218/sour/routes"
)

type authContextType string

var tokenKey authContextType = "token"

func NewAuthMiddleware(ac *AuthChecker) routes.Middleware {
	return &authMiddleware{ac}
}

type authMiddleware struct {
	ac *AuthChecker
}

func (am *authMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokString, err := request.BearerExtractor{}.ExtractToken(r)
		if err != nil {
			response.NewError(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized)).Respond(w)
			return
		}
		token, err := am.ac.CheckAuth(tokString)
		if err != nil {
			response.NewError(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized)).Respond(w)
			return
		}
		ctx := context.WithValue(r.Context(), tokenKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
