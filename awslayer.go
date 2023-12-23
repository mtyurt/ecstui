package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	autoscaling "github.com/aws/aws-sdk-go/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/mtyurt/ecstui/types"
)

type AWSInteractionLayer struct {
	sess *session.Session
	ecs  *ecs.ECS
	asg  *autoscaling.ApplicationAutoScaling
}

func NewAWSInteractionLayer() *AWSInteractionLayer {
	sess := session.Must(session.NewSession())

	return &AWSInteractionLayer{
		sess: sess,
		ecs:  ecs.New(sess),
		asg:  autoscaling.New(sess),
	}
}

func (a *AWSInteractionLayer) ListClusters() ([]*string, error) {
	result, err := a.ecs.ListClusters(&ecs.ListClustersInput{})
	if err != nil {
		return nil, err
	}

	return result.ClusterArns, nil
}

func (a *AWSInteractionLayer) ListServices(cluster string) ([]*string, error) {
	result, err := a.ecs.ListServices(&ecs.ListServicesInput{
		Cluster: aws.String(cluster),
	})
	if err != nil {
		return nil, err
	}

	return result.ServiceArns, nil
}

type ECSService struct {
	Service string
	Cluster string
	Arn     string
}

func (a *AWSInteractionLayer) FetchServiceList() ([]ECSService, error) {
	clusters, err := a.ListClusters()
	if err != nil {
		return nil, err
	}

	var itemList []ECSService
	for _, cluster := range clusters {
		services, err := a.ListServices(*cluster)
		if err != nil {
			return nil, err
		}

		for _, service := range services {
			itemList = append(itemList, ECSService{
				Service: getLastItemAfterSplit(*service, "/"),
				Cluster: getLastItemAfterSplit(*cluster, "/"),
			})
		}
	}

	return itemList, nil
}

func getLastItemAfterSplit(str, separator string) string {
	split := strings.Split(str, separator)
	return split[len(split)-1]
}

func (a *AWSInteractionLayer) FetchServiceStatus(cluster, service string) (*types.ServiceStatus, error) {
	input := &ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: []*string{&service},
	}

	log.Printf("fetching service status with input %v\n", input)
	result, err := a.ecs.DescribeServices(input)
	if err != nil {
		return nil, err
	}

	if len(result.Services) == 0 {
		return nil, nil
	}

	response := &types.ServiceStatus{
		Ecs: result.Services[0],
	}

	resourceID := fmt.Sprintf("service/%s/%s", cluster, service)
	targets, err := a.asg.DescribeScalableTargets(&autoscaling.DescribeScalableTargetsInput{
		ServiceNamespace: aws.String("ecs"),
		ResourceIds:      []*string{&resourceID},
	})
	if err != nil {
		return nil, err
	}
	if len(targets.ScalableTargets) > 0 {
		response.Asg = types.ServiceScale{
			Min: *targets.ScalableTargets[0].MinCapacity,
			Max: *targets.ScalableTargets[0].MaxCapacity,
		}
	}

	return response, nil
}
