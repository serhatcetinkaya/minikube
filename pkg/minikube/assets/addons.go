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

package assets

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/pkg/errors"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/localpath"
	"k8s.io/minikube/pkg/util"
)

// Addon is a named list of assets, that can be enabled
type Addon struct {
	Assets    []*BinAsset
	enabled   bool
	addonName string
}

// NewAddon creates a new Addon
func NewAddon(assets []*BinAsset, enabled bool, addonName string) *Addon {
	a := &Addon{
		Assets:    assets,
		enabled:   enabled,
		addonName: addonName,
	}
	return a
}

// Name get the addon name
func (a *Addon) Name() string {
	return a.addonName
}

// IsEnabled checks if an Addon is enabled
func (a *Addon) IsEnabled() (bool, error) {
	addonStatusText, err := config.Get(a.addonName)
	if err == nil {
		addonStatus, err := strconv.ParseBool(addonStatusText)
		if err != nil {
			return false, err
		}
		return addonStatus, nil
	}
	return a.enabled, nil
}

// Addons is the list of addons
// TODO: Make dynamically loadable: move this data to a .yaml file within each addon directory
var Addons = map[string]*Addon{
	"addon-manager": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/addon-manager.yaml.tmpl",
			constants.GuestManifestsDir,
			"addon-manager.yaml.tmpl",
			"0640",
			true),
	}, true, "addon-manager"),
	"dashboard": NewAddon([]*BinAsset{
		MustBinAsset("deploy/addons/dashboard/dashboard-clusterrole.yaml", constants.GuestAddonsDir, "dashboard-clusterrole.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-clusterrolebinding.yaml", constants.GuestAddonsDir, "dashboard-clusterrolebinding.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-configmap.yaml", constants.GuestAddonsDir, "dashboard-configmap.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-dp.yaml", constants.GuestAddonsDir, "dashboard-dp.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-ns.yaml", constants.GuestAddonsDir, "dashboard-ns.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-role.yaml", constants.GuestAddonsDir, "dashboard-role.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-rolebinding.yaml", constants.GuestAddonsDir, "dashboard-rolebinding.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-sa.yaml", constants.GuestAddonsDir, "dashboard-sa.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-secret.yaml", constants.GuestAddonsDir, "dashboard-secret.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-svc.yaml", constants.GuestAddonsDir, "dashboard-svc.yaml", "0640", false),
	}, false, "dashboard"),
	"default-storageclass": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/storageclass/storageclass.yaml.tmpl",
			constants.GuestAddonsDir,
			"storageclass.yaml",
			"0640",
			false),
	}, true, "default-storageclass"),
	"storage-provisioner": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/storage-provisioner/storage-provisioner.yaml.tmpl",
			constants.GuestAddonsDir,
			"storage-provisioner.yaml",
			"0640",
			true),
	}, true, "storage-provisioner"),
	"storage-provisioner-gluster": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/storage-provisioner-gluster/storage-gluster-ns.yaml.tmpl",
			constants.GuestAddonsDir,
			"storage-gluster-ns.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/storage-provisioner-gluster/glusterfs-daemonset.yaml.tmpl",
			constants.GuestAddonsDir,
			"glusterfs-daemonset.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/storage-provisioner-gluster/heketi-deployment.yaml.tmpl",
			constants.GuestAddonsDir,
			"heketi-deployment.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/storage-provisioner-gluster/storage-provisioner-glusterfile.yaml.tmpl",
			constants.GuestAddonsDir,
			"storage-privisioner-glusterfile.yaml",
			"0640",
			false),
	}, false, "storage-provisioner-gluster"),
	"heapster": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/heapster/influx-grafana-rc.yaml.tmpl",
			constants.GuestAddonsDir,
			"influxGrafana-rc.yaml",
			"0640",
			true),
		MustBinAsset(
			"deploy/addons/heapster/grafana-svc.yaml.tmpl",
			constants.GuestAddonsDir,
			"grafana-svc.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/heapster/influxdb-svc.yaml.tmpl",
			constants.GuestAddonsDir,
			"influxdb-svc.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/heapster/heapster-rc.yaml.tmpl",
			constants.GuestAddonsDir,
			"heapster-rc.yaml",
			"0640",
			true),
		MustBinAsset(
			"deploy/addons/heapster/heapster-svc.yaml.tmpl",
			constants.GuestAddonsDir,
			"heapster-svc.yaml",
			"0640",
			false),
	}, false, "heapster"),
	"efk": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/efk/elasticsearch-rc.yaml.tmpl",
			constants.GuestAddonsDir,
			"elasticsearch-rc.yaml",
			"0640",
			true),
		MustBinAsset(
			"deploy/addons/efk/elasticsearch-svc.yaml.tmpl",
			constants.GuestAddonsDir,
			"elasticsearch-svc.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/efk/fluentd-es-rc.yaml.tmpl",
			constants.GuestAddonsDir,
			"fluentd-es-rc.yaml",
			"0640",
			true),
		MustBinAsset(
			"deploy/addons/efk/fluentd-es-configmap.yaml.tmpl",
			constants.GuestAddonsDir,
			"fluentd-es-configmap.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/efk/kibana-rc.yaml.tmpl",
			constants.GuestAddonsDir,
			"kibana-rc.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/efk/kibana-svc.yaml.tmpl",
			constants.GuestAddonsDir,
			"kibana-svc.yaml",
			"0640",
			false),
	}, false, "efk"),
	"ingress": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/ingress/ingress-configmap.yaml.tmpl",
			constants.GuestAddonsDir,
			"ingress-configmap.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/ingress/ingress-rbac.yaml.tmpl",
			constants.GuestAddonsDir,
			"ingress-rbac.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/ingress/ingress-dp.yaml.tmpl",
			constants.GuestAddonsDir,
			"ingress-dp.yaml",
			"0640",
			true),
	}, false, "ingress"),
	"metrics-server": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/metrics-server/metrics-apiservice.yaml.tmpl",
			constants.GuestAddonsDir,
			"metrics-apiservice.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/metrics-server/metrics-server-deployment.yaml.tmpl",
			constants.GuestAddonsDir,
			"metrics-server-deployment.yaml",
			"0640",
			true),
		MustBinAsset(
			"deploy/addons/metrics-server/metrics-server-service.yaml.tmpl",
			constants.GuestAddonsDir,
			"metrics-server-service.yaml",
			"0640",
			false),
	}, false, "metrics-server"),
	"registry": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/registry/registry-rc.yaml.tmpl",
			constants.GuestAddonsDir,
			"registry-rc.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/registry/registry-svc.yaml.tmpl",
			constants.GuestAddonsDir,
			"registry-svc.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/registry/registry-proxy.yaml.tmpl",
			constants.GuestAddonsDir,
			"registry-proxy.yaml",
			"0640",
			false),
	}, false, "registry"),
	"registry-creds": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/registry-creds/registry-creds-rc.yaml.tmpl",
			constants.GuestAddonsDir,
			"registry-creds-rc.yaml",
			"0640",
			false),
	}, false, "registry-creds"),
	"freshpod": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/freshpod/freshpod-rc.yaml.tmpl",
			constants.GuestAddonsDir,
			"freshpod-rc.yaml",
			"0640",
			true),
	}, false, "freshpod"),
	"nvidia-driver-installer": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/gpu/nvidia-driver-installer.yaml.tmpl",
			constants.GuestAddonsDir,
			"nvidia-driver-installer.yaml",
			"0640",
			true),
	}, false, "nvidia-driver-installer"),
	"nvidia-gpu-device-plugin": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/gpu/nvidia-gpu-device-plugin.yaml.tmpl",
			constants.GuestAddonsDir,
			"nvidia-gpu-device-plugin.yaml",
			"0640",
			true),
	}, false, "nvidia-gpu-device-plugin"),
	"logviewer": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/logviewer/logviewer-dp-and-svc.yaml.tmpl",
			constants.GuestAddonsDir,
			"logviewer-dp-and-svc.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/logviewer/logviewer-rbac.yaml.tmpl",
			constants.GuestAddonsDir,
			"logviewer-rbac.yaml",
			"0640",
			false),
	}, false, "logviewer"),
	"gvisor": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/gvisor/gvisor-pod.yaml.tmpl",
			constants.GuestAddonsDir,
			"gvisor-pod.yaml",
			"0640",
			true),
		MustBinAsset(
			"deploy/addons/gvisor/gvisor-runtimeclass.yaml",
			constants.GuestAddonsDir,
			"gvisor-runtimeclass.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/gvisor/gvisor-config.toml",
			constants.GvisorFilesPath,
			constants.GvisorConfigTomlTargetName,
			"0640",
			true),
	}, false, "gvisor"),
}

// AddMinikubeDirAssets adds all addons and files to the list
// of files to be copied to the vm.
func AddMinikubeDirAssets(assets *[]CopyableFile) error {
	if err := addMinikubeDirToAssets(localpath.MakeMiniPath("addons"), constants.GuestAddonsDir, assets); err != nil {
		return errors.Wrap(err, "adding addons folder to assets")
	}
	if err := addMinikubeDirToAssets(localpath.MakeMiniPath("files"), "", assets); err != nil {
		return errors.Wrap(err, "adding files rootfs to assets")
	}

	return nil
}

// AddMinikubeDirToAssets adds all the files in the basedir argument to the list
// of files to be copied to the vm.  If vmpath is left blank, the files will be
// transferred to the location according to their relative minikube folder path.
func addMinikubeDirToAssets(basedir, vmpath string, assets *[]CopyableFile) error {
	return filepath.Walk(basedir, func(hostpath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		isDir, err := util.IsDirectory(hostpath)
		if err != nil {
			return errors.Wrapf(err, "checking if %s is directory", hostpath)
		}
		if !isDir {
			vmdir := vmpath
			if vmdir == "" {
				rPath, err := filepath.Rel(basedir, hostpath)
				if err != nil {
					return errors.Wrap(err, "generating relative path")
				}
				rPath = filepath.Dir(rPath)
				rPath = filepath.ToSlash(rPath)
				vmdir = path.Join("/", rPath)
			}
			permString := fmt.Sprintf("%o", info.Mode().Perm())
			// The conversion will strip the leading 0 if present, so add it back
			// if we need to.
			if len(permString) == 3 {
				permString = fmt.Sprintf("0%s", permString)
			}

			f, err := NewFileAsset(hostpath, vmdir, filepath.Base(hostpath), permString)
			if err != nil {
				return errors.Wrapf(err, "creating file asset for %s", hostpath)
			}
			*assets = append(*assets, f)
		}

		return nil
	})
}

// GenerateTemplateData generates template data for template assets
func GenerateTemplateData(cfg config.KubernetesConfig) interface{} {

	a := runtime.GOARCH
	// Some legacy docker images still need the -arch suffix
	// for  less common architectures blank suffix for amd64
	ea := ""
	if runtime.GOARCH != "amd64" {
		ea = runtime.GOARCH
	}
	opts := struct {
		Arch            string
		ExoticArch      string
		ImageRepository string
	}{
		Arch:            a,
		ExoticArch:      ea,
		ImageRepository: cfg.ImageRepository,
	}

	return opts
}
