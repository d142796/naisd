package api

import (
	"fmt"
	"github.com/nais/naisd/api/app"
	redisapi "github.com/spotahome/redis-operator/api/redisfailover/v1alpha2"
	redisclient "github.com/spotahome/redis-operator/client/k8s/clientset/versioned/typed/redisfailover/v1alpha2"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8srest "k8s.io/client-go/rest"
)

func createRedisFailoverDef(spec app.Spec) *redisapi.RedisFailover {
	replicas := int32(3)
	resources := redisapi.RedisFailoverResources{
		Limits:   redisapi.CPUAndMem{Memory: "100Mi"},
		Requests: redisapi.CPUAndMem{CPU: "100m"},
	}

	redisSpec := redisapi.RedisFailoverSpec{
		HardAntiAffinity: false,
		Sentinel: redisapi.SentinelSettings{
			Replicas:  replicas,
			Resources: resources,
		},
		Redis: redisapi.RedisSettings{
			Replicas:  replicas,
			Resources: resources,
			Exporter:  true,
		},
	}

	meta := generateObjectMeta(spec)
	return &redisapi.RedisFailover{Spec: redisSpec, ObjectMeta: meta}
}

func getExistingFailover(failoverInterface redisclient.RedisFailoverInterface, resourceName string) (*redisapi.RedisFailover, error) {
	failover, err := failoverInterface.Get(resourceName, k8smeta.GetOptions{})

	switch {
	case err == nil:
		return failover, err
	case k8serrors.IsNotFound(err):
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected error: %s", err)
	}
}

func updateOrCreateRedisSentinelCluster(spec app.Spec) (*redisapi.RedisFailover, error) {
	newFailover := createRedisFailoverDef(spec)

	config, err := k8srest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("can't create InClusterConfig: %s", err)
	}

	client, err := redisclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("can't create new Redis client for InClusterConfig: %s", err)
	}

	existingFailover, err := getExistingFailover(redisclient.RedisFailoversGetter(client).RedisFailovers(spec.Namespace), spec.ResourceName())
	if err != nil {
		return nil, fmt.Errorf("unable to get existing redis failover: %s", err)
	}

	if existingFailover != nil {
		existingFailover.Spec = newFailover.Spec
		existingFailover.ObjectMeta = mergeObjectMeta(existingFailover.ObjectMeta, newFailover.ObjectMeta)
		return redisclient.RedisFailoversGetter(client).RedisFailovers(spec.Namespace).Update(existingFailover)
	}

	return redisclient.RedisFailoversGetter(client).RedisFailovers(spec.Namespace).Create(newFailover)
}
