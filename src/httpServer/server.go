///////////////////////////////////////////////////////////////////////////
// Copyright 2019 Roku, Inc.
//
//Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//////////////////////////////////////////////////////////////////////////

package httpServer

import (
	"context"
	ecp "driver/ecpClient"
	"driver/logger"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type key int

const (
	requestIDKey key = 0
)

type Server struct {
	router   *mux.Router
	sessions map[string]*SessionInfo
}

type SessionInfo struct {
	client     *ecp.EcpClient
	plugin     *ecp.PluginClient
	capability *Capability
	pressDelay time.Duration
}

func GetServerInstance() *Server {
	server := &Server{
		router:   mux.NewRouter(),
		sessions: make(map[string]*SessionInfo),
	}

	return server
}

func (s *Server) Start(port string, logPath string) {
	router := http.NewServeMux()
	s.SetUpRoutes(router)
	// TODO: add os.create
	logger := logger.NewLogger(logPath, true)

	nextRequestID := func() string {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      tracing(nextRequestID)(logging(nextRequestID, logger)(router)),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
	err := server.ListenAndServe()
	// err := http.ListenAndServe(":"+port, nil)
	if err != http.ErrServerClosed {
		logrus.WithError(err).Error("Http Server stopped unexpected")
	} else {
		logrus.WithError(err).Info("Http Server stopped")
	}
}

func logging(nextRequestID func() string, logger *logger.Golanglogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				// requestID, ok := r.Context().Value(requestIDKey).(string)
				// if !ok {
				// 	requestID = "unknown"
				// }
				logger.Info(nextRequestID(), r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func tracing(nextRequestID func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-Id")
			if requestID == "" {
				requestID = nextRequestID()
			}
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			w.Header().Set("X-Request-Id", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
