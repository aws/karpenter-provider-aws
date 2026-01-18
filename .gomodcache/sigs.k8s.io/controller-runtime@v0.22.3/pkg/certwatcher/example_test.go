/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package certwatcher_test

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
)

type sampleServer struct {
}

func Example() {
	// Setup Context
	ctx := ctrl.SetupSignalHandler()

	// Initialize a new cert watcher with cert/key pair
	watcher, err := certwatcher.New("ssl/tls.crt", "ssl/tls.key")
	if err != nil {
		panic(err)
	}

	// Start goroutine with certwatcher running against supplied cert
	go func() {
		if err := watcher.Start(ctx); err != nil {
			panic(err)
		}
	}()

	// Setup TLS listener using GetCertficate for fetching the cert when changes
	listener, err := tls.Listen("tcp", "localhost:9443", &tls.Config{
		GetCertificate: watcher.GetCertificate,
		MinVersion:     tls.VersionTLS12,
	})
	if err != nil {
		panic(err)
	}

	// Initialize your tls server
	srv := &http.Server{
		Handler:           &sampleServer{},
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start goroutine for handling server shutdown.
	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			panic(err)
		}
	}()

	// Serve t
	if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

func (s *sampleServer) ServeHTTP(http.ResponseWriter, *http.Request) {
}
