package types

import (
	"github.com/aws/aws-sdk-go/service/ecs"
)

type ServiceScale struct {
	Min int64
	Max int64
}

type LbConfig struct {
	TaskSetID    string
	LBName       string
	TGName       string
	TGWeigth     int64
	ListenerPort int64
	Priority     string
}
type ServiceStatus struct {
	Ecs                *ecs.Service
	Asg                ServiceScale
	Images             []string
	TaskSetImages      map[string][]string
	TaskSetConnections map[string][]LbConfig
	TaskSetTasks       map[string][]*ecs.Task
}
