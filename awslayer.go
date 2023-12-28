package main

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	autoscaling "github.com/aws/aws-sdk-go/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/mtyurt/ecstui/types"
	"github.com/mtyurt/ecstui/utils"
)

type AWSInteractionLayer struct {
	sess  *session.Session
	ecs   *ecs.ECS
	asg   *autoscaling.ApplicationAutoScaling
	elbv2 *elbv2.ELBV2
}

func NewAWSInteractionLayer() *AWSInteractionLayer {
	sess := session.Must(session.NewSession())

	return &AWSInteractionLayer{
		sess:  sess,
		ecs:   ecs.New(sess),
		asg:   autoscaling.New(sess),
		elbv2: elbv2.New(sess),
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
				Service: utils.GetLastItemAfterSplit(*service, "/"),
				Cluster: utils.GetLastItemAfterSplit(*cluster, "/"),
			})
		}
	}

	return itemList, nil
}

func (a *AWSInteractionLayer) GetImagesInTaskDefinition(taskDefinitionArn string) ([]string, error) {
	taskDefinition, err := a.ecs.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefinitionArn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe task definition: %v", err)
	}

	var images []string
	log.Printf("task definition: %v\n", taskDefinition.TaskDefinition.ContainerDefinitions)
	for _, container := range taskDefinition.TaskDefinition.ContainerDefinitions {
		images = append(images, utils.GetLastItemAfterSplit(*container.Image, "amazonaws.com/"))
	}

	return images, nil
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
		log.Printf("failed to describe scalable targets: %v\n", err)
		return nil, err
	}
	if len(targets.ScalableTargets) > 0 {
		response.Asg = types.ServiceScale{
			Min: *targets.ScalableTargets[0].MinCapacity,
			Max: *targets.ScalableTargets[0].MaxCapacity,
		}
	}

	if result.Services[0].TaskDefinition != nil {
		response.Images, err = a.GetImagesInTaskDefinition(*result.Services[0].TaskDefinition)
		if err != nil {
			log.Printf("failed to get images in task definition: %v\n", err)
			return nil, err
		}
	}

	if len(result.Services[0].TaskSets) > 0 {
		for _, ts := range result.Services[0].TaskSets {
			if ts.LoadBalancers != nil && len(ts.LoadBalancers) > 0 {
				for _, lb := range ts.LoadBalancers {
					lbConfig, err := a.findLoadBalancersForTargetGroup(*ts.Id, *lb.TargetGroupArn)
					if err != nil {
						log.Printf("failed to find load balancers for target group: %v\n", err)
						return nil, err
					}
					response.LbConfigs = append(response.LbConfigs, lbConfig...)
				}
			}
		}

	}
	return response, nil
}

func (a *AWSInteractionLayer) findLoadBalancersForTargetGroup(taskSetID, targetGroupArn string) ([]types.LbConfig, error) {
	// Describe the load balancers
	lbResp, err := a.elbv2.DescribeLoadBalancers(&elbv2.DescribeLoadBalancersInput{})
	if err != nil {
		return nil, err
	}

	var lbConfigs []types.LbConfig
	for _, lb := range lbResp.LoadBalancers {
		// Describe the listeners to find associated target groups
		listenerResp, err := a.elbv2.DescribeListeners(&elbv2.DescribeListenersInput{
			LoadBalancerArn: lb.LoadBalancerArn,
		})
		if err != nil {
			return nil, err
		}

		for _, listener := range listenerResp.Listeners {
			ruleResp, err := a.elbv2.DescribeRules(&elbv2.DescribeRulesInput{
				ListenerArn: listener.ListenerArn,
			})

			if err != nil {
				return nil, err
			}

			for _, rule := range ruleResp.Rules {
				for _, action := range rule.Actions {
					if *action.Type == "forward" {
						if *action.TargetGroupArn == targetGroupArn {
							lbConfigs = append(lbConfigs, types.LbConfig{
								LBName:   *lb.LoadBalancerName,
								TGName:   utils.GetLastItemAfterSplit(*rule.Actions[0].TargetGroupArn, "targetgroup/"),
								TGWeigth: 100,
							})
						} else if action.ForwardConfig != nil && action.ForwardConfig.TargetGroups != nil {
							for _, tg := range action.ForwardConfig.TargetGroups {
								if *tg.TargetGroupArn == targetGroupArn {
									log.Println("found target group in forward config", *action.ForwardConfig)
									lbConfigs = append(lbConfigs, types.LbConfig{
										LBName:   *lb.LoadBalancerName,
										TGName:   utils.GetLastItemAfterSplit(*tg.TargetGroupArn, "targetgroup/"),
										TGWeigth: *tg.Weight,
									})
								}
							}
						}
					}
				}
			}
		}
	}

	return lbConfigs, nil
}
