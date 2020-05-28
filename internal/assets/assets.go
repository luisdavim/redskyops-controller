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

// Generate updated assets

//go:generate mkdir -p kustomizeTemp
//go:generate kustomize build ../../config/install -o kustomizeTemp

// Namespace
//go:generate go run ../generator/generator.go --header ../../hack/boilerplate.go.txt --file kustomizeTemp/~g_v1_namespace_redsky-system.yaml --package assets --output ./namespace.go

// CRD
//go:generate go run ../generator/generator.go --header ../../hack/boilerplate.go.txt --file kustomizeTemp/apiextensions.k8s.io_v1beta1_customresourcedefinition_trials.redskyops.dev.yaml --package assets --output ./trials.go
//go:generate go run ../generator/generator.go --header ../../hack/boilerplate.go.txt --file kustomizeTemp/apiextensions.k8s.io_v1beta1_customresourcedefinition_experiments.redskyops.dev.yaml --package assets --output ./experiments.go

// RBAC
//go:generate go run ../generator/generator.go --header ../../hack/boilerplate.go.txt --file kustomizeTemp/rbac.authorization.k8s.io_v1_clusterrolebinding_redsky-manager-rolebinding.yaml --package assets --output ./role_binding.go
//go:generate go run ../generator/generator.go --header ../../hack/boilerplate.go.txt --file kustomizeTemp/rbac.authorization.k8s.io_v1_clusterrole_redsky-manager-role.yaml --package assets --output ./role.go

// Deployment
//go:generate go run ../generator/generator.go --header ../../hack/boilerplate.go.txt --file kustomizeTemp/apps_v1_deployment_redsky-controller-manager.yaml --package assets --output ./deployment.go

// Cleanup
//go:generate rm -r kustomizeTemp

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"log"

	redskyv1alpha1 "github.com/redskyops/redskyops-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	kyaml "sigs.k8s.io/yaml"
)

type Asset struct {
	data    string
	bytes   []byte
	objects []runtime.Object
}

func (a *Asset) Reader() (io.Reader, error) {
	err := a.decode()
	return bytes.NewReader(a.bytes), err
}

func (a *Asset) String() (string, error) {
	err := a.decode()

	return string(a.bytes), err
}

func (a *Asset) InjectMetadata(namespace string, labels map[string]string) (data string, err error) {
	if err = a.decode(); err != nil {
		return data, err
	}

	if err = a.kubeObjects(); err != nil {
		return data, err
	}

	var buf bytes.Buffer
	frameWriter := json.YAMLFramer.NewFrameWriter(&buf)

	// Update objects with labels and namespace and prefix
	for _, obj := range a.objects {

		// This is ugly but allows us to cheese the defer mechanism so we can
		// marshal the object after we've done all our manipulations
		err = func() error {
			defer func() {
				yamlBytes, err := kyaml.Marshal(obj)
				if err != nil {
					log.Println(err)
				}
				frameWriter.Write(yamlBytes)
			}()

			m, err := meta.Accessor(obj)
			if err != nil {
				return err
			}

			m.SetName(fmt.Sprintf("%s-%s", "redsky", m.GetName()))

			updateLabels(m, labels)

			// Handle these kinds with some special TLC
			switch obj.GetObjectKind().GroupVersionKind().Kind {
			case "ClusterRoleBinding":
				return nil
			case "ClusterRole":
				return nil
			case "Namespace":
				return nil
			case "Deployment":
				// labels for spec.template.metadata
				podMeta := obj.(*appsv1.Deployment).Spec.Template.ObjectMeta
				updateLabels(&podMeta, labels)

				// update image
			}

			m.SetNamespace(namespace)

			updateLabels(m, labels)

			return nil
		}()

		if err != nil {
			return data, err
		}

	}

	// Update stored state
	a.bytes = buf.Bytes()

	return a.String()
}

func (a *Asset) decode() (err error) {
	var (
		decoded []byte
		output  bytes.Buffer
		zr      *gzip.Reader
	)

	// No need to decode again
	if len(a.bytes) > 0 {
		return nil
	}

	decoded, err = base64.StdEncoding.DecodeString(a.data)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(decoded)

	if zr, err = gzip.NewReader(buf); err != nil {
		return err
	}

	if _, err = io.Copy(&output, zr); err != nil {
		return err
	}

	if err = zr.Close(); err != nil {
		return err
	}

	a.bytes = output.Bytes()

	return nil
}

func (a *Asset) kubeObjects() (err error) {
	scheme := runtime.NewScheme()

	groups := []runtime.SchemeBuilder{
		corev1.SchemeBuilder,
		rbacv1.SchemeBuilder,
		appsv1.SchemeBuilder,
		apiext.SchemeBuilder,
		apiextv1beta1.SchemeBuilder,
	}

	for _, builder := range groups {
		builder.AddToScheme(scheme)
	}

	redskyv1alpha1.AddToScheme(scheme)

	codecs := serializer.NewCodecFactory(scheme)

	yReader := yaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(a.bytes)))

	var objs []runtime.Object

	for {
		objBytes, err := yReader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// If document starts with `---` or a leading newline
		// there's nothing for us to do with it, so skip and move on
		// to the next document
		if len(objBytes) <= 1 {
			continue
		}

		decode := codecs.UniversalDeserializer().Decode
		obj, _, err := decode(objBytes, nil, nil)
		if err != nil {
			return err
		}

		objs = append(objs, obj)
	}

	a.objects = objs

	return err
}

func updateLabels(m metav1.Object, labels map[string]string) {
	metaLabels := m.GetLabels()
	if metaLabels == nil {
		metaLabels = make(map[string]string)
	}

	for k, v := range labels {
		metaLabels[k] = v
	}

	m.SetLabels(metaLabels)
}
