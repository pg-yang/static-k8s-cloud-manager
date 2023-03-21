package cloud

import (
	cloudprovider "k8s.io/cloud-provider"
)

var (
	ProviderName = "static-cloud"
)

type StaticCloudProvider struct {
	clientBuilder               cloudprovider.ControllerClientBuilder
	ipPool                      string
	ipTrackerConfigMapNamespace string
	ipTrackerConfigMap          string
}

func NewStaticCloudProvider(ipPool, namespace, configmap string) *StaticCloudProvider {
	return &StaticCloudProvider{
		ipPool:                      ipPool,
		ipTrackerConfigMapNamespace: namespace,
		ipTrackerConfigMap:          configmap,
	}
}

func (s *StaticCloudProvider) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, _ <-chan struct{}) {
	s.clientBuilder = clientBuilder
}

func (s *StaticCloudProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return &StaticLoadBalancer{
		ipPool:                      s.ipPool,
		clientBuilder:               s.clientBuilder,
		ipTrackerConfigMapNamespace: s.ipTrackerConfigMapNamespace,
		ipTrackerConfigMap:          s.ipTrackerConfigMap,
	}, true
}

func (s *StaticCloudProvider) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

func (s *StaticCloudProvider) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return nil, false
}

func (s *StaticCloudProvider) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

func (s *StaticCloudProvider) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

func (s *StaticCloudProvider) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

func (s *StaticCloudProvider) ProviderName() string {
	return ProviderName
}

func (s *StaticCloudProvider) HasClusterID() bool {
	return true
}
