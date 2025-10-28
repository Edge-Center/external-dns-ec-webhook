package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/Edge-Center/external-dns-ec-webhook/provider"
	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"
)

const (
	HeaderContentType = "Content-Type"
	HeaderAccept      = "Accept"

	ContentTypePlainText = "text/plain"
)

var supportedMediaVersions = "application/json"

type Server struct {
	*http.Server
}

func StartServer(provider *provider.DnsProvider) {

}

func InitEndpoints() {
	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		logWithReqInfo(r).Debug("GET /")

		err := checkHeaders(w, r)
		if err != nil {
			logWithReqInfo(r).WithField(logKeyError, err).Error("failed header check")
		}
	})
	r.Get("/records", func(w http.ResponseWriter, r *http.Request) {})
	r.Post("/records", func(w http.ResponseWriter, r *http.Request) {})
	r.Post("/adjust_endpoints", func(w http.ResponseWriter, r *http.Request) {})
}

func logWithReqInfo(r *http.Request) *log.Entry {
	return log.WithFields(log.Fields{"method": r.Method, "url": r.URL.Path})
}

func checkHeaders(w http.ResponseWriter, r *http.Request) error {
	var header string
	if r.Method == http.MethodPost {
		header = r.Header.Get(HeaderContentType)
	} else {
		header = r.Header.Get(HeaderAccept)
	}

	if len(header) == 0 {
		w.Header().Set(HeaderContentType, ContentTypePlainText)
		w.WriteHeader(http.StatusBadRequest)

		var err error
		if r.Method == http.MethodPost {
			err = errors.New("'Content-Type' header is required")
		} else {
			err = errors.New("'Accept' header is required")
		}
		if _, er := fmt.Fprint(w, err); err != nil {
			logWithReqInfo(r).WithField(logKeyError, er).Fatal("got error on writing error message to response writer")
		}
		return err
	}

	if header != supportedMediaVersions {
		w.Header().Set(HeaderContentType, ContentTypePlainText)
		w.WriteHeader(http.StatusUnsupportedMediaType)

		var err error
		if r.Method == http.MethodPost {
			err = errors.New("valid media type is required in 'Content-Type' header")
		} else {
			err = errors.New("valid media type is required in 'Accept' header")
		}
		if _, er := fmt.Fprint(w, err); err != nil {
			logWithReqInfo(r).WithField(logKeyError, er).Fatal("got error on writing error message to response writer")
		}
		return err
	}

	return nil
}
