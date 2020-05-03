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
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ExperimentNamespacedName returns the namespaced name of the experiment for this trial
func (in *Trial) ExperimentNamespacedName() types.NamespacedName {
	nn := types.NamespacedName{Namespace: in.Namespace, Name: in.Name}
	if in.Labels[LabelExperiment] != "" {
		nn.Name = in.Labels[LabelExperiment]
	}
	if in.Spec.ExperimentRef != nil {
		if in.Spec.ExperimentRef.Namespace != "" {
			nn.Namespace = in.Spec.ExperimentRef.Namespace
		}
		if in.Spec.ExperimentRef.Name != "" {
			nn.Name = in.Spec.ExperimentRef.Name
		}
	}
	return nn
}

// Checks to see if the trial has an initializer
func (in *Trial) HasInitializer() bool {
	return strings.TrimSpace(in.GetAnnotations()[AnnotationInitializer]) != ""
}

// Returns an assignment value by name
func (in *Trial) GetAssignment(name string) (int64, bool) {
	for i := range in.Spec.Assignments {
		if in.Spec.Assignments[i].Name == name {
			return in.Spec.Assignments[i].Value, true
		}
	}
	return 0, false
}

// Returns the job selector
func (in *Trial) GetJobSelector() *metav1.LabelSelector {
	if in.Spec.Selector != nil {
		return in.Spec.Selector
	}

	if in.Spec.Template != nil && len(in.Spec.Template.Labels) > 0 {
		return &metav1.LabelSelector{MatchLabels: in.Spec.Template.Labels}
	}

	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			LabelTrial:     in.Name,
			LabelTrialRole: "trialRun",
		},
	}
}

func (in *Trial) DelayStartTime() time.Duration {
	if in.Spec.InitialDelaySeconds == 0 {
		return 0
	}

	delay := time.Duration(in.Spec.InitialDelaySeconds) * time.Second
	var expireTime time.Duration

	for _, c := range in.Status.Conditions {
		if c.Type != TrialReady {
			continue
		}

		delayedStartTime := c.LastTransitionTime.Add(delay)

		if time.Now().Before(delayedStartTime) {
			expireTime = delayedStartTime.Sub(time.Now())
		}
	}

	return expireTime
}

// IsFinished checks to see if the specified trial is finished
func (in *Trial) IsFinished() bool {
	for _, c := range in.Status.Conditions {
		if c.Status == corev1.ConditionTrue {
			if c.Type == TrialComplete || c.Type == TrialFailed {
				return true
			}
		}
	}
	return false
}

// IsAbandoned checks to see if the specified trial is abandoned
func (in *Trial) IsAbandoned() bool {
	return !in.IsFinished() && !in.GetDeletionTimestamp().IsZero()
}

// IsActive checks to see if the specified trial and any setup delete tasks are NOT finished
func (in *Trial) IsActive() bool {
	// Not finished, definitely active
	if !in.IsFinished() {
		return true
	}

	// Check if a setup delete task exists and has not yet completed (remember the TrialSetupDeleted status is optional!)
	for _, c := range in.Status.Conditions {
		if c.Type == TrialSetupDeleted && c.Status != corev1.ConditionTrue {
			return true
		}
	}

	return false
}

// NeedsCleanup checks to see if a trial's TTL has expired
func (in *Trial) NeedsCleanup() bool {
	// Already deleted or still active, no cleanup necessary
	if !in.GetDeletionTimestamp().IsZero() || in.IsActive() {
		return false
	}

	// Try to determine effective finish time and TTL
	finishTime := metav1.Time{}
	ttlSeconds := in.Spec.TTLSecondsAfterFinished
	for _, c := range in.Status.Conditions {
		if isFinishTimeCondition(c) {
			// Adjust the TTL if specified separately for failures
			if c.Type == TrialFailed && in.Spec.TTLSecondsAfterFailure != nil {
				ttlSeconds = in.Spec.TTLSecondsAfterFailure
			}

			// Take the latest time possible
			if finishTime.Before(&c.LastTransitionTime) {
				finishTime = c.LastTransitionTime
			}
		}
	}

	// No finish time or TTL, no cleanup necessary
	if finishTime.IsZero() || ttlSeconds == nil || *ttlSeconds < 0 {
		return false
	}

	// Check to see if we are still in the TTL window
	ttl := time.Duration(*ttlSeconds) * time.Second
	return finishTime.UTC().Add(ttl).Before(time.Now().UTC())
}

// isFinishTimeCondition returns true if the condition is relevant to the "finish time"
func isFinishTimeCondition(c TrialCondition) bool {
	switch c.Type {
	case TrialComplete, TrialFailed, TrialSetupDeleted:
		return c.Status == corev1.ConditionTrue
	default:
		return false
	}
}

// AppendAssignmentEnv appends an environment variable for each trial assignment
func (in *Trial) AssignmentEnv() (env []corev1.EnvVar) {
	for _, a := range in.Spec.Assignments {
		name := strings.ReplaceAll(strings.ToUpper(a.Name), ".", "_")
		env = append(env, corev1.EnvVar{Name: name, Value: fmt.Sprintf("%d", a.Value)})
	}

	return env
}

// IsTrialJobReference checks to see if the supplied reference likely points to the job of a trial. This is
// used primarily to give special handling to patch operations so they can refer to trial job before it exists.
func (in *Trial) IsTrialJobReference(ref *corev1.ObjectReference) bool {
	// Kind _must_ be job
	if ref.Kind != "Job" {
		return false
	}

	// Allow version to be omitted for compatibility with old job definitions
	if ref.APIVersion != "" && ref.APIVersion != "batch/v1" {
		return false
	}

	// Allow namespace to be omitted for trials that run in multiple namespaces
	if ref.Namespace != "" && ref.Namespace != in.Namespace {
		return false
	}

	// If the trial job template has name, it must match...
	if in.Spec.Template != nil && in.Spec.Template.Name != "" {
		return in.Spec.Template.Name != ref.Name
	}

	// ...otherwise the trial name must match by prefix
	if !strings.HasPrefix(in.Name, ref.Name) {
		return false
	}

	return true
}
