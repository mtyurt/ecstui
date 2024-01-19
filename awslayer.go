package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	autoscaling "github.com/aws/aws-sdk-go/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/mtyurt/ecstui/logger"
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
				Arn:     *service,
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
	for _, container := range taskDefinition.TaskDefinition.ContainerDefinitions {
		image := utils.GetLastItemAfterSplit(*container.Image, "amazonaws.com/")
		if image != "" {
			images = append(images, image)
		}
	}

	return images, nil
}

func (a *AWSInteractionLayer) FetchServiceStatus(cluster, service string) (*types.ServiceStatus, error) {
	input := &ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: []*string{&service},
	}

	logger.Printf("fetching service status with input %v\n", input)
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
		logger.Printf("failed to describe scalable targets: %v\n", err)
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
			logger.Printf("failed to get images in task definition: %v\n", err)
			return nil, err
		}
	}
	return response, nil
}
func (a *AWSInteractionLayer) FetchDeploymentsStatus(cluster, service string, deployments []*ecs.Deployment, loadBalancers []*ecs.LoadBalancer) (*types.DeploymentStatus, error) {
	response := &types.DeploymentStatus{}
	response.DeploymentImages = make(map[string][]string)
	response.DeploymentTasks = make(map[string][]*ecs.Task)

	var err error
	if len(deployments) > 0 {
		for _, d := range deployments {
			if d.TaskDefinition != nil {
				response.DeploymentImages[*d.Id], err = a.GetImagesInTaskDefinition(*d.TaskDefinition)
				if err != nil {
					logger.Printf("failed to get images in task definition: %v\n", err)
					return nil, err
				}
			}

			tasks, err := a.findTasksForTaskSet(cluster, service, *d.Id)
			if err != nil {
				logger.Printf("failed to find tasks for deployment[%s]: %v\n", *d.Id, err)
				return nil, err
			}
			response.DeploymentTasks[*d.Id] = tasks
		}
	}
	if len(loadBalancers) > 0 {
		lbConfigs := make([]types.ConnectionConfig, 0)
		for _, lb := range loadBalancers {
			lbConfig, err := a.findLoadBalancersForTargetGroup(*lb.TargetGroupArn)
			if err != nil {
				logger.Printf("failed to find load balancers for target group: %v\n", err)
				return nil, err
			}
			lbConfigs = append(lbConfigs, lbConfig...)
		}
		response.DeploymentConnections = lbConfigs
	}
	return response, nil
}
func (a *AWSInteractionLayer) FetchTaskSetStatus(cluster, service string, taskSets []*ecs.TaskSet) (*types.TaskSetStatus, error) {
	response := &types.TaskSetStatus{}
	response.TaskSetImages = make(map[string][]string)
	response.TaskSetConnections = make(map[string][]types.ConnectionConfig)
	response.TaskSetTasks = make(map[string][]*ecs.Task)
	var err error
	if len(taskSets) > 0 {
		for _, ts := range taskSets {
			if ts.LoadBalancers != nil && len(ts.LoadBalancers) > 0 {
				lbConfigs := make([]types.ConnectionConfig, 0)
				for _, lb := range ts.LoadBalancers {
					lbConfig, err := a.findLoadBalancersForTargetGroupWithTaskSetID(*ts.Id, *lb.TargetGroupArn)
					if err != nil {
						logger.Printf("failed to find load balancers for target group: %v\n", err)
						return nil, err
					}
					lbConfigs = append(lbConfigs, lbConfig...)
				}
				response.TaskSetConnections[*ts.Id] = lbConfigs
			}
			if ts.TaskDefinition != nil {
				response.TaskSetImages[*ts.Id], err = a.GetImagesInTaskDefinition(*ts.TaskDefinition)
				if err != nil {
					logger.Printf("failed to get images in task definition: %v\n", err)
					return nil, err
				}
			}

			tasks, err := a.findTasksForTaskSet(cluster, service, *ts.Id)
			if err != nil {
				logger.Printf("failed to find tasks for task set[%s]: %v\n", *ts.Id, err)
				return nil, err
			}
			response.TaskSetTasks[*ts.Id] = tasks
		}

	}
	return response, nil
}

func (a *AWSInteractionLayer) findTasksForTaskSet(cluster, service, taskSetID string) ([]*ecs.Task, error) {
	logger.Println("finding tasks for task set", cluster, service, taskSetID)
	listResp, err := a.ecs.ListTasks(&ecs.ListTasksInput{
		Cluster:   aws.String(cluster),
		StartedBy: aws.String(taskSetID),
	})
	if err != nil {
		return nil, err
	}
	if len(listResp.TaskArns) == 0 {
		logger.Println("no tasks found for task set", taskSetID)
		return []*ecs.Task{}, nil
	}
	taskResp, err := a.ecs.DescribeTasks(&ecs.DescribeTasksInput{
		Cluster: aws.String(cluster),
		Tasks:   listResp.TaskArns,
	})
	if err != nil {
		return nil, err
	}

	return taskResp.Tasks, nil
}
func (a *AWSInteractionLayer) findLoadBalancersForTargetGroupWithTaskSetID(taskSetID, targetGroupArn string) ([]types.ConnectionConfig, error) {
	conns, err := a.findLoadBalancersForTargetGroup(targetGroupArn)
	if err != nil {
		return nil, err
	}

	for i := range conns {
		conns[i].TaskSetID = taskSetID
	}
	return conns, nil
}
func (a *AWSInteractionLayer) findLoadBalancersForTargetGroup(targetGroupArn string) ([]types.ConnectionConfig, error) {
	// Describe the load balancers
	lbResp, err := a.elbv2.DescribeLoadBalancers(&elbv2.DescribeLoadBalancersInput{})
	if err != nil {
		return nil, err
	}

	var lbConfigs []types.ConnectionConfig
	found := false
	shortTgName := utils.GetLastItemAfterSplit(targetGroupArn, "targetgroup/")
	tgHealthCache := make(map[string][]*elbv2.TargetHealthDescription)
	for _, lb := range lbResp.LoadBalancers {
		// Describe the listeners to find associated target groups
		listenerResp, err := a.elbv2.DescribeListeners(&elbv2.DescribeListenersInput{
			LoadBalancerArn: lb.LoadBalancerArn,
		})
		if err != nil {
			return nil, err
		}

		for _, listener := range listenerResp.Listeners {
			if *listener.Port != 443 {
				continue
			}
			ruleResp, err := a.elbv2.DescribeRules(&elbv2.DescribeRulesInput{
				ListenerArn: listener.ListenerArn,
			})

			if err != nil {
				return nil, err
			}

			for _, rule := range ruleResp.Rules {
				if *rule.Priority == "default" {
					continue
				}
				for _, action := range rule.Actions {
					if *action.Type == "forward" {
						if action.TargetGroupArn != nil && *action.TargetGroupArn == targetGroupArn {
							tgHealth, err := a.getTGHealth(tgHealthCache, targetGroupArn)
							if err != nil {
								return nil, err
							}
							lbConfigs = append(lbConfigs, types.ConnectionConfig{
								LBName:   *lb.LoadBalancerName,
								TGName:   shortTgName,
								TGWeigth: *action.ForwardConfig.TargetGroups[0].Weight,
								Priority: *rule.Priority,
								TGHealth: tgHealth,
							})
							found = true
						} else if action.ForwardConfig != nil && action.ForwardConfig.TargetGroups != nil {
							for _, tg := range action.ForwardConfig.TargetGroups {
								if *tg.TargetGroupArn == targetGroupArn {
									logger.Println("found target group in forward config", *action.ForwardConfig)
									tgHealth, err := a.getTGHealth(tgHealthCache, targetGroupArn)
									if err != nil {
										return nil, err
									}
									lbConfigs = append(lbConfigs, types.ConnectionConfig{
										LBName:   *lb.LoadBalancerName,
										TGName:   shortTgName,
										TGWeigth: *tg.Weight,
										Priority: *rule.Priority,
										TGHealth: tgHealth,
									})
									found = true
								}
							}
						}
					}
				}
			}
		}
	}

	if !found {
		tgHealth, err := a.getTGHealth(tgHealthCache, targetGroupArn)
		if err != nil {
			return nil, err
		}
		lbConfigs = append(lbConfigs, types.ConnectionConfig{
			TGName:   shortTgName,
			TGHealth: tgHealth,
		})
	}

	return lbConfigs, nil
}

func (a *AWSInteractionLayer) getTGHealth(cache map[string][]*elbv2.TargetHealthDescription, tgArn string) ([]*elbv2.TargetHealthDescription, error) {
	if health, ok := cache[tgArn]; ok {
		return health, nil
	}
	resp, err := a.elbv2.DescribeTargetHealth(&elbv2.DescribeTargetHealthInput{
		TargetGroupArn: aws.String(tgArn),
	})
	if err != nil {
		return nil, err
	}

	cache[tgArn] = resp.TargetHealthDescriptions
	return resp.TargetHealthDescriptions, nil
}
