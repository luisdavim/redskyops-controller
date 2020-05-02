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

package v1alpha1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTrial(t *testing.T) {
	saveTime := time.Now()
	cases := []struct {
		desc         string
		delaySeconds int32
		conditions   []TrialCondition
		expectedTime time.Duration
	}{
		{
			desc: "no delay",
			conditions: []TrialCondition{
				{
					Type:               TrialReady,
					Status:             corev1.ConditionTrue,
					LastProbeTime:      metav1.NewTime(saveTime),
					LastTransitionTime: metav1.NewTime(saveTime),
				},
			},
			expectedTime: time.Duration(0),
		},
		{
			desc:         "5s delay",
			delaySeconds: 5,
			conditions: []TrialCondition{
				{
					Type:               TrialReady,
					Status:             corev1.ConditionTrue,
					LastProbeTime:      metav1.NewTime(saveTime),
					LastTransitionTime: metav1.NewTime(saveTime),
				},
			},
			expectedTime: time.Duration(5) * time.Second,
		},
		{
			desc:         "delay with no condition",
			delaySeconds: 5,
			conditions:   []TrialCondition{},
			expectedTime: time.Duration(0),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.desc, func(t *testing.T) {
			trial := &Trial{
				Spec: TrialSpec{
					InitialDelaySeconds: testCase.delaySeconds,
				},
				Status: TrialStatus{
					Conditions: testCase.conditions,
				},
			}

			// We'll give it a 1s wiggle room for test
			assert.WithinDuration(
				t,
				saveTime.Add(testCase.expectedTime),
				saveTime.Add(trial.DelayStartTime()),
				time.Duration(1)*time.Second,
			)
		})
	}
}
