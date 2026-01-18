/*
Copyright 2021 The Kubernetes Authors.

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

package authentication

import (
	authenticationv1 "k8s.io/api/authentication/v1"
)

// Authenticated constructs a response indicating that the given token
// is valid.
func Authenticated(reason string, user authenticationv1.UserInfo) Response {
	return ReviewResponse(true, user, reason)
}

// Unauthenticated constructs a response indicating that the given token
// is not valid.
func Unauthenticated(reason string, user authenticationv1.UserInfo) Response {
	return ReviewResponse(false, authenticationv1.UserInfo{}, reason)
}

// Errored creates a new Response for error-handling a request.
func Errored(err error) Response {
	return Response{
		TokenReview: authenticationv1.TokenReview{
			Spec: authenticationv1.TokenReviewSpec{},
			Status: authenticationv1.TokenReviewStatus{
				Authenticated: false,
				Error:         err.Error(),
			},
		},
	}
}

// ReviewResponse returns a response for admitting a request.
func ReviewResponse(authenticated bool, user authenticationv1.UserInfo, err string, audiences ...string) Response {
	resp := Response{
		TokenReview: authenticationv1.TokenReview{
			Status: authenticationv1.TokenReviewStatus{
				Authenticated: authenticated,
				User:          user,
				Audiences:     audiences,
			},
		},
	}
	if len(err) > 0 {
		resp.TokenReview.Status.Error = err
	}
	return resp
}
