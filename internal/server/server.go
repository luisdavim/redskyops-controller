/*
Copyright 2019 GramLabs, Inc.

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

package server

import (
	"encoding/json"
	"strconv"

	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	redskyapi "github.com/redskyops/k8s-experiment/redskyapi/redsky/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const (
	// Finalizer is used to ensure synchronization with the server
	Finalizer = "serverFinalizer.redskyops.dev"
)

// TODO Split this into trial.go and experiment.go ?

// FromCluster converts cluster state to API state
func FromCluster(in *redskyv1alpha1.Experiment) (redskyapi.ExperimentName, *redskyapi.Experiment) {
	out := &redskyapi.Experiment{}
	out.ExperimentMeta.LastModified = in.CreationTimestamp.Time
	out.ExperimentMeta.Self = in.Annotations[redskyv1alpha1.AnnotationExperimentURL]
	out.ExperimentMeta.NextTrial = in.Annotations[redskyv1alpha1.AnnotationNextTrialURL]

	out.Optimization = redskyapi.Optimization{}
	if in.Spec.Parallelism != nil {
		out.Optimization.ParallelTrials = *in.Spec.Parallelism
	}
	if in.Spec.BurnIn != nil {
		out.Optimization.BurnIn = *in.Spec.BurnIn
	}
	if in.Spec.Budget != nil {
		out.Optimization.ExperimentBudget = *in.Spec.Budget
	}

	out.Parameters = nil
	for _, p := range in.Spec.Parameters {
		// This is a special case for testing
		if p.Min == p.Max {
			continue
		}

		out.Parameters = append(out.Parameters, redskyapi.Parameter{
			Type: redskyapi.ParameterTypeInteger,
			Name: p.Name,
			Bounds: redskyapi.Bounds{
				Min: json.Number(strconv.FormatInt(p.Min, 10)),
				Max: json.Number(strconv.FormatInt(p.Max, 10)),
			},
		})
	}

	out.Metrics = nil
	for _, m := range in.Spec.Metrics {
		out.Metrics = append(out.Metrics, redskyapi.Metric{
			Name:     m.Name,
			Minimize: m.Minimize,
		})
	}

	n := redskyapi.NewExperimentName(in.Name)
	return n, out
}

// ToCluster converts API state to cluster state
func ToCluster(exp *redskyv1alpha1.Experiment, ee *redskyapi.Experiment) {
	if exp.GetAnnotations() == nil {
		exp.SetAnnotations(make(map[string]string))
	}

	exp.GetAnnotations()[redskyv1alpha1.AnnotationExperimentURL] = ee.Self
	exp.GetAnnotations()[redskyv1alpha1.AnnotationNextTrialURL] = ee.NextTrial

	if exp.Replicas() > ee.Optimization.ParallelTrials && ee.Optimization.ParallelTrials > 0 {
		exp.Spec.Replicas = &ee.Optimization.ParallelTrials
	}

	if exp.Spec.Budget == nil && ee.Optimization.ExperimentBudget > 0 {
		exp.Spec.Budget = &ee.Optimization.ExperimentBudget
	}

	if exp.Spec.BurnIn == nil && ee.Optimization.BurnIn > 0 {
		exp.Spec.BurnIn = &ee.Optimization.BurnIn
	}
}

// ToClusterTrial converts API state to cluster state
func ToClusterTrial(trial *redskyv1alpha1.Trial, suggestion *redskyapi.TrialAssignments) {
	trial.GetAnnotations()[redskyv1alpha1.AnnotationReportTrialURL] = suggestion.ReportTrial

	for _, a := range suggestion.Assignments {
		if v, err := a.Value.Int64(); err == nil {
			trial.Spec.Assignments = append(trial.Spec.Assignments, redskyv1alpha1.Assignment{
				Name:  a.ParameterName,
				Value: v,
			})
		}
	}
}

// FromClusterTrial converts cluster state to API state
func FromClusterTrial(in *redskyv1alpha1.Trial) *redskyapi.TrialValues {
	out := &redskyapi.TrialValues{}

	// Check to see if the trial failed
	for _, c := range in.Status.Conditions {
		if c.Type == redskyv1alpha1.TrialFailed && c.Status == corev1.ConditionTrue {
			out.Failed = true
		}
	}

	// Record the values only if we didn't fail
	out.Values = nil
	if !out.Failed {
		for _, v := range in.Spec.Values {
			if fv, err := strconv.ParseFloat(v.Value, 64); err == nil {
				value := redskyapi.Value{
					MetricName: v.Name,
					Value:      fv,
				}
				if ev, err := strconv.ParseFloat(v.Error, 64); err == nil {
					value.Error = ev
				}
				out.Values = append(out.Values, value)
			}
		}
	}

	return out
}

// StopExperiment updates the experiment in the event that it should be paused or halted
func StopExperiment(exp *redskyv1alpha1.Experiment, err error) bool {
	if rse, ok := err.(*redskyapi.Error); ok && rse.Type == redskyapi.ErrExperimentStopped {
		exp.SetReplicas(0)
		delete(exp.GetAnnotations(), redskyv1alpha1.AnnotationNextTrialURL)
		return true
	}
	return false
}