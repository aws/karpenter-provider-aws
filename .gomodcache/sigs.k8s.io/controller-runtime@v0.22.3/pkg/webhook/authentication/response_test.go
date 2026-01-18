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
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	authenticationv1 "k8s.io/api/authentication/v1"
)

var _ = Describe("Authentication Webhook Response Helpers", func() {
	Describe("Authenticated", func() {
		It("should return an 'allowed' response", func() {
			Expect(Authenticated("", authenticationv1.UserInfo{})).To(Equal(
				Response{
					TokenReview: authenticationv1.TokenReview{
						Status: authenticationv1.TokenReviewStatus{
							Authenticated: true,
							User:          authenticationv1.UserInfo{},
						},
					},
				},
			))
		})

		It("should populate a status with a reason when a reason is given", func() {
			Expect(Authenticated("acceptable", authenticationv1.UserInfo{})).To(Equal(
				Response{
					TokenReview: authenticationv1.TokenReview{
						Status: authenticationv1.TokenReviewStatus{
							Authenticated: true,
							User:          authenticationv1.UserInfo{},
							Error:         "acceptable",
						},
					},
				},
			))
		})
	})

	Describe("Unauthenticated", func() {
		It("should return a 'not allowed' response", func() {
			Expect(Unauthenticated("", authenticationv1.UserInfo{})).To(Equal(
				Response{
					TokenReview: authenticationv1.TokenReview{
						Status: authenticationv1.TokenReviewStatus{
							Authenticated: false,
							User:          authenticationv1.UserInfo{},
							Error:         "",
						},
					},
				},
			))
		})

		It("should populate a status with a reason when a reason is given", func() {
			Expect(Unauthenticated("UNACCEPTABLE!", authenticationv1.UserInfo{})).To(Equal(
				Response{
					TokenReview: authenticationv1.TokenReview{
						Status: authenticationv1.TokenReviewStatus{
							Authenticated: false,
							User:          authenticationv1.UserInfo{},
							Error:         "UNACCEPTABLE!",
						},
					},
				},
			))
		})
	})

	Describe("Errored", func() {
		It("should return a unauthenticated response with an error", func() {
			err := errors.New("this is an error")
			expected := Response{
				TokenReview: authenticationv1.TokenReview{
					Status: authenticationv1.TokenReviewStatus{
						Authenticated: false,
						User:          authenticationv1.UserInfo{},
						Error:         err.Error(),
					},
				},
			}
			resp := Errored(err)
			Expect(resp).To(Equal(expected))
		})
	})

	Describe("ReviewResponse", func() {
		It("should populate a status with a Error when a reason is given", func() {
			By("checking that a message is populated for 'allowed' responses")
			Expect(ReviewResponse(true, authenticationv1.UserInfo{}, "acceptable")).To(Equal(
				Response{
					TokenReview: authenticationv1.TokenReview{
						Status: authenticationv1.TokenReviewStatus{
							Authenticated: true,
							User:          authenticationv1.UserInfo{},
							Error:         "acceptable",
						},
					},
				},
			))

			By("checking that a message is populated for 'Unauthenticated' responses")
			Expect(ReviewResponse(false, authenticationv1.UserInfo{}, "UNACCEPTABLE!")).To(Equal(
				Response{
					TokenReview: authenticationv1.TokenReview{
						Status: authenticationv1.TokenReviewStatus{
							Authenticated: false,
							User:          authenticationv1.UserInfo{},
							Error:         "UNACCEPTABLE!",
						},
					},
				},
			))
		})

		It("should return an authentication decision", func() {
			By("checking that it returns an 'allowed' response when allowed is true")
			Expect(ReviewResponse(true, authenticationv1.UserInfo{}, "")).To(Equal(
				Response{
					TokenReview: authenticationv1.TokenReview{
						Status: authenticationv1.TokenReviewStatus{
							Authenticated: true,
							User:          authenticationv1.UserInfo{},
						},
					},
				},
			))

			By("checking that it returns an 'Unauthenticated' response when allowed is false")
			Expect(ReviewResponse(false, authenticationv1.UserInfo{}, "")).To(Equal(
				Response{
					TokenReview: authenticationv1.TokenReview{
						Status: authenticationv1.TokenReviewStatus{
							Authenticated: false,
							User:          authenticationv1.UserInfo{},
						},
					},
				},
			))
		})
	})
})
