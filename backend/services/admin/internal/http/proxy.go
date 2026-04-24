package http

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"go.uber.org/zap"
)

// ServiceProxy reverse-proxies requests to a downstream service, injecting
// the X-Admin-Secret header so the downstream can authenticate admin requests.
type ServiceProxy struct {
	proxy       *httputil.ReverseProxy
	adminSecret string
	stripPrefix string
}

// NewServiceProxy creates a proxy that forwards requests to targetURL,
// stripping stripPrefix from the request path before forwarding.
// The downstream service receives X-Admin-Secret for authentication.
func NewServiceProxy(targetURL, stripPrefix, adminSecret string, log *zap.Logger) (http.Handler, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	rp := httputil.NewSingleHostReverseProxy(target)

	// Custom director: rewrite path and inject admin secret.
	defaultDirector := rp.Director
	rp.Director = func(req *http.Request) {
		defaultDirector(req)

		// Strip the gateway prefix (e.g. /admin/kyc) and rewrite to downstream path.
		// /admin/kyc/applications/123 → /kyc/applications/123
		if stripPrefix != "" && strings.HasPrefix(req.URL.Path, stripPrefix) {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, stripPrefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
			req.URL.RawPath = ""
		}

		req.Header.Set("X-Admin-Secret", adminSecret)
		req.Header.Del("Authorization") // do not forward user JWT to downstream
	}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Warn("proxy error", zap.String("path", r.URL.Path), zap.Error(err))
		writeError(w, http.StatusBadGateway, "upstream_error", "upstream service unavailable")
	}

	return &ServiceProxy{proxy: rp, adminSecret: adminSecret, stripPrefix: stripPrefix}, nil
}

func (sp *ServiceProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sp.proxy.ServeHTTP(w, r)
}
