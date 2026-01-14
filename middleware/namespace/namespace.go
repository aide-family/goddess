// Package namespace is a middleware that validates namespace tokens.
package namespace

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/aide-family/goddess/middleware"
	config "github.com/aide-family/goddess/pkg/config/v1"
	"github.com/aide-family/goddess/pkg/merr"
	v1 "github.com/aide-family/goddess/pkg/middleware/namespace"
	"github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	defaultNamespaceKey = "X-Namespace"
	defaultTimeout      = 5 * time.Second
	modeWhitelist       = "whitelist"
	modeAPI             = "api"
)

func init() {
	middleware.Register("namespace", Middleware)
}

func Middleware(c *config.Middleware) (middleware.Middleware, error) {
	options := &v1.Namespace{}
	if c.Options != nil {
		if err := anypb.UnmarshalTo(c.Options, options, proto.UnmarshalOptions{Merge: true}); err != nil {
			return nil, err
		}
	}
	namespaceKey := options.Key
	if namespaceKey == "" {
		namespaceKey = defaultNamespaceKey
	}

	// Create HTTP client for API validation if needed
	var httpClient *http.Client
	if options.ValidateApi != nil && options.ValidateApi.Url != "" {
		timeout := options.ValidateApi.Timeout.AsDuration()
		if timeout <= 0 {
			timeout = defaultTimeout
		}
		httpClient = &http.Client{
			Timeout: timeout,
		}
	}

	// Build whitelist map for fast lookup
	whitelistMap := make(map[string]bool)
	for _, ns := range options.AllowedNamespaces {
		whitelistMap[ns] = true
	}
	validationMode := strings.ToLower(strings.TrimSpace(options.ValidationMode))

	var validationFunc func(ctx context.Context, ns string) error
	switch validationMode {
	case modeWhitelist:
		if len(whitelistMap) == 0 {
			return nil, merr.ErrorInternal("whitelist validation mode is specified but no allowed_namespaces configured")
		}
		validationFunc = func(ctx context.Context, ns string) error {
			if whitelistMap[ns] {
				return nil
			}
			return merr.ErrorForbidden("namespace is not allowed")
		}
	case modeAPI:
		if httpClient == nil {
			return nil, merr.ErrorInternal("api validation mode is specified but http client is not configured")
		}
		validationFunc = func(ctx context.Context, ns string) error {
			return validateNamespaceViaAPI(ctx, httpClient, ns, options.ValidateApi)
		}
	default:
		validationFunc = func(ctx context.Context, ns string) error {
			if len(whitelistMap) > 0 {
				if whitelistMap[ns] {
					return nil
				}
			}
			if httpClient != nil {
				if err := validateNamespaceViaAPI(ctx, httpClient, ns, options.ValidateApi); err != nil {
					return err
				}
			}
			return merr.ErrorForbidden("namespace is not allowed")
		}
	}

	return func(next http.RoundTripper) http.RoundTripper {
		return middleware.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			namespace := req.Header.Get(namespaceKey)

			if options.Required && namespace == "" {
				return newForbiddenResponse(merr.ErrorForbidden("namespace is required"))
			}

			if namespace != "" {
				if err := validationFunc(req.Context(), namespace); err != nil {
					return newForbiddenResponse(err)
				}
			}
			return next.RoundTrip(req)
		})
	}, nil
}

// validateNamespaceViaAPI validates namespace by calling external API
func validateNamespaceViaAPI(ctx context.Context, client *http.Client, namespace string, apiConfig *v1.ValidateApi) error {
	// Prepare request body
	var body io.Reader
	if apiConfig.BodyTemplate != "" {
		tmpl, err := template.New("body").Parse(apiConfig.BodyTemplate)
		if err != nil {
			return merr.ErrorInternal("failed to parse body template: %v", err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, map[string]string{"namespace": namespace}); err != nil {
			return merr.ErrorInternal("failed to execute body template: %v", err)
		}
		body = bytes.NewBuffer(buf.Bytes())
	}

	// Create request
	method := apiConfig.Method
	if method == "" {
		method = http.MethodPost // default method
	}
	req, err := http.NewRequestWithContext(ctx, method, apiConfig.Url, body)
	if err != nil {
		return merr.ErrorInternal("failed to create validation request: %v", err)
	}

	// Set headers
	for k, v := range apiConfig.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return merr.ErrorInternal("failed to validate namespace: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	successCodes := apiConfig.SuccessStatusCodes
	if len(successCodes) == 0 {
		successCodes = []int32{200} // default success code
	}
	isSuccess := slices.Contains(successCodes, int32(resp.StatusCode))

	if !isSuccess {
		return merr.ErrorForbidden("namespace validation failed: status code %d", resp.StatusCode)
	}

	return nil
}

// newForbiddenResponse creates a forbidden HTTP response
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
