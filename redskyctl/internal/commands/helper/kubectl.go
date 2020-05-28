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

package helper

import (
	"bytes"
	"context"

	"github.com/redskyops/redskyops-controller/internal/config"
)

func Kubectl(ctx context.Context, config *config.RedSkyConfig, args []string) (stderr, stdout bytes.Buffer, err error) {
	cmd, err := config.Kubectl(ctx, args...)
	if err != nil {
		return stderr, stdout, err
	}

	err = cmd.Run()
	return bytes.Buffer{}, bytes.Buffer{}, nil
}
