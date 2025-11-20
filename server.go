package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Edge-Center/external-dns-ec-webhook/log"
	"github.com/Edge-Center/external-dns-ec-webhook/provider"
	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

const (
	HeaderContentType = "Content-Type"
	HeaderAccept      = "Accept"
	HeaderVary        = "Vary"

	ContentTypePlainText = "text/plain"
	ContentTypeAppJson   = "application/external.dns.webhook+json;version=1"
)

var supportedMediaVersion = "application/external.dns.webhook+json;version=1"

type Server struct {
	*http.Server
}

func StartServer(p *provider.DnsProvider, addr string) {
	logger := log.Logger(context.Background())

	api := InitAPI(p)
	srv := &Server{
		&http.Server{
			Addr:    addr,
			Handler: api,
		},
	}

	// start listening
	go func() {
		logger.Infof("starting listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("can't serve on addr %s", srv.Addr)
		}
	}()

	// shutdown process
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	sig := <-sigCh
	logger.Infof("shutting down server due to received signal: %v", sig)
	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	if err := srv.Shutdown(ctx); err != nil {
		logger.WithField(log.ErrorKey, err).Error("error shutting down server")
	}
}

// InitAPI will create a router with the following API
// - / (GET): initialization, negotiates headers and returns the domain filter
// - /healthz (GET) health endpount for checking if app is up
// - /records (GET): returns the current records
// - /records (POST): applies the changes
// - /adjustendpoints (POST): executes the AdjustEndpoints method
func InitAPI(p *provider.DnsProvider) *chi.Mux {
	r := chi.NewRouter()

	//
	// GET /healthz
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	//
	// GET /
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(log.Trace(r.Context()))
		logger := logWithReqInfo(r)
		logger.Debug("GET /")

		err := checkHeaders(w, r)
		if err != nil {
			logger.WithField(log.ErrorKey, err).Error("failed header check")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		b, err := p.GetDomainFilter(r.Context()).MarshalJSON()
		if err != nil {
			logger.WithField(log.ErrorKey, err).Error("failed to marshal domain filter")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set(HeaderContentType, ContentTypeAppJson)
		if _, err = w.Write(b); err != nil {
			logger.WithField(log.ErrorKey, err).Error("failed to write response")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	//
	// GET /records
	r.Get("/records", func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(log.Trace(r.Context()))
		logger := logWithReqInfo(r)
		logger.Debug("GET /records")

		err := checkHeaders(w, r)
		if err != nil {
			logger.WithField(log.ErrorKey, err).Error("failed header check")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		records, err := p.Records(r.Context())
		if err != nil {
			logger.WithField(log.ErrorKey, err).Error("failed to get records from provider")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		logger.Infof("found %d records", len(records))

		w.Header().Set(HeaderContentType, ContentTypeAppJson)
		w.Header().Set(HeaderVary, HeaderContentType)

		err = json.NewEncoder(w).Encode(records)
		if err != nil {
			logger.WithField(log.ErrorKey, err).Error("failed to encode records")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
	r.Post("/records", func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(log.Trace(r.Context()))
		logger := logWithReqInfo(r)
		logger.Info("POST /records")

		err := checkHeaders(w, r)
		if err != nil {
			logger.WithField(log.ErrorKey, err).Error("failed header check")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var changes *plan.Changes
		if err = json.NewDecoder(r.Body).Decode(changes); err != nil {
			logger.WithField(log.ErrorKey, err).Warning("failed to decode changes")

			w.Header().Set(HeaderContentType, ContentTypePlainText)
			errMsg := fmt.Sprintf("failed to decode changes: %s", err)
			if _, err = fmt.Fprint(w, errMsg); err != nil {
				logger.WithField(log.ErrorKey, err).Error("failed to write error message to response")
			}

			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err = p.ApplyChanges(r.Context(), changes); err != nil {
			logger.WithField(log.ErrorKey, err).Error("failed to apply changes")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	r.Post("/adjustendpoints", func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(log.Trace(r.Context()))
		logger := logWithReqInfo(r)
		logger.Info("POST /adjustendpoints")

		err := checkHeaders(w, r)
		if err != nil {
			logger.WithField(log.ErrorKey, err).Error("failed header check")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var endpoints []*endpoint.Endpoint
		if err = json.NewDecoder(r.Body).Decode(&endpoints); err != nil {
			logger.WithField(log.ErrorKey, err).Warning("failed to decode endpoints for adjustment")

			w.Header().Set(HeaderContentType, ContentTypePlainText)
			errMsg := fmt.Sprintf("failed to decode endpoints for adjustment: %s", err)
			if _, err = fmt.Fprint(w, errMsg); err != nil {
				logger.WithField(log.ErrorKey, err).Error("failed to write error message to response")
			}

			w.WriteHeader(http.StatusBadRequest)
			return
		}

		endpoints, err = p.AdjustEndpoints(endpoints)
		if err != nil {
			logger.WithField(log.ErrorKey, err).Error("failed to adjust endpoints")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set(HeaderContentType, ContentTypeAppJson)
		w.Header().Set(HeaderVary, HeaderContentType)
		err = json.NewEncoder(w).Encode(endpoints)
		if err != nil {
			logger.WithField(log.ErrorKey, err).Error("failed to encode endpoints")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	return r
}

func checkHeaders(w http.ResponseWriter, r *http.Request) error {
	var header string
	logger := logWithReqInfo(r)
	if r.Method == http.MethodPost {
		header = r.Header.Get(HeaderContentType)
	} else {
		header = r.Header.Get(HeaderAccept)
	}

	logger.WithField("header", header).Debug("start validating header")

	if len(header) == 0 {
		w.Header().Set(HeaderContentType, ContentTypePlainText)
		w.WriteHeader(http.StatusBadRequest)

		var err error
		if r.Method == http.MethodPost {
			err = errors.New("'Content-Type' header is required")
		} else {
			err = errors.New("'Accept' header is required")
		}
		if _, writeEr := fmt.Fprint(w, err); writeEr != nil {
			logWithReqInfo(r).WithField(log.ErrorKey, writeEr).Fatal("got error on writing error message to response writer")
		}
		return err
	}

	if header != supportedMediaVersion {
		w.Header().Set(HeaderContentType, ContentTypePlainText)
		w.WriteHeader(http.StatusUnsupportedMediaType)

		var err error
		if r.Method == http.MethodPost {
			err = errors.New("valid media type is required in 'Content-Type' header")
		} else {
			err = errors.New("valid media type is required in 'Accept' header")
		}
		if _, writeEr := fmt.Fprint(w, err); writeEr != nil {
			logger.WithField(log.ErrorKey, writeEr).Fatal("got error on writing error message to response writer")
		}
		return err
	}

	return nil
}

// logWithReqInfo uses traced(!) ctx and info from request to log valuable debug fields
func logWithReqInfo(r *http.Request) *logrus.Entry {
	return log.Logger(r.Context()).WithFields(logrus.Fields{"method": r.Method, "url": r.URL.Path})
}
