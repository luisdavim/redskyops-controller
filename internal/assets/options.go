/*
Copyright 2020 GramLabs, Inc.

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

package assets

import "github.com/redskyops/redskyops-controller/internal/version"

type Options struct {
	Namespace string
	Labels    map[string]string

	Image string
}

type Option func(*Options) error

func WithNamespace(o string) Option {
	return func(opt *Options) (err error) {
		opt.Namespace = o
		return err
	}
}

func WithLabels(o map[string]string) Option {
	return func(opt *Options) (err error) {
		opt.Labels = o
		return err
	}
}

func WithImage(o string) Option {
	return func(opt *Options) (err error) {
		opt.Image = o
		return err
	}
}

func defaultOptions() *Options {
	return &Options{
		Namespace: "redsky-system",
		Labels: map[string]string{
			"app.kubernetes.io/version":    version.GetInfo().String(),
			"app.kubernetes.io/managed-by": "redskyctl",
			"app.kubernetes.io/name":       "redskyops",
		},
		Image: "gcr.io/redskyops/redskyops-controller" + version.GetInfo().String(),
	}
}
