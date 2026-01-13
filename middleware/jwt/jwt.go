package jwt

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/aide-family/goddess/middleware"
	config "github.com/aide-family/goddess/pkg/config/v1"
	"github.com/aide-family/goddess/pkg/merr"
	jwtv1 "github.com/aide-family/goddess/pkg/middleware/jwt/v1"
	"github.com/go-kratos/kratos/v2/errors"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func init() {
	middleware.Register("jwt", Middleware)
}

func Middleware(c *config.Middleware) (middleware.Middleware, error) {
	options := &jwtv1.Jwt{}
	if c.Options != nil {
		if err := anypb.UnmarshalTo(c.Options, options, proto.UnmarshalOptions{Merge: true}); err != nil {
			return nil, err
		}
	}
	return func(next http.RoundTripper) http.RoundTripper {
		return middleware.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			auths := strings.SplitN(req.Header.Get("Authorization"), " ", 2)
			if len(auths) != 2 || !strings.EqualFold(auths[0], "Bearer") {
				return newForbiddenResponse(merr.ErrorForbidden("invalid token 0"))
			}
			jwtToken := auths[1]
			token, err := jwtv5.Parse(jwtToken, func(token *jwtv5.Token) (interface{}, error) {
				return []byte(options.Secret), nil
			}, jwtv5.WithValidMethods(options.Algorithms), jwtv5.WithIssuer(options.Issuer))
			if err != nil {
				return newForbiddenResponse(merr.ErrorForbidden("invalid token 1"))
			}
			if !token.Valid {
				return newForbiddenResponse(merr.ErrorForbidden("invalid token 2"))
			}

			// TODO: add user id to request context
			return next.RoundTrip(req)
		})
	}, nil
}

func newForbiddenResponse(err error) (*http.Response, error) {
	kerr := errors.FromError(err)
	body, err := json.Marshal(kerr)
	if err != nil {
		return nil, err
	}
	header := http.Header{}
	header.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: int(kerr.Code),
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(body)),
	}, nil
}
