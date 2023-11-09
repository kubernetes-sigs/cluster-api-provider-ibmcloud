/*
Copyright 2022 The Kubernetes Authors.

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

package cos

import (
	"fmt"
	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/IBM/ibm-cos-sdk-go/aws/request"
	cosSession "github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
	"golang.org/x/net/http/httpproxy"
	"net"
	"net/http"
	"net/url"
	"time"
)

const IAMEndpoint = "https://iam.cloud.ibm.com/identity/token"

// Service holds the IBM Cloud Resource Controller Service specific information.
type Service struct {
	client *s3.S3
}

// ServiceOptions holds the IBM Cloud Resource Controller Service Options specific information.
type ServiceOptions struct {
	*cosSession.Options
}

func (s *Service) GetBucketByName(name string) (*s3.HeadBucketOutput, error) {
	input := &s3.HeadBucketInput{
		Bucket: &name,
	}
	return s.client.HeadBucket(input)
}

func (s *Service) CreateBucket(input *s3.CreateBucketInput) (*s3.CreateBucketOutput, error) {
	return s.client.CreateBucket(input)
}

func (s *Service) CreateBucketWithContext(ctx aws.Context, input *s3.CreateBucketInput, opts ...request.Option) (*s3.CreateBucketOutput, error) {
	return s.client.CreateBucketWithContext(ctx, input, opts...)
}

func (s *Service) PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	return s.client.PutObject(input)
}

func (s *Service) GetObjectRequest(input *s3.GetObjectInput) (*request.Request, *s3.GetObjectOutput) {
	return s.client.GetObjectRequest(input)
}

func (s *Service) PutPublicAccessBlock(input *s3.PutPublicAccessBlockInput) (*s3.PutPublicAccessBlockOutput, error) {
	return s.client.PutPublicAccessBlock(input)
}

// NewService returns a new service for the IBM Cloud Resource Controller api client.
// TODO(karthik-k-n): pass location as a part of options
func NewService(options ServiceOptions, location, apikey, serviceInstance string) (*Service, error) {
	if options.Options == nil {
		options.Options = &cosSession.Options{}
	}
	serviceEndpoint := fmt.Sprintf("s3.%s.cloud-object-storage.appdomain.cloud", location)

	//TODO(karthik-k-n): handle URL
	options.Config = aws.Config{
		Endpoint: &serviceEndpoint,
		Region:   &location,
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				Proxy: func(req *http.Request) (*url.URL, error) {
					return httpproxy.FromEnvironment().ProxyFunc()(req.URL)
				},
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
					DualStack: true,
				}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
		S3ForcePathStyle: aws.Bool(true),
	}

	// TODO(karthik-k-n): Fix me
	options.Config.Credentials = ibmiam.NewStaticCredentials(aws.NewConfig(), IAMEndpoint, apikey, serviceInstance)

	//options.Config.Credentials = ibmiam.NewEnvCredentials(aws.NewConfig())

	sess, err := cosSession.NewSessionWithOptions(*options.Options)
	if err != nil {
		return nil, err
	}
	return &Service{
		client: s3.New(sess),
	}, nil
}
