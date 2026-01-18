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

package main

import (
	"context"

	v1 "k8s.io/api/authentication/v1"

	"sigs.k8s.io/controller-runtime/pkg/webhook/authentication"
)

// authenticator validates tokenreviews
type authenticator struct {
}

// authenticator admits a request by the token.
func (a *authenticator) Handle(ctx context.Context, req authentication.Request) authentication.Response {
	if req.Spec.Token == "invalid" {
		return authentication.Unauthenticated("invalid is an invalid token", v1.UserInfo{})
	}
	return authentication.Authenticated("", v1.UserInfo{})
}
