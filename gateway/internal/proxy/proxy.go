package proxy

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"gateway/internal/middleware"
)

type Proxy struct {
	authURL         *url.URL
	userURL         *url.URL
	postURL         *url.URL
	notificationURL *url.URL
	logger          *slog.Logger
	timeout         time.Duration
}

func New(authURL, userURL, postURL, notificationURL string, timeout time.Duration, logger *slog.Logger) (*Proxy, error) {
	a, err := url.Parse(authURL)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(userURL)
	if err != nil {
		return nil, err
	}
	p, err := url.Parse(postURL)
	if err != nil {
		return nil, err
	}
	n, err := url.Parse(notificationURL)
	if err != nil {
		return nil, err
	}

	return &Proxy{
		authURL:         a,
		userURL:         u,
		postURL:         p,
		notificationURL: n,
		logger:          logger,
		timeout:         timeout,
	}, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/") {
		p.sendError(w, r, "Not Found", http.StatusNotFound)
		return
	}

	trimmedPath := strings.TrimPrefix(path, "/api")
	var target *url.URL

	switch {
	case trimmedPath == "/auth" || strings.HasPrefix(trimmedPath, "/auth/"):
		target = p.authURL
	case trimmedPath == "/users" || strings.HasPrefix(trimmedPath, "/users/"):
		target = p.userURL
	case trimmedPath == "/posts" || strings.HasPrefix(trimmedPath, "/posts/"):
		target = p.postURL
	case trimmedPath == "/notifications" || strings.HasPrefix(trimmedPath, "/notifications/"):
		target = p.notificationURL
	default:
		p.sendError(w, r, "Not Found", http.StatusNotFound)
		return
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = trimmedPath
			if target.RawQuery == "" || req.URL.RawQuery == "" {
				req.URL.RawQuery = target.RawQuery + req.URL.RawQuery
			} else {
				req.URL.RawQuery = target.RawQuery + "&" + req.URL.RawQuery
			}
			if _, ok := req.Header["User-Agent"]; !ok {
				req.Header.Set("User-Agent", "")
			}
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			status := http.StatusBadGateway
			var netErr net.Error
			if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netErr) && netErr.Timeout() {
				status = http.StatusGatewayTimeout
			}
			p.logger.Error("proxy error",
				slog.Any("error", err),
				slog.Int("status", status),
				slog.String("request_id", r.Header.Get(middleware.RequestIDHeader)),
			)
			p.sendError(w, r, http.StatusText(status), status)
		},
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: p.timeout,
		},
	}

	proxy.ServeHTTP(w, r)
}

func (p *Proxy) sendError(w http.ResponseWriter, r *http.Request, message string, code int) {
	middleware.WriteJSON(w, code, map[string]string{
		"error":      message,
		"request_id": r.Header.Get(middleware.RequestIDHeader),
	})
}
