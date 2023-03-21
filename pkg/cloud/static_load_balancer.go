package cloud

import (
	"context"
	"errors"
	v1 "k8s.io/api/core/v1"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"strconv"
	"strings"
)

var (
	loadBalancerName   = "static-ip-range"
	notFoundIp         = errors.New("not found ip")
	staticIpAnnotation = "pg-yang.github.com/static.ip"
)

type ipConfigMap struct {
	Namespace          string                 `json:"namespace"`
	ServiceName        string                 `json:"service_name"`
	LoadBalancerStatus *v1.LoadBalancerStatus `json:"load_balancer_status"`
}

type StaticLoadBalancer struct {
	ipPool                      string
	clientBuilder               cloudprovider.ControllerClientBuilder
	ipTrackerConfigMapNamespace string
	ipTrackerConfigMap          string
}

func (s *StaticLoadBalancer) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	client, err := s.clientBuilder.Client(clusterName)
	if err != nil {
		return nil, false, err
	}
	configKey := service.Namespace + "_" + service.Name
	configMap, err := s.createOrGetConfigMap(ctx, client)
	if err != nil {
		return nil, false, err
	}
	if data, ok := configMap.Data[configKey]; ok {
		ipConfigMap := &ipConfigMap{}
		if err := json.Unmarshal([]byte(data), ipConfigMap); err != nil {
			return nil, false, err
		}
		return ipConfigMap.LoadBalancerStatus, true, nil
	}
	return nil, false, nil
}

func (s *StaticLoadBalancer) GetLoadBalancerName(_ context.Context, _ string, _ *v1.Service) string {
	return loadBalancerName
}

func (s *StaticLoadBalancer) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, _ []*v1.Node) (*v1.LoadBalancerStatus, error) {
	client, err := s.clientBuilder.Client(clusterName)
	if err != nil {
		return nil, err
	}
	ingress := service.Status.LoadBalancer.Ingress
	if ingress != nil && len(ingress) > 0 {
		return &v1.LoadBalancerStatus{
			Ingress: ingress,
		}, nil
	}
	configKey := service.Namespace + "_" + service.Name
	configMap, err := s.createOrGetConfigMap(ctx, client)
	if err != nil {
		return nil, err
	}
	var ip string
	if value, ok := service.Annotations[staticIpAnnotation]; ok && len(value) != 0 {
		if err = s.validateIp(configMap, ip, configKey); err != nil {
			return nil, err
		}
		ip = value
	}
	if len(ip) == 0 {
		ip, err = s.chooseIp(configMap)
		if err != nil {
			return nil, err
		}
	}
	var loadBalancerPorts []v1.PortStatus
	for _, port := range service.Spec.Ports {
		loadBalancerPorts = append(loadBalancerPorts,
			v1.PortStatus{
				Port:     port.Port,
				Protocol: port.Protocol,
			},
		)
	}
	loadBalancer := &v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{
		{
			IP:       ip,
			Hostname: service.Name,
			Ports:    loadBalancerPorts,
		},
	}}
	marshal, err := json.Marshal(ipConfigMap{LoadBalancerStatus: loadBalancer, ServiceName: service.Name, Namespace: service.Namespace})
	if err != nil {
		return nil, err
	}
	if configMap.Data == nil {
		configMap.Data = map[string]string{}
	}
	configMap.Data[configKey] = string(marshal)
	_, err = client.CoreV1().ConfigMaps(s.ipTrackerConfigMapNamespace).Update(ctx, configMap, v12.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return loadBalancer, nil
}

func (s *StaticLoadBalancer) chooseIp(ipMapping *v1.ConfigMap) (string, error) {
	allocatedIps := map[string]struct{}{}
	for _, v := range ipMapping.Data {
		ipConfigMap := &ipConfigMap{}
		if err := json.Unmarshal([]byte(v), ipConfigMap); err != nil {
			return "", err
		}
		for _, ingress := range ipConfigMap.LoadBalancerStatus.Ingress {
			allocatedIps[ingress.IP] = struct{}{}
		}
	}
	split := strings.Split(s.ipPool, "-")
	end := split[1]
	for beginner := split[0]; compare(beginner, end); beginner = nextIp(beginner) {
		if _, ok := allocatedIps[beginner]; !ok {
			return beginner, nil
		}
	}
	return "", notFoundIp
}

func (s *StaticLoadBalancer) validateIp(ipMapping *v1.ConfigMap, ip string, key string) error {
	for k, v := range ipMapping.Data {
		ipConfigMap := &ipConfigMap{}
		if err := json.Unmarshal([]byte(v), ipConfigMap); err != nil {
			return err
		}
		for _, ingress := range ipConfigMap.LoadBalancerStatus.Ingress {
			if ingress.IP == ip && key != k {
				return errors.New("duplicate ip found :" + ip + ",key " + k)
			}
		}
	}
	return nil
}

func nextIp(beginner string) string {
	m := strings.Split(beginner, ".")
	i, _ := strconv.Atoi(m[3])
	next := strconv.Itoa(i + 1)
	return m[0] + "." + m[1] + "." + m[2] + "." + next
}

func compare(beginner string, end string) bool {
	m := strings.Split(beginner, ".")
	n := strings.Split(end, ".")
	i, _ := strconv.Atoi(m[3])
	j, _ := strconv.Atoi(n[3])
	return i <= j
}

func (s *StaticLoadBalancer) UpdateLoadBalancer(_ context.Context, _ string, _ *v1.Service, _ []*v1.Node) error {
	return nil
}

func (s *StaticLoadBalancer) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	client, err := s.clientBuilder.Client(clusterName)
	if err != nil {
		return err
	}
	configMap, err := client.CoreV1().ConfigMaps(s.ipTrackerConfigMapNamespace).Get(ctx, s.ipTrackerConfigMap, v12.GetOptions{})

	if status, ok := err.(*errors2.StatusError); ok && status.Status().Code == 404 {
		return nil
	}
	if err != nil {
		return err
	}
	configKey := service.Namespace + "_" + service.Name
	delete(configMap.Data, configKey)
	_, err = client.CoreV1().ConfigMaps(s.ipTrackerConfigMapNamespace).Update(ctx, configMap, v12.UpdateOptions{})
	return err
}

func (s *StaticLoadBalancer) createEmptyConfigMap(client kubernetes.Interface) (*v1.ConfigMap, error) {
	immutable := false
	configMap := v1.ConfigMap{
		TypeMeta: v12.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: v12.ObjectMeta{},
		Immutable:  &immutable,
		Data:       map[string]string{},
	}
	return client.CoreV1().ConfigMaps(s.ipTrackerConfigMapNamespace).Create(context.TODO(),
		&configMap,
		v12.CreateOptions{FieldManager: "static-ip"},
	)
}

func (s *StaticLoadBalancer) createOrGetConfigMap(ctx context.Context, client kubernetes.Interface) (*v1.ConfigMap, error) {
	configMap, err := client.CoreV1().ConfigMaps(s.ipTrackerConfigMapNamespace).Get(ctx, s.ipTrackerConfigMap, v12.GetOptions{})
	if status, ok := err.(*errors2.StatusError); ok && status.Status().Code == 404 {
		return s.createEmptyConfigMap(client)
	}
	return configMap, err
}
