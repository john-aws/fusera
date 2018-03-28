// Copyright 2018 The MITRE Corporation
// Author Matthew Bianchi
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsutil

import (
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jacobsa/fuse"
	"github.com/mattrbianchi/twig"
	"github.com/pkg/errors"
)

// Makes an http HEAD request using the URL provided.
// URL should either point to a public obejct or be
// a signed URL giving the user GET permissions.
func HeadObject(url string) (*http.Response, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Makes an http GET request using the URL provided.
// URL should either point to a public obejct or be
// a signed URL giving the user GET permissions.
// url: full url path to the object on AWS
func GetObject(url string) (*http.Response, error) {
	resp, err := GetObjectRange(url, "")
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Makes a ranged http GET request using the URL and byteRange provided.
// URL should either point to a public obejct or be
// a signed URL giving the user GET permissions.
// url: full url path to the object on AWS
// byteRange: the desired range of bytes of a file
// Should resemble the format for an http header Range.
// Example: "bytes="0-1000"
// Example: "bytes="1000-"
func GetObjectRange(url, byteRange string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if byteRange != "" {
		req.Header.Add("Range", byteRange)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		twig.Debugf("status code: %d\n", resp.StatusCode)
		return nil, parseHTTPError(resp.StatusCode)
	}
	return resp, nil
}

// Expects the url to point to a valid ngc file.
// Uses the aws-sdk to read the file, assuming that
// this file will not be publicly accessible and will
// need to utilize aws credentials on the machine.
func ReadNgcFile(path string) ([]byte, error) {
	// Users should be using virtual-hosted style:
	// http://[bucket].s3.amazonaws.com/[file]
	if !strings.Contains(path, "s3.amazonaws.com") {
		return nil, errors.Errorf("url did not point to a valid amazon s3 location or follow the virtual-hosted style of https://[bucket].[region].s3.amazonzws.com/[file]: %s", path)
	}
	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	sections := strings.Split(u.Hostname(), ".")
	if len(sections) < 5 {
		return nil, errors.Errorf("url did not point to a valid amazon s3 location or follow the virtual-hosted style of https://[bucket].[region].s3.amazonzws.com/[file]: %s", path)
	}
	bucket := sections[0]
	twig.Debugf("bucket: %s", bucket)
	region := sections[1]
	twig.Debugf("region: %s", region)
	file := u.Path
	twig.Debugf("file: %s", file)
	cfg := (&aws.Config{
		Region: &region,
	}).WithHTTPClient(&http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: 15 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          1000,
			MaxIdleConnsPerHost:   1000,
			IdleConnTimeout:       20 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 10 * time.Second,
		},
	})
	sess := session.New(cfg)
	svc := s3.New(sess)
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(file),
	}
	obj, err := svc.GetObject(input)
	if err != nil {
		twig.Debug("error from GetObject")
		return nil, err
	}
	bytes, err := ioutil.ReadAll(obj.Body)
	return bytes, err
}

func parseHTTPError(code int) error {
	switch code {
	case 400:
		twig.Debug("converting to EINVAL")
		return fuse.EINVAL
	case 403:
		twig.Debug("converting to EACCES")
		return syscall.EACCES
	case 404:
		twig.Debug("converting to ENOENT")
		return fuse.ENOENT
	case 405:
		twig.Debug("converting to ENOTSUP")
		return syscall.ENOTSUP
	case 500:
		twig.Debug("converting to EAGAIN")
		return syscall.EAGAIN
	default:
		// TODO: log this and re-evaluate whether this is a good move.
		twig.Debug("converting to EOF")
		return io.EOF
	}
}

func String(s string) *string {
	return &s
}

func Int64Value(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}
