/*
 * Minio Cloud Storage, (C) 2014 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"net/http"

	router "github.com/gorilla/mux"
	jsonrpc "github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json"
	"github.com/minio/minio-xl/pkg/donut"
)

// registerAPI - register all the object API handlers to their respective paths
func registerAPI(mux *router.Router, a API) {
	mux.HandleFunc("/", a.ListBucketsHandler).Methods("GET")
	mux.HandleFunc("/{bucket}", a.GetBucketACLHandler).Queries("acl", "").Methods("GET")
	mux.HandleFunc("/{bucket}", a.ListObjectsHandler).Methods("GET")
	mux.HandleFunc("/{bucket}", a.PutBucketACLHandler).Queries("acl", "").Methods("PUT")
	mux.HandleFunc("/{bucket}", a.PutBucketHandler).Methods("PUT")
	mux.HandleFunc("/{bucket}", a.HeadBucketHandler).Methods("HEAD")
	mux.HandleFunc("/{bucket}", a.PostPolicyBucketHandler).Methods("POST")
	mux.HandleFunc("/{bucket}/{object:.*}", a.HeadObjectHandler).Methods("HEAD")
	mux.HandleFunc("/{bucket}/{object:.*}", a.PutObjectPartHandler).Queries("partNumber", "{partNumber:[0-9]+}", "uploadId", "{uploadId:.*}").Methods("PUT")
	mux.HandleFunc("/{bucket}/{object:.*}", a.ListObjectPartsHandler).Queries("uploadId", "{uploadId:.*}").Methods("GET")
	mux.HandleFunc("/{bucket}/{object:.*}", a.CompleteMultipartUploadHandler).Queries("uploadId", "{uploadId:.*}").Methods("POST")
	mux.HandleFunc("/{bucket}/{object:.*}", a.NewMultipartUploadHandler).Methods("POST")
	mux.HandleFunc("/{bucket}/{object:.*}", a.AbortMultipartUploadHandler).Queries("uploadId", "{uploadId:.*}").Methods("DELETE")
	mux.HandleFunc("/{bucket}/{object:.*}", a.GetObjectHandler).Methods("GET")
	mux.HandleFunc("/{bucket}/{object:.*}", a.PutObjectHandler).Methods("PUT")

	// not implemented yet
	mux.HandleFunc("/{bucket}", a.DeleteBucketHandler).Methods("DELETE")

	// unsupported API
	mux.HandleFunc("/{bucket}/{object:.*}", a.DeleteObjectHandler).Methods("DELETE")
}

// APIOperation container for individual operations read by Ticket Master
type APIOperation struct {
	ProceedCh chan struct{}
}

// API container for API and also carries OP (operation) channel
type API struct {
	OP        chan APIOperation
	Donut     donut.Interface
	Anonymous bool // do not checking for incoming signatures, allow all requests
}

// getNewAPI instantiate a new minio API
func getNewAPI(anonymous bool) API {
	// ignore errors for now
	d, err := donut.New()
	fatalIf(err.Trace(), "Instantiating donut failed.", nil)

	return API{
		OP:        make(chan APIOperation),
		Donut:     d,
		Anonymous: anonymous,
	}
}

func getAPIHandler(anonymous bool, api API) http.Handler {
	var mwHandlers = []MiddlewareHandler{
		TimeValidityHandler,
		IgnoreResourcesHandler,
		CorsHandler,
	}
	if !anonymous {
		mwHandlers = append(mwHandlers, SignatureHandler)
	}
	mux := router.NewRouter()
	registerAPI(mux, api)
	apiHandler := registerCustomMiddleware(mux, mwHandlers...)
	return apiHandler
}

func getServerRPCHandler(anonymous bool) http.Handler {
	var mwHandlers = []MiddlewareHandler{
		TimeValidityHandler,
	}
	if !anonymous {
		mwHandlers = append(mwHandlers, RPCSignatureHandler)
	}

	s := jsonrpc.NewServer()
	s.RegisterCodec(json.NewCodec(), "application/json")
	s.RegisterService(new(serverRPCService), "Server")
	s.RegisterService(new(donutRPCService), "Donut")
	mux := router.NewRouter()
	mux.Handle("/rpc", s)

	rpcHandler := registerCustomMiddleware(mux, mwHandlers...)
	return rpcHandler
}

// getControllerRPCHandler rpc handler for controller
func getControllerRPCHandler(anonymous bool) http.Handler {
	var mwHandlers = []MiddlewareHandler{
		TimeValidityHandler,
	}
	if !anonymous {
		mwHandlers = append(mwHandlers, RPCSignatureHandler)
	}

	s := jsonrpc.NewServer()
	codec := json.NewCodec()
	s.RegisterCodec(codec, "application/json")
	s.RegisterCodec(codec, "application/json; charset=UTF-8")
	s.RegisterService(new(controllerRPCService), "Controller")
	mux := router.NewRouter()
	// Add new RPC services here
	mux.Handle("/rpc", s)
	mux.Handle("/{file:.*}", http.FileServer(assetFS()))

	rpcHandler := registerCustomMiddleware(mux, mwHandlers...)
	return rpcHandler
}
