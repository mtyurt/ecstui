package types

import (
	"github.com/aws/aws-sdk-go/service/ecs"
)

type ServiceScale struct {
	Min int64
	Max int64
}
type ServiceStatus struct {
	Ecs    *ecs.Service
	Asg    ServiceScale
	Images []string
}
