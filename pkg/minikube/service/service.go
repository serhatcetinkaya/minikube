/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package service

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/docker/machine/libmachine"
	"github.com/golang/glog"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/browser"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	typed_core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/minikube/pkg/minikube/cluster"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/out"
	"k8s.io/minikube/pkg/minikube/proxy"
	"k8s.io/minikube/pkg/util/retry"
)

// K8sClient represents a kubernetes client
type K8sClient interface {
	GetCoreClient() (typed_core.CoreV1Interface, error)
	GetClientset(timeout time.Duration) (*kubernetes.Clientset, error)
}

// K8sClientGetter can get a K8sClient
type K8sClientGetter struct{}

// K8s is the current K8sClient
var K8s K8sClient

func init() {
	K8s = &K8sClientGetter{}
}

// GetCoreClient returns a core client
func (k *K8sClientGetter) GetCoreClient() (typed_core.CoreV1Interface, error) {
	client, err := k.GetClientset(constants.DefaultK8sClientTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "getting clientset")
	}
	return client.CoreV1(), nil
}

// GetClientset returns a clientset
func (*K8sClientGetter) GetClientset(timeout time.Duration) (*kubernetes.Clientset, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	profile := viper.GetString(config.MachineProfile)
	configOverrides := &clientcmd.ConfigOverrides{
		Context: clientcmdapi.Context{
			Cluster:  profile,
			AuthInfo: profile,
		},
	}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	clientConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("kubeConfig: %v", err)
	}
	clientConfig.Timeout = timeout
	clientConfig = proxy.UpdateTransport(clientConfig)
	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "client from config")
	}

	return client, nil
}

// SvcURL represents a service URL. Each item in the URLs field combines the service URL with one of the configured
// node ports. The PortNames field contains the configured names of the ports in the URLs field (sorted correspondingly -
// first item in PortNames belongs to the first item in URLs).
type SvcURL struct {
	Namespace string
	Name      string
	URLs      []string
	PortNames []string
}

// URLs represents a list of URL
type URLs []SvcURL

// GetServiceURLs returns a SvcURL object for every service in a particular namespace.
// Accepts a template for formatting
func GetServiceURLs(api libmachine.API, namespace string, t *template.Template) (URLs, error) {
	host, err := cluster.CheckIfHostExistsAndLoad(api, config.GetMachineName())
	if err != nil {
		return nil, err
	}

	ip, err := host.Driver.GetIP()
	if err != nil {
		return nil, err
	}

	client, err := K8s.GetCoreClient()
	if err != nil {
		return nil, err
	}

	serviceInterface := client.Services(namespace)

	svcs, err := serviceInterface.List(meta.ListOptions{})
	if err != nil {
		return nil, err
	}

	var serviceURLs []SvcURL
	for _, svc := range svcs.Items {
		svcURL, err := printURLsForService(client, ip, svc.Name, svc.Namespace, t)
		if err != nil {
			return nil, err
		}
		serviceURLs = append(serviceURLs, svcURL)
	}

	return serviceURLs, nil
}

// GetServiceURLsForService returns a SvcUrl object for a service in a namespace. Supports optional formatting.
func GetServiceURLsForService(api libmachine.API, namespace, service string, t *template.Template) (SvcURL, error) {
	host, err := cluster.CheckIfHostExistsAndLoad(api, config.GetMachineName())
	if err != nil {
		return SvcURL{}, errors.Wrap(err, "Error checking if api exist and loading it")
	}

	ip, err := host.Driver.GetIP()
	if err != nil {
		return SvcURL{}, errors.Wrap(err, "Error getting ip from host")
	}

	client, err := K8s.GetCoreClient()
	if err != nil {
		return SvcURL{}, err
	}

	return printURLsForService(client, ip, service, namespace, t)
}

func printURLsForService(c typed_core.CoreV1Interface, ip, service, namespace string, t *template.Template) (SvcURL, error) {
	if t == nil {
		return SvcURL{}, errors.New("Error, attempted to generate service url with nil --format template")
	}

	svc, err := c.Services(namespace).Get(service, meta.GetOptions{})
	if err != nil {
		return SvcURL{}, errors.Wrapf(err, "service '%s' could not be found running", service)
	}

	endpoints, err := c.Endpoints(namespace).Get(service, meta.GetOptions{})
	m := make(map[int32]string)
	if err == nil && endpoints != nil && len(endpoints.Subsets) > 0 {
		for _, ept := range endpoints.Subsets {
			for _, p := range ept.Ports {
				m[p.Port] = p.Name
			}
		}
	}

	urls := []string{}
	portNames := []string{}
	for _, port := range svc.Spec.Ports {
		if port.NodePort > 0 {
			var doc bytes.Buffer
			err = t.Execute(&doc, struct {
				IP   string
				Port int32
				Name string
			}{
				ip,
				port.NodePort,
				m[port.TargetPort.IntVal],
			})
			if err != nil {
				return SvcURL{}, err
			}
			urls = append(urls, doc.String())
			portNames = append(portNames, m[port.TargetPort.IntVal])
		}
	}
	return SvcURL{Namespace: svc.Namespace, Name: svc.Name, URLs: urls, PortNames: portNames}, nil
}

// CheckService checks if a service is listening on a port.
func CheckService(namespace string, service string) error {
	client, err := K8s.GetCoreClient()
	if err != nil {
		return errors.Wrap(err, "Error getting kubernetes client")
	}

	svc, err := client.Services(namespace).Get(service, meta.GetOptions{})
	if err != nil {
		return &retry.RetriableError{
			Err: errors.Wrapf(err, "Error getting service %s", service),
		}
	}
	if len(svc.Spec.Ports) == 0 {
		return fmt.Errorf("%s:%s has no ports", namespace, service)
	}
	glog.Infof("Found service: %+v", svc)
	return nil
}

// OptionallyHTTPSFormattedURLString returns a formatted URL string, optionally HTTPS
func OptionallyHTTPSFormattedURLString(bareURLString string, https bool) (string, bool) {
	httpsFormattedString := bareURLString
	isHTTPSchemedURL := false

	if u, parseErr := url.Parse(bareURLString); parseErr == nil {
		isHTTPSchemedURL = u.Scheme == "http"
	}

	if isHTTPSchemedURL && https {
		httpsFormattedString = strings.Replace(bareURLString, "http", "https", 1)
	}

	return httpsFormattedString, isHTTPSchemedURL
}

// PrintServiceList prints a list of services as a table which has
// "Namespace", "Name" and "URL" columns to a writer
func PrintServiceList(writer io.Writer, data [][]string) {
	table := tablewriter.NewWriter(writer)
	table.SetHeader([]string{"Namespace", "Name", "Target Port", "URL"})
	table.SetBorders(tablewriter.Border{Left: true, Top: true, Right: true, Bottom: true})
	table.SetCenterSeparator("|")
	table.AppendBulk(data)
	table.Render()
}

// WaitAndMaybeOpenService waits for a service, and opens it when running
func WaitAndMaybeOpenService(api libmachine.API, namespace string, service string, urlTemplate *template.Template, urlMode bool, https bool,
	wait int, interval int) error {
	// Convert "Amount of time to wait" and "interval of each check" to attempts
	if interval == 0 {
		interval = 1
	}
	chkSVC := func() error { return CheckService(namespace, service) }

	if err := retry.Expo(chkSVC, time.Duration(interval)*time.Second, time.Duration(wait)*time.Second); err != nil {
		return errors.Wrapf(err, "Could not find finalized endpoint being pointed to by %s", service)
	}

	serviceURL, err := GetServiceURLsForService(api, namespace, service, urlTemplate)
	if err != nil {
		return errors.Wrap(err, "Check that minikube is running and that you have specified the correct namespace")
	}

	if !urlMode {
		var data [][]string
		if len(serviceURL.URLs) == 0 {
			data = append(data, []string{namespace, service, "", "No node port"})
		} else {
			data = append(data, []string{namespace, service, strings.Join(serviceURL.PortNames, "\n"), strings.Join(serviceURL.URLs, "\n")})
		}
		PrintServiceList(os.Stdout, data)
	}

	if len(serviceURL.URLs) == 0 {
		out.T(out.Sad, "service {{.namespace_name}}/{{.service_name}} has no node port", out.V{"namespace_name": namespace, "service_name": service})
		return nil
	}

	for _, bareURLString := range serviceURL.URLs {
		urlString, isHTTPSchemedURL := OptionallyHTTPSFormattedURLString(bareURLString, https)

		if urlMode || !isHTTPSchemedURL {
			out.T(out.Empty, urlString)
		} else {
			out.T(out.Celebrate, "Opening kubernetes service  {{.namespace_name}}/{{.service_name}} in default browser...", out.V{"namespace_name": namespace, "service_name": service})
			if err := browser.OpenURL(urlString); err != nil {
				out.ErrT(out.Empty, "browser failed to open url: {{.error}}", out.V{"error": err})
			}
		}
	}
	return nil
}

// GetServiceListByLabel returns a ServiceList by label
func GetServiceListByLabel(namespace string, key string, value string) (*core.ServiceList, error) {
	client, err := K8s.GetCoreClient()
	if err != nil {
		return &core.ServiceList{}, &retry.RetriableError{Err: err}
	}
	return getServiceListFromServicesByLabel(client.Services(namespace), key, value)
}

func getServiceListFromServicesByLabel(services typed_core.ServiceInterface, key string, value string) (*core.ServiceList, error) {
	selector := labels.SelectorFromSet(labels.Set(map[string]string{key: value}))
	serviceList, err := services.List(meta.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return &core.ServiceList{}, &retry.RetriableError{Err: err}
	}

	return serviceList, nil
}

// CreateSecret creates or modifies secrets
func CreateSecret(namespace, name string, dataValues map[string]string, labels map[string]string) error {
	client, err := K8s.GetCoreClient()
	if err != nil {
		return &retry.RetriableError{Err: err}
	}
	secrets := client.Secrets(namespace)
	secret, _ := secrets.Get(name, meta.GetOptions{})

	// Delete existing secret
	if len(secret.Name) > 0 {
		err = DeleteSecret(namespace, name)
		if err != nil {
			return &retry.RetriableError{Err: err}
		}
	}

	// convert strings to data secrets
	data := map[string][]byte{}
	for key, value := range dataValues {
		data[key] = []byte(value)
	}

	// Create Secret
	secretObj := &core.Secret{
		ObjectMeta: meta.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Data: data,
		Type: core.SecretTypeOpaque,
	}

	_, err = secrets.Create(secretObj)
	if err != nil {
		return &retry.RetriableError{Err: err}
	}

	return nil
}

// DeleteSecret deletes a secret from a namespace
func DeleteSecret(namespace, name string) error {
	client, err := K8s.GetCoreClient()
	if err != nil {
		return &retry.RetriableError{Err: err}
	}

	secrets := client.Secrets(namespace)
	err = secrets.Delete(name, &meta.DeleteOptions{})
	if err != nil {
		return &retry.RetriableError{Err: err}
	}

	return nil
}
