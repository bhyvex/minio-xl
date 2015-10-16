/*
 * Minio Cloud Storage, (C) 2015 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implieapi.Donut.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/minio/minio-xl/pkg/donut"
	"github.com/minio/minio-xl/pkg/probe"
	signv4 "github.com/minio/minio-xl/pkg/signature"
)

const (
	maxPartsList = 1000
)

// GetObjectHandler - GET Object
// ----------
// This implementation of the GET operation retrieves object. To use GET,
// you must have READ access to the object.
func (api API) GetObjectHandler(w http.ResponseWriter, req *http.Request) {
	// ticket master block
	{
		op := APIOperation{}
		op.ProceedCh = make(chan struct{})
		api.OP <- op
		// block until Ticket master gives us a go
		<-op.ProceedCh
	}

	var object, bucket string
	vars := mux.Vars(req)
	bucket = vars["bucket"]
	object = vars["object"]

	metadata, err := api.Donut.GetObjectMetadata(bucket, object)
	if err != nil {
		errorIf(err.Trace(), "GetObject failed.", nil)
		switch err.ToGoError().(type) {
		case donut.BucketNameInvalid:
			writeErrorResponse(w, req, InvalidBucketName, req.URL.Path)
		case donut.BucketNotFound:
			writeErrorResponse(w, req, NoSuchBucket, req.URL.Path)
		case donut.ObjectNotFound:
			writeErrorResponse(w, req, NoSuchKey, req.URL.Path)
		case donut.ObjectNameInvalid:
			writeErrorResponse(w, req, NoSuchKey, req.URL.Path)
		default:
			writeErrorResponse(w, req, InternalError, req.URL.Path)
		}
		return
	}
	var hrange *httpRange
	hrange, err = getRequestedRange(req.Header.Get("Range"), metadata.Size)
	if err != nil {
		writeErrorResponse(w, req, InvalidRange, req.URL.Path)
		return
	}
	setObjectHeaders(w, metadata, hrange)
	if _, err = api.Donut.GetObject(w, bucket, object, hrange.start, hrange.length); err != nil {
		errorIf(err.Trace(), "GetObject failed.", nil)
		return
	}
}

// HeadObjectHandler - HEAD Object
// -----------
// The HEAD operation retrieves metadata from an object without returning the object itself.
func (api API) HeadObjectHandler(w http.ResponseWriter, req *http.Request) {
	// ticket master block
	{
		op := APIOperation{}
		op.ProceedCh = make(chan struct{})
		api.OP <- op
		// block until Ticket master gives us a go
		<-op.ProceedCh
	}

	var object, bucket string
	vars := mux.Vars(req)
	bucket = vars["bucket"]
	object = vars["object"]

	metadata, err := api.Donut.GetObjectMetadata(bucket, object)
	if err != nil {
		errorIf(err.Trace(), "GetObjectMetadata failed.", nil)
		switch err.ToGoError().(type) {
		case donut.BucketNameInvalid:
			writeErrorResponse(w, req, InvalidBucketName, req.URL.Path)
		case donut.BucketNotFound:
			writeErrorResponse(w, req, NoSuchBucket, req.URL.Path)
		case donut.ObjectNotFound:
			writeErrorResponse(w, req, NoSuchKey, req.URL.Path)
		case donut.ObjectNameInvalid:
			writeErrorResponse(w, req, NoSuchKey, req.URL.Path)
		default:
			writeErrorResponse(w, req, InternalError, req.URL.Path)
		}
		return
	}
	setObjectHeaders(w, metadata, nil)
	w.WriteHeader(http.StatusOK)
}

// PutObjectHandler - PUT Object
// ----------
// This implementation of the PUT operation adds an object to a bucket.
func (api API) PutObjectHandler(w http.ResponseWriter, req *http.Request) {
	// Ticket master block
	{
		op := APIOperation{}
		op.ProceedCh = make(chan struct{})
		api.OP <- op
		// block until Ticket master gives us a go
		<-op.ProceedCh
	}

	var object, bucket string
	vars := mux.Vars(req)
	bucket = vars["bucket"]
	object = vars["object"]

	// get Content-MD5 sent by client and verify if valid
	md5 := req.Header.Get("Content-MD5")
	if !isValidMD5(md5) {
		writeErrorResponse(w, req, InvalidDigest, req.URL.Path)
		return
	}
	/// if Content-Length missing, deny the request
	size := req.Header.Get("Content-Length")
	if size == "" {
		writeErrorResponse(w, req, MissingContentLength, req.URL.Path)
		return
	}
	/// maximum Upload size for objects in a single operation
	if isMaxObjectSize(size) {
		writeErrorResponse(w, req, EntityTooLarge, req.URL.Path)
		return
	}
	/// minimum Upload size for objects in a single operation
	//
	// Surprisingly while Amazon in their document states that S3 objects have 1byte
	// as the minimum limit, they do not seem to enforce it one can successfully
	// create a 0byte file using a regular putObject() operation
	//
	// if isMinObjectSize(size) {
	//      writeErrorResponse(w, req, EntityTooSmall,  req.URL.Path)
	//	return
	// }
	var sizeInt64 int64
	{
		var err error
		sizeInt64, err = strconv.ParseInt(size, 10, 64)
		if err != nil {
			writeErrorResponse(w, req, InvalidRequest, req.URL.Path)
			return
		}
	}

	var signature *signv4.Signature
	if !api.Anonymous {
		if _, ok := req.Header["Authorization"]; ok {
			// Init signature V4 verification
			var err *probe.Error
			signature, err = initSignatureV4(req)
			if err != nil {
				errorIf(err.Trace(), "Initializing signature v4 failed.", nil)
				writeErrorResponse(w, req, InternalError, req.URL.Path)
				return
			}
		}
	}

	metadata, err := api.Donut.CreateObject(bucket, object, md5, sizeInt64, req.Body, nil, signature)
	if err != nil {
		errorIf(err.Trace(), "CreateObject failed.", nil)
		switch err.ToGoError().(type) {
		case donut.BucketNotFound:
			writeErrorResponse(w, req, NoSuchBucket, req.URL.Path)
		case donut.BucketNameInvalid:
			writeErrorResponse(w, req, InvalidBucketName, req.URL.Path)
		case donut.ObjectExists:
			writeErrorResponse(w, req, MethodNotAllowed, req.URL.Path)
		case donut.BadDigest:
			writeErrorResponse(w, req, BadDigest, req.URL.Path)
		case signv4.MissingDateHeader:
			writeErrorResponse(w, req, RequestTimeTooSkewed, req.URL.Path)
		case signv4.DoesNotMatch:
			writeErrorResponse(w, req, SignatureDoesNotMatch, req.URL.Path)
		case donut.IncompleteBody:
			writeErrorResponse(w, req, IncompleteBody, req.URL.Path)
		case donut.EntityTooLarge:
			writeErrorResponse(w, req, EntityTooLarge, req.URL.Path)
		case donut.InvalidDigest:
			writeErrorResponse(w, req, InvalidDigest, req.URL.Path)
		default:
			writeErrorResponse(w, req, InternalError, req.URL.Path)
		}
		return
	}
	w.Header().Set("ETag", metadata.MD5Sum)
	writeSuccessResponse(w)
}

/// Multipart API

// NewMultipartUploadHandler - New multipart upload
func (api API) NewMultipartUploadHandler(w http.ResponseWriter, req *http.Request) {
	// Ticket master block
	{
		op := APIOperation{}
		op.ProceedCh = make(chan struct{})
		api.OP <- op
		// block until Ticket master gives us a go
		<-op.ProceedCh
	}

	if !isRequestUploads(req.URL.Query()) {
		writeErrorResponse(w, req, MethodNotAllowed, req.URL.Path)
		return
	}

	var object, bucket string
	vars := mux.Vars(req)
	bucket = vars["bucket"]
	object = vars["object"]

	uploadID, err := api.Donut.NewMultipartUpload(bucket, object, req.Header.Get("Content-Type"))
	if err != nil {
		errorIf(err.Trace(), "NewMultipartUpload failed.", nil)
		switch err.ToGoError().(type) {
		case donut.ObjectExists:
			writeErrorResponse(w, req, MethodNotAllowed, req.URL.Path)
		default:
			writeErrorResponse(w, req, InternalError, req.URL.Path)
		}
		return
	}

	response := generateInitiateMultipartUploadResponse(bucket, object, uploadID)
	encodedSuccessResponse := encodeSuccessResponse(response)
	// write headers
	setCommonHeaders(w, len(encodedSuccessResponse))
	// write body
	w.Write(encodedSuccessResponse)
}

// PutObjectPartHandler - Upload part
func (api API) PutObjectPartHandler(w http.ResponseWriter, req *http.Request) {
	// Ticket master block
	{
		op := APIOperation{}
		op.ProceedCh = make(chan struct{})
		api.OP <- op
		// block until Ticket master gives us a go
		<-op.ProceedCh
	}

	// get Content-MD5 sent by client and verify if valid
	md5 := req.Header.Get("Content-MD5")
	if !isValidMD5(md5) {
		writeErrorResponse(w, req, InvalidDigest, req.URL.Path)
		return
	}

	/// if Content-Length missing, throw away
	size := req.Header.Get("Content-Length")
	if size == "" {
		writeErrorResponse(w, req, MissingContentLength, req.URL.Path)
		return
	}

	/// maximum Upload size for multipart objects in a single operation
	if isMaxObjectSize(size) {
		writeErrorResponse(w, req, EntityTooLarge, req.URL.Path)
		return
	}

	var sizeInt64 int64
	{
		var err error
		sizeInt64, err = strconv.ParseInt(size, 10, 64)
		if err != nil {
			writeErrorResponse(w, req, InvalidRequest, req.URL.Path)
			return
		}
	}

	vars := mux.Vars(req)
	bucket := vars["bucket"]
	object := vars["object"]

	uploadID := req.URL.Query().Get("uploadId")
	partIDString := req.URL.Query().Get("partNumber")

	var partID int
	{
		var err error
		partID, err = strconv.Atoi(partIDString)
		if err != nil {
			writeErrorResponse(w, req, InvalidPart, req.URL.Path)
			return
		}
	}

	var signature *signv4.Signature
	if !api.Anonymous {
		if _, ok := req.Header["Authorization"]; ok {
			// Init signature V4 verification
			var err *probe.Error
			signature, err = initSignatureV4(req)
			if err != nil {
				errorIf(err.Trace(), "Initializing signature v4 failed.", nil)
				writeErrorResponse(w, req, InternalError, req.URL.Path)
				return
			}
		}
	}

	calculatedMD5, err := api.Donut.CreateObjectPart(bucket, object, uploadID, partID, "", md5, sizeInt64, req.Body, signature)
	if err != nil {
		errorIf(err.Trace(), "CreateObjectPart failed.", nil)
		switch err.ToGoError().(type) {
		case donut.InvalidUploadID:
			writeErrorResponse(w, req, NoSuchUpload, req.URL.Path)
		case donut.ObjectExists:
			writeErrorResponse(w, req, MethodNotAllowed, req.URL.Path)
		case donut.BadDigest:
			writeErrorResponse(w, req, BadDigest, req.URL.Path)
		case signv4.DoesNotMatch:
			writeErrorResponse(w, req, SignatureDoesNotMatch, req.URL.Path)
		case donut.IncompleteBody:
			writeErrorResponse(w, req, IncompleteBody, req.URL.Path)
		case donut.EntityTooLarge:
			writeErrorResponse(w, req, EntityTooLarge, req.URL.Path)
		case donut.InvalidDigest:
			writeErrorResponse(w, req, InvalidDigest, req.URL.Path)
		default:
			writeErrorResponse(w, req, InternalError, req.URL.Path)
		}
		return
	}
	w.Header().Set("ETag", calculatedMD5)
	writeSuccessResponse(w)
}

// AbortMultipartUploadHandler - Abort multipart upload
func (api API) AbortMultipartUploadHandler(w http.ResponseWriter, req *http.Request) {
	// Ticket master block
	{
		op := APIOperation{}
		op.ProceedCh = make(chan struct{})
		api.OP <- op
		// block until Ticket master gives us a go
		<-op.ProceedCh
	}

	vars := mux.Vars(req)
	bucket := vars["bucket"]
	object := vars["object"]

	objectResourcesMetadata := getObjectResources(req.URL.Query())

	err := api.Donut.AbortMultipartUpload(bucket, object, objectResourcesMetadata.UploadID)
	if err != nil {
		errorIf(err.Trace(), "AbortMutlipartUpload failed.", nil)
		switch err.ToGoError().(type) {
		case donut.InvalidUploadID:
			writeErrorResponse(w, req, NoSuchUpload, req.URL.Path)
		default:
			writeErrorResponse(w, req, InternalError, req.URL.Path)
		}
		return
	}
	setCommonHeaders(w, 0)
	w.WriteHeader(http.StatusNoContent)
}

// ListObjectPartsHandler - List object parts
func (api API) ListObjectPartsHandler(w http.ResponseWriter, req *http.Request) {
	// Ticket master block
	{
		op := APIOperation{}
		op.ProceedCh = make(chan struct{})
		api.OP <- op
		// block until Ticket master gives us a go
		<-op.ProceedCh
	}

	objectResourcesMetadata := getObjectResources(req.URL.Query())
	if objectResourcesMetadata.PartNumberMarker < 0 {
		writeErrorResponse(w, req, InvalidPartNumberMarker, req.URL.Path)
		return
	}
	if objectResourcesMetadata.MaxParts < 0 {
		writeErrorResponse(w, req, InvalidMaxParts, req.URL.Path)
		return
	}
	if objectResourcesMetadata.MaxParts == 0 {
		objectResourcesMetadata.MaxParts = maxPartsList
	}

	vars := mux.Vars(req)
	bucket := vars["bucket"]
	object := vars["object"]

	objectResourcesMetadata, err := api.Donut.ListObjectParts(bucket, object, objectResourcesMetadata)
	if err != nil {
		errorIf(err.Trace(), "ListObjectParts failed.", nil)
		switch err.ToGoError().(type) {
		case donut.InvalidUploadID:
			writeErrorResponse(w, req, NoSuchUpload, req.URL.Path)
		default:
			writeErrorResponse(w, req, InternalError, req.URL.Path)
		}
		return
	}
	response := generateListPartsResponse(objectResourcesMetadata)
	encodedSuccessResponse := encodeSuccessResponse(response)
	// write headers
	setCommonHeaders(w, len(encodedSuccessResponse))
	// write body
	w.Write(encodedSuccessResponse)
}

// CompleteMultipartUploadHandler - Complete multipart upload
func (api API) CompleteMultipartUploadHandler(w http.ResponseWriter, req *http.Request) {
	// Ticket master block
	{
		op := APIOperation{}
		op.ProceedCh = make(chan struct{})
		api.OP <- op
		// block until Ticket master gives us a go
		<-op.ProceedCh
	}

	vars := mux.Vars(req)
	bucket := vars["bucket"]
	object := vars["object"]

	objectResourcesMetadata := getObjectResources(req.URL.Query())

	var signature *signv4.Signature
	if !api.Anonymous {
		if _, ok := req.Header["Authorization"]; ok {
			// Init signature V4 verification
			var err *probe.Error
			signature, err = initSignatureV4(req)
			if err != nil {
				errorIf(err.Trace(), "Initializing signature v4 failed.", nil)
				writeErrorResponse(w, req, InternalError, req.URL.Path)
				return
			}
		}
	}

	metadata, err := api.Donut.CompleteMultipartUpload(bucket, object, objectResourcesMetadata.UploadID, req.Body, signature)
	if err != nil {
		errorIf(err.Trace(), "CompleteMultipartUpload failed.", nil)
		switch err.ToGoError().(type) {
		case donut.InvalidUploadID:
			writeErrorResponse(w, req, NoSuchUpload, req.URL.Path)
		case donut.InvalidPart:
			writeErrorResponse(w, req, InvalidPart, req.URL.Path)
		case donut.InvalidPartOrder:
			writeErrorResponse(w, req, InvalidPartOrder, req.URL.Path)
		case signv4.MissingDateHeader:
			writeErrorResponse(w, req, RequestTimeTooSkewed, req.URL.Path)
		case signv4.DoesNotMatch:
			writeErrorResponse(w, req, SignatureDoesNotMatch, req.URL.Path)
		case donut.IncompleteBody:
			writeErrorResponse(w, req, IncompleteBody, req.URL.Path)
		case donut.MalformedXML:
			writeErrorResponse(w, req, MalformedXML, req.URL.Path)
		default:
			writeErrorResponse(w, req, InternalError, req.URL.Path)
		}
		return
	}
	response := generateCompleteMultpartUploadResponse(bucket, object, "", metadata.MD5Sum)
	encodedSuccessResponse := encodeSuccessResponse(response)
	// write headers
	setCommonHeaders(w, len(encodedSuccessResponse))
	// write body
	w.Write(encodedSuccessResponse)
}

/// Delete API

// DeleteBucketHandler - Delete bucket
func (api API) DeleteBucketHandler(w http.ResponseWriter, req *http.Request) {
	error := getErrorCode(MethodNotAllowed)
	w.WriteHeader(error.HTTPStatusCode)
}

// DeleteObjectHandler - Delete object
func (api API) DeleteObjectHandler(w http.ResponseWriter, req *http.Request) {
	error := getErrorCode(MethodNotAllowed)
	w.WriteHeader(error.HTTPStatusCode)
}
