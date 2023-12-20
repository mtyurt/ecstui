package main

import (
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
)

type AWSInteractionLayer struct {
	sess *session.Session
	ecs  *ecs.ECS
}

func NewAWSInteractionLayer() *AWSInteractionLayer {
	sess := session.Must(session.NewSession())

	return &AWSInteractionLayer{
		sess: sess,
		ecs:  ecs.New(sess),
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

func (a *AWSInteractionLayer) FetchServiceStatus(cluster string, serviceArn string) (*ecs.Service, error) {
	input := &ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: []*string{&serviceArn},
	}

	log.Printf("fetching service status with input %v\n", input)
	result, err := a.ecs.DescribeServices(input)
	if err != nil {
		return nil, err
	}

	if len(result.Services) == 0 {
		return nil, nil
	}

	return result.Services[0], nil
}
