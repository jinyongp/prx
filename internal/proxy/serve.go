package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"time"
)

// Run serves the HTTPS plane on httpsAddr and the HTTP→HTTPS redirector on
// httpAddr until ctx is cancelled, then drains in-flight requests gracefully.
func (s *Server) Run(ctx context.Context, httpsAddr, httpAddr string) error {
	httpsSrv := &http.Server{
		Addr:              httpsAddr,
		Handler:           s.HTTPSHandler(),
		ReadHeaderTimeout: 10 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: s.getCert,
		},
	}
	httpSrv := &http.Server{
		Addr:              httpAddr,
		Handler:           s.HTTPHandler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 2)
	go func() { errc <- serveTLS(httpsSrv) }()
	go func() { errc <- httpSrv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutdown(ctx, httpsSrv)
		shutdown(ctx, httpSrv)
		return ctx.Err()
	case err := <-errc:
		shutdown(ctx, httpsSrv)
		shutdown(ctx, httpSrv)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func serveTLS(srv *http.Server) error {
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	// Certificates come from TLSConfig.GetCertificate, so no files are passed.
	return srv.ServeTLS(ln, "", "")
}

// shutdown drains srv. The parent ctx is typically already cancelled, so the
// grace deadline runs on a detached copy (WithoutCancel) to allow in-flight
// requests to finish.
func shutdown(ctx context.Context, srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
