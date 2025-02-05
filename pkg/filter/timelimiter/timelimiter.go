/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
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

package timelimiter

import (
	"fmt"
	"net/http"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/util/urlrule"
)

const (
	// Kind is the kind of TimeLimiter.
	Kind          = "TimeLimiter"
	resultTimeout = "timeout"
)

var (
	results    = []string{resultTimeout}
	errTimeout = fmt.Errorf("timeout")
)

func init() {
	httppipeline.Register(&TimeLimiter{})
}

type (
	URLRule struct {
		urlrule.URLRule `yaml:",inline"`
		TimeoutDuration string `yaml:"timeoutDuration" jsonschema:"omitempty,format=duration"`
		timeout         time.Duration
	}

	Spec struct {
		DefaultTimeoutDuration string `yaml:"defaultTimeoutDuration" jsonschema:"omitempty,format=duration"`
		defaultTimeout         time.Duration
		URLs                   []*URLRule `yaml:"urls" jsonschema:"required"`
	}

	TimeLimiter struct {
		filterSpec *httppipeline.FilterSpec
		spec       *Spec
	}
)

// Kind returns the kind of TimeLimiter.
func (tl *TimeLimiter) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of TimeLimiter.
func (tl *TimeLimiter) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of TimeLimiter
func (tl *TimeLimiter) Description() string {
	return "TimeLimiter implements a time limiter for http request."
}

// Results returns the results of TimeLimiter.
func (tl *TimeLimiter) Results() []string {
	return results
}

// Init initializes TimeLimiter.
func (tl *TimeLimiter) Init(filterSpec *httppipeline.FilterSpec) {
	tl.filterSpec, tl.spec = filterSpec, filterSpec.FilterSpec().(*Spec)

	if d := tl.spec.DefaultTimeoutDuration; d != "" {
		tl.spec.defaultTimeout, _ = time.ParseDuration(d)
	} else {
		tl.spec.defaultTimeout = 500 * time.Millisecond
	}

	for _, url := range tl.spec.URLs {
		url.Init()
		if d := url.TimeoutDuration; d != "" {
			url.timeout, _ = time.ParseDuration(d)
		} else {
			url.timeout = tl.spec.defaultTimeout
		}
	}
}

// Inherit inherits previous generation of TimeLimiter.
func (tl *TimeLimiter) Inherit(filterSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	tl.Init(filterSpec)
}

func (tl *TimeLimiter) handle(ctx context.HTTPContext, u *URLRule) string {
	timer := time.AfterFunc(u.timeout, func() {
		ctx.Cancel(errTimeout)
	})

	result := ctx.CallNextHandler("")
	if !timer.Stop() {
		ctx.AddTag("timeLimiter: timed out")
		logger.Infof("time limiter %s timed out on URL(%s)", tl.filterSpec.Name(), u.ID())
		ctx.Response().SetStatusCode(http.StatusRequestTimeout)
		ctx.Response().Std().Header().Set("X-EG-Time-Limiter", "timed-out")
		result = resultTimeout
	}

	return result
}

// Handle handles HTTP request
func (tl *TimeLimiter) Handle(ctx context.HTTPContext) string {
	for _, u := range tl.spec.URLs {
		if u.Match(ctx.Request()) {
			return tl.handle(ctx, u)
		}
	}
	return ctx.CallNextHandler("")
}

// Status returns Status generated by Runtime.
func (tl *TimeLimiter) Status() interface{} {
	return nil
}

// Close closes TimeLimiter.
func (tl *TimeLimiter) Close() {
}
