package types

import (
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/elbv2"
)

type ServiceScale struct {
	Min int64
	Max int64
}

type ConnectionConfig struct {
	TaskSetID    string
	LBName       string
	TGName       string
	TGWeigth     int64
	ListenerPort int64
	Priority     string
	TGHealth     []*elbv2.TargetHealthDescription
}
type ServiceStatus struct {
	Ecs                *ecs.Service
	Asg                ServiceScale
	Images             []string
	TaskSetImages      map[string][]string
	TaskSetConnections map[string][]ConnectionConfig
	TaskSetTasks       map[string][]*ecs.Task
}

type TaskSetStatus struct {
	TaskSetImages      map[string][]string
	TaskSetConnections map[string][]ConnectionConfig
	TaskSetTasks       map[string][]*ecs.Task
}

type TaskSetStatusFetcher func(cluster, service string, taskSets []*ecs.TaskSet) (*TaskSetStatus, error)
