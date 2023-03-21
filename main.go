package main

import (
	"bytes"
	"io"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	"k8s.io/cloud-provider/options"
	"k8s.io/component-base/cli"
	cliflag "k8s.io/component-base/cli/flag"
	_ "k8s.io/component-base/metrics/prometheus/clientgo" // load all the prometheus client-go plugins
	_ "k8s.io/component-base/metrics/prometheus/version"  // for version metric registration
	"k8s.io/klog/v2"
	"os"
	"pg-yang.github.com/static-k8s-cloud-manager/pkg/cloud"
	// For existing cloud providers, the option to import legacy providers is still available.
	// e.g. _"k8s.io/legacy-cloud-providers/<provider>"
)

func init() {
	cloudprovider.RegisterCloudProvider(cloud.ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(config); err != nil {
			return nil, err
		}
		static := StaticCloudConfig{}
		if err := yaml.Unmarshal(buf.Bytes(), &static); err != nil {
			return nil, err
		}
		return cloud.NewStaticCloudProvider(
			static.StaticCloud.IpPool,
			static.StaticCloud.IpTrackerConfigMapNamespace,
			static.StaticCloud.IpTrackerConfigMap,
		), nil
	})
}

func main() {
	ccmOptions, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}
	controllerInitializers := app.DefaultInitFuncConstructors
	fss := cliflag.NamedFlagSets{}
	command := app.NewCloudControllerManagerCommand(ccmOptions, cloudInitializer, controllerInitializers, fss, wait.NeverStop)
	code := cli.Run(command)
	os.Exit(code)
}
func cloudInitializer(config *cloudcontrollerconfig.CompletedConfig) cloudprovider.Interface {
	cloudConfig := config.ComponentConfig.KubeCloudShared.CloudProvider
	provider, err := cloudprovider.InitCloudProvider(cloudConfig.Name, cloudConfig.CloudConfigFile)
	if err != nil {
		klog.Fatalf("Cloud provider could not be initialized: %v", err)
	}
	if provider == nil {
		klog.Fatalf("Cloud provider is nil")
	}
	if !provider.HasClusterID() {
		if config.ComponentConfig.KubeCloudShared.AllowUntaggedCloud {
			klog.Warning("detected a cluster without a ClusterID.  A ClusterID will be required in the future.  Please tag your cluster to avoid any future issues")
		} else {
			klog.Fatalf("no ClusterID found.  A ClusterID is required for the cloud provider to function properly.  This check can be bypassed by setting the allow-untagged-cloud option")
		}
	}
	return provider
}

type StaticCloudConfigEntity struct {
	IpPool                      string `yaml:"ip_pool" json:"ip_pool"`
	IpTrackerConfigMapNamespace string `yaml:"ip_tracker_config_map_namespace" json:"ip_tracker_config_map_namespace"`
	IpTrackerConfigMap          string `yaml:"ip_tracker_config_map" json:"ip_tracker_config_map"`
}

type StaticCloudConfig struct {
	StaticCloud StaticCloudConfigEntity `yaml:"static_cloud" json:"static_cloud"`
}
