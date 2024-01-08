package service

import (
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	humanizer "github.com/dustin/go-humanize"
	"github.com/mtyurt/ecstui/spinnertui"
	"github.com/mtyurt/ecstui/types"
	"github.com/mtyurt/ecstui/utils"
)

type sessionState int

const (
	initial sessionState = iota
	loaded
	errorState
)

var (
	styles            = list.DefaultStyles()
	smallSectionStyle = lipgloss.NewStyle().
				Width(35).
				Height(6).
				Margin(0, 1, 0, 0).
				Align(lipgloss.Center).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"})
	largeSectionStyle = lipgloss.NewStyle().
				Width(111).
				Height(10).
				Margin(0, 1, 0, 0).
				Align(lipgloss.Left, lipgloss.Center).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"})
	foreground = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#383838", Dark: "#D9DCCF"})
	subtle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#9B9B9B"))
)

type Model struct {
	state               sessionState
	cluster, serviceArn string
	serviceFetcher      func() tea.Msg
	spinner             spinnertui.Model
	ecsStatus           *types.ServiceStatus
	err                 error
	taskSetCreation     map[string]ecs.TaskSet
}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

type ServiceMsg *types.ServiceStatus

func New(cluster, service, serviceArn string, ecsStatusFetcher func(string, string) (*types.ServiceStatus, error)) Model {
	serviceFetcher := func() tea.Msg {
		log.Println("started fetching service status")
		defer log.Println("finished fetching service status")
		serviceConfig, err := ecsStatusFetcher(cluster, service)
		if err != nil {
			return errMsg{err}
		}
		return ServiceMsg(serviceConfig)
	}
	return Model{cluster: cluster,
		serviceArn:      serviceArn,
		serviceFetcher:  serviceFetcher,
		spinner:         spinnertui.New(fmt.Sprintf("Fetching %s status...", service)),
		taskSetCreation: make(map[string]ecs.TaskSet),
	}
}

func (m *Model) SetSize(width, height int) {
}
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.serviceFetcher, m.spinner.SpinnerTick())
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ServiceMsg:
		log.Println("servicedetail loaded")
		m.ecsStatus = msg
		m.initializeSections()
		m.state = loaded
	case errMsg:
		log.Println("servicedetail error")
		m.err = msg
		m.state = errorState
	default:
		log.Printf("servicedetail update msg type: %s\n", msg)
	}

	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch m.state {
	case initial:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	default:
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) initializeSections() {
	serviceStatus := *m.ecsStatus.Ecs

	if serviceStatus.TaskSets != nil && len(serviceStatus.TaskSets) > 0 {
		for _, ts := range serviceStatus.TaskSets {
			m.taskSetCreation[*ts.Id] = *ts
		}
	}
}

func (m Model) taskView() string {
	serviceStatus := *m.ecsStatus.Ecs
	taskString := foreground.Render(fmt.Sprintf("%d", *serviceStatus.RunningCount)) + "\n" + subtle.Render(fmt.Sprintf("desired: %d", *serviceStatus.DesiredCount))
	taskString = taskString + "\n" + subtle.Render(fmt.Sprintf("min: %d, max: %d", m.ecsStatus.Asg.Min, m.ecsStatus.Asg.Max))
	return m.renderSmallSection("task", taskString)
}

func (m Model) deploymentView() string {
	serviceStatus := *m.ecsStatus.Ecs
	deploymentString := ""
	if len(serviceStatus.CapacityProviderStrategy) > 0 {
		deploymentString = *serviceStatus.CapacityProviderStrategy[0].CapacityProvider + "\n"
	}
	deploymentString = deploymentString + fmt.Sprintf("controller: %s\nstatus: %s\n", *serviceStatus.DeploymentController.Type, *serviceStatus.Status)
	return m.renderSmallSection("deployment", deploymentString)
}

func (m Model) taskdefView() *string {
	serviceStatus := *m.ecsStatus.Ecs
	if serviceStatus.TaskDefinition == nil {
		return nil
	}
	taskDef := utils.GetLastItemAfterSplit(*serviceStatus.TaskDefinition, "/")

	content := fmt.Sprintf("taskdef: %s\n%s", taskDef, strings.Join(m.ecsStatus.Images, "\n"))
	view := m.renderSmallSection("taskDef", content)
	return &view
}

func (m Model) loadbalancerView() string {
	lbSection := "not configured"
	if m.ecsStatus.LbConfigs != nil && len(m.ecsStatus.LbConfigs) > 0 {
		lbSection = m.renderLbConfig(m.ecsStatus.LbConfigs)
	}
	return m.renderLargeSection("loadbalancer", lbSection)
}
func (m *Model) renderLbConfig(lbConfig []types.LbConfig) string {
	lbString := ""
	for _, lb := range lbConfig {
		taskCreation := *m.taskSetCreation[lb.TaskSetID].CreatedAt
		lbString = lbString + fmt.Sprintf("%s - %s -> %s (%%%d)\ncreated %s\n", lb.TaskSetID, lb.LBName, lb.TGName, lb.TGWeigth, humanizer.Time(taskCreation))
	}
	return lbString
}

func (m Model) eventsView() string {
	events := ""
	serviceStatus := *m.ecsStatus.Ecs
	for i, event := range serviceStatus.Events {
		events = events + *event.Message + "\n"
		if i > 5 {
			break
		}
	}
	if events == "" {
		return ""
	}
	return m.renderLargeSection("events", events)
}

func (m *Model) TestUpdate(status *types.ServiceStatus) {
	m.ecsStatus = status
	m.state = loaded
	m.initializeSections()
}

func (m Model) renderSmallSection(title, content string) string {
	return smallSectionStyle.Render(styles.Title.AlignHorizontal(lipgloss.Center).Render(title) + "\n\n" + content + "\n")
}
func (m Model) renderLargeSection(title, content string) string {
	return largeSectionStyle.Render(styles.Title.AlignHorizontal(lipgloss.Center).Render(title) + "\n\n" + content + "\n")
}
func (m Model) sectionsView() string {
	rows := []string{}
	firstRow := lipgloss.JoinHorizontal(lipgloss.Left, m.taskView(), m.deploymentView())
	taskdef := m.taskdefView()
	if taskdef != nil {
		firstRow = lipgloss.JoinHorizontal(lipgloss.Left, firstRow, *taskdef)
	}
	rows = append(rows, firstRow)
	rows = append(rows, m.loadbalancerView())
	events := m.eventsView()
	if events != "" {
		rows = append(rows, events)
	}
	return lipgloss.JoinVertical(lipgloss.Top, rows...)
}

func (m Model) View() string {
	switch m.state {
	case initial:
		return m.spinner.View()
	case loaded:
		return m.sectionsView()
	case errorState:
		return m.err.Error()
	default:
		return m.serviceArn
	}
}
