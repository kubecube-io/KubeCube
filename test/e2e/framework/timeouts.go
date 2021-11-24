/*
Copyright 2021 KubeCube Authors

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

package framework

import "time"

const (
	// Default timeouts to be used in TimeoutContext
	httpRequestTimeout = 1 * time.Minute
	WaitInterval       = 2 * time.Second
	WaitTimeout        = 120 * time.Second
)

// TimeoutContext contains timeout settings for several actions.
type TimeoutContext struct {
	// HttpRequest is how long to wait for the http request
	HttpRequest time.Duration

	// ResourceCreate is how long to wait for resource creation
	// Use it in case create fail and hold
	WaitInterval time.Duration

	// ResourceCreate is how long to wait for resource delete
	// Use it in case delete fail and hold
	WaitTimeout time.Duration
}

// NewTimeoutContextWithDefaults returns a TimeoutContext with default values.
func NewTimeoutContextWithDefaults() *TimeoutContext {
	return &TimeoutContext{
		HttpRequest:  httpRequestTimeout,
		WaitInterval: WaitInterval,
		WaitTimeout:  WaitTimeout,
	}
}
