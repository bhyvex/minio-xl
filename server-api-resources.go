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
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"net/url"
	"strconv"

	"github.com/minio/minio-xl/pkg/donut"
)

// parse bucket url queries
func getBucketResources(values url.Values) (v donut.BucketResourcesMetadata) {
	v.Prefix = values.Get("prefix")
	v.Marker = values.Get("marker")
	v.Maxkeys, _ = strconv.Atoi(values.Get("max-keys"))
	v.Delimiter = values.Get("delimiter")
	v.EncodingType = values.Get("encoding-type")
	return
}

// part bucket url queries for ?uploads
func getBucketMultipartResources(values url.Values) (v donut.BucketMultipartResourcesMetadata) {
	v.Prefix = values.Get("prefix")
	v.KeyMarker = values.Get("key-marker")
	v.MaxUploads, _ = strconv.Atoi(values.Get("max-uploads"))
	v.Delimiter = values.Get("delimiter")
	v.EncodingType = values.Get("encoding-type")
	v.UploadIDMarker = values.Get("upload-id-marker")
	return
}

// parse object url queries
func getObjectResources(values url.Values) (v donut.ObjectResourcesMetadata) {
	v.UploadID = values.Get("uploadId")
	v.PartNumberMarker, _ = strconv.Atoi(values.Get("part-number-marker"))
	v.MaxParts, _ = strconv.Atoi(values.Get("max-parts"))
	v.EncodingType = values.Get("encoding-type")
	return
}

// check if req quere values carry uploads resource
func isRequestUploads(values url.Values) bool {
	_, ok := values["uploads"]
	return ok
}
