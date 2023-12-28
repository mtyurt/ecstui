package main

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/service/ecs"
	servicetui "github.com/mtyurt/ecstui/tui/service"
	"github.com/mtyurt/ecstui/types"
)

func main() {
	str := func(s string) *string {
		return &s
	}
	int := func(i int64) *int64 {
		return &i
	}

	float := func(f float64) *float64 {
		return &f
	}

	bol := func(b bool) *bool {
		return &b
	}
	status := types.ServiceStatus{}
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	status.Ecs = &ecs.Service{
		ClusterArn: str("arn:aws:ecs:me-central-1:139007003299:cluster/app-cluster-staging"),
		CreatedBy:  str("arn:aws:iam::139007003299:role/BayzatApiTerraformRole"),
		DeploymentConfiguration: &ecs.DeploymentConfiguration{
			MaximumPercent:        int(200),
			MinimumHealthyPercent: int(100),
		},
		DeploymentController: &ecs.DeploymentController{Type: str("EXTERNAL")},
		DesiredCount:         int(1),
		EnableECSManagedTags: bol(false),
		EnableExecuteCommand: bol(true),
		Events: []*ecs.ServiceEvent{
			{
				Id:      str("198df382-fd88-46d4-9707-5a466d6d8f8d"),
				Message: str("service staging-api, taskSet ecs-svc/8895224990753999325) registered 1 targets in (target-group arn:aws:elasticloadbalancing:me-central-1:139007003299:targetgroup/staging-api-green/7f0c1d4ac8c3b215)"),
			},
			{
				Id:      str("cdad122d-5587-492e-a6c0-17588048b61d"),
				Message: str("(service staging-api, taskSet ecs-svc/8895224990753999325) has started 1 tasks: (task f6dd8e0f7a2a41e5ac58ae4b15b8ed8e)."),
			},
			{
				Id:      str("a6615958-9ea5-417f-8f1d-57fa9923a0b4"),
				Message: str("(service staging-api) updated computedDesiredCount for taskSet ecs-svc/8895224990753999325 to 1."),
			},
			{
				Id:      str("2410601c-1f6d-49f6-b9f0-8fa6d1fdbf83"),
				Message: str("(service staging-api) has reached a steady state."),
			},
		},
		HealthCheckGracePeriodSeconds: int(450),
		LoadBalancers: []*ecs.LoadBalancer{{
			ContainerName:  str("staging-api"),
			ContainerPort:  int(9292),
			TargetGroupArn: str("arn:aws:elasticloadbalancing:me-central-1:139007003299:targetgroup/staging-api-kt-tg/cdf8771ab7ca7fae"),
		}},
		PendingCount:       int(0),
		PlatformFamily:     str("Linux"),
		PlatformVersion:    str("1.4.0"),
		PropagateTags:      str("TASK_DEFINITION"),
		RoleArn:            str("arn:aws:iam::139007003299:role/aws-service-role/ecs.amazonaws.com/AWSServiceRoleForECS"),
		RunningCount:       int(2),
		SchedulingStrategy: str("REPLICA"),
		ServiceArn:         str("arn:aws:ecs:me-central-1:139007003299:service/app-cluster-staging/staging-api"),
		ServiceName:        str("staging-api"),
		Status:             str("ACTIVE"),
		TaskDefinition:     str("arn:aws:ecs:me-central-1:139007003299:task-definition/staging-api:442"),
		TaskSets: []*ecs.TaskSet{{
			ClusterArn:           str("arn:aws:ecs:me-central-1:139007003299:cluster/app-cluster-staging"),
			ComputedDesiredCount: int(1),
			Id:                   str("ecs-svc/3517849243791983451"),
			LoadBalancers: []*ecs.LoadBalancer{
				{
					ContainerName:  str("staging-api"),
					ContainerPort:  int(9292),
					TargetGroupArn: str("arn:aws:elasticloadbalancing:me-central-1:139007003299:targetgroup/staging-api-kt-tg/cdf8771ab7ca7fae"),
				}},
			PendingCount:    int(0),
			PlatformFamily:  str("Linux"),
			PlatformVersion: str("1.4.0"),
			RunningCount:    int(1),
			Scale: &ecs.Scale{
				Unit:  str("PERCENT"),
				Value: float(100),
			},
			ServiceArn:        str("arn:aws:ecs:me-central-1:139007003299:service/app-cluster-staging/staging-api"),
			StabilityStatus:   str("STEADY_STATE"),
			StabilityStatusAt: &now,
			Status:            str("PRIMARY"),
			TaskDefinition:    str("arn:aws:ecs:me-central-1:139007003299:task-definition/staging-api:442"),
			TaskSetArn:        str("arn:aws:ecs:me-central-1:139007003299:task-set/app-cluster-staging/staging-api/ecs-svc/3517849243791983451"),
			UpdatedAt:         &now,
			CreatedAt:         &oneHourAgo,
		}, {
			ClusterArn:           str("arn:aws:ecs:me-central-1:139007003299:cluster/app-cluster-staging"),
			ComputedDesiredCount: int(1),
			CreatedAt:            &now,
			Id:                   str("ecs-svc/8895224990753999325"),
			LoadBalancers: []*ecs.LoadBalancer{{
				ContainerName:  str("staging-api"),
				ContainerPort:  int(9292),
				TargetGroupArn: str("arn:aws:elasticloadbalancing:me-central-1:139007003299:targetgroup/staging-api-green/7f0c1d4ac8c3b215"),
			}},
			PendingCount:    int(0),
			PlatformFamily:  str("Linux"),
			PlatformVersion: str("1.4.0"),
			RunningCount:    int(1),
			Scale: &ecs.Scale{
				Unit:  str("PERCENT"),
				Value: float(100),
			},
			ServiceArn:        str("arn:aws:ecs:me-central-1:139007003299:service/app-cluster-staging/staging-api"),
			StabilityStatus:   str("STABILIZING"),
			StabilityStatusAt: &now,
			Status:            str("ACTIVE"),
			TaskDefinition:    str("arn:aws:ecs:me-central-1:139007003299:task-definition/staging-api:442"),
			TaskSetArn:        str("arn:aws:ecs:me-central-1:139007003299:task-set/app-cluster-staging/staging-api/ecs-svc/8895224990753999325"),
		}},
	}

	status.LbConfigs = []types.LbConfig{
		{
			TaskSetID: "ecs-svc/3517849243791983451",
			LBName:    "staging-api-kt-lb",
			TGName:    "staging-api-kt-tg",
			TGWeigth:  100,
		},
		{
			TaskSetID: "ecs-svc/8895224990753999325",
			LBName:    "staging-api-kt-lb",
			TGName:    "staging-api-green",
			TGWeigth:  100,
		},
	}

	m := servicetui.New("test-cluster", "test-service", "service-arn", nil)

	m.TestUpdate(&status)

	fmt.Println(m.View())

}
