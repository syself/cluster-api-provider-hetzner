/*
Copyright 2026 The Kubernetes Authors.

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

package hcloudclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func TestWrapUnauthorized(t *testing.T) {
	unauthorizedErr := hcloud.Error{Code: hcloud.ErrorCodeUnauthorized, Message: "unauthorized"}
	otherErr := hcloud.Error{Code: hcloud.ErrorCodeServerError, Message: "server error"}

	if got := wrapUnauthorized(nil); got != nil {
		t.Errorf("wrapUnauthorized(nil) = %v, want nil", got)
	}

	if got := wrapUnauthorized(unauthorizedErr); !errors.Is(got, ErrUnauthorized) {
		t.Errorf("wrapUnauthorized(%v) = %v, want errors.Is match with ErrUnauthorized", unauthorizedErr, got)
	}

	// A caller-added message must still match.
	wrappedWithContext := fmt.Errorf("something failed for %d: %w", 42, unauthorizedErr)
	if got := wrapUnauthorized(wrappedWithContext); !errors.Is(got, ErrUnauthorized) {
		t.Errorf("wrapUnauthorized(%v) = %v, want errors.Is match with ErrUnauthorized", wrappedWithContext, got)
	}

	if got := wrapUnauthorized(otherErr); !errors.Is(got, otherErr) || errors.Is(got, ErrUnauthorized) {
		t.Errorf("wrapUnauthorized(%v) = %v, want err unchanged and no ErrUnauthorized match", otherErr, got)
	}
}

// TestAllMethodsWrapUnauthorized calls every method of the Client interface against a server that
// answers 401. A method that does not wrap the error with wrapUnauthorized fails the test.
func TestAllMethodsWrapUnauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"unauthorized"}}`))
	}))
	defer ts.Close()

	var c Client = &realClient{client: hcloud.NewClient(
		hcloud.WithEndpoint(ts.URL),
		hcloud.WithToken("token"),
		hcloud.WithPollOpts(hcloud.PollOpts{BackoffFunc: hcloud.ConstantBackoff(0)}),
	)}

	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	errType := reflect.TypeOf((*error)(nil)).Elem()
	v := reflect.ValueOf(c)
	for i := 0; i < v.NumMethod(); i++ {
		name := v.Type().Method(i).Name
		m := v.Method(i)
		args := make([]reflect.Value, m.Type().NumIn())
		for j := range args {
			at := m.Type().In(j)
			switch {
			case at == ctxType:
				args[j] = reflect.ValueOf(context.Background())
			case at.Kind() == reflect.Ptr:
				args[j] = reflect.New(at.Elem())
				fillNilPointers(args[j].Elem(), 3)
			case at.Kind() == reflect.String:
				args[j] = reflect.ValueOf("x").Convert(at)
			case at.Kind() == reflect.Struct:
				args[j] = reflect.New(at).Elem()
				fillNilPointers(args[j], 3)
			default:
				args[j] = reflect.Zero(at)
			}
		}
		out := m.Call(args)
		last := out[len(out)-1]
		if last.Type() != errType {
			t.Fatalf("%s: last return value is not an error", name)
		}
		err, _ := last.Interface().(error)
		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("%s: err = %v, want errors.Is match with ErrUnauthorized", name, err)
		}
	}
}

// fillNilPointers sets string fields of v to "x" and nil struct pointer fields to non-nil zero
// values, so that the hcloud SDK accepts the opts and can dereference them while building the
// request.
func fillNilPointers(v reflect.Value, depth int) {
	if depth == 0 || v.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.CanSet() && f.Kind() == reflect.String {
			f.SetString("x")
		}
		// Datacenter is mutually exclusive with Location in ServerCreateOpts.
		if v.Type().Field(i).Name == "Datacenter" {
			continue
		}
		if !f.CanSet() || f.Kind() != reflect.Ptr || f.Type().Elem().Kind() != reflect.Struct {
			continue
		}
		f.Set(reflect.New(f.Type().Elem()))
		fillNilPointers(f.Elem(), depth-1)
	}
}
