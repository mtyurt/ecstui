package service

import (
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
				Width(80).
				Height(10).
				Margin(0, 1, 0, 0).
				Align(lipgloss.Left, lipgloss.Center).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"})
	foreground = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#383838", Dark: "#D9DCCF"})
	subtle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#565656", Dark: "#454545"})
)

type sectionSize int

const (
	smallSection = iota
	largeSection
)

type section struct {
	content string
	title   string
	size    sectionSize
}

type Model struct {
	state               sessionState
	cluster, serviceArn string
	serviceFetcher      func() tea.Msg
	spinner             spinnertui.Model
	ecsStatus           *types.ServiceStatus
	err                 error
	sections            []section
}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

type serviceMsg *types.ServiceStatus

func New(cluster, service, serviceArn string, ecsStatusFetcher func(string, string) (*types.ServiceStatus, error)) Model {
	serviceFetcher := func() tea.Msg {
		log.Println("started fetching service status")
		defer log.Println("finished fetching service status")
		serviceConfig, err := ecsStatusFetcher(cluster, service)
		if err != nil {
			return errMsg{err}
		}
		return serviceMsg(serviceConfig)
	}
	return Model{cluster: cluster,
		serviceArn:     serviceArn,
		serviceFetcher: serviceFetcher,
		spinner:        spinnertui.New(fmt.Sprintf("Fetching %s status...", service))}
}

func (m *Model) SetSize(width, height int) {
}
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.serviceFetcher, m.spinner.SpinnerTick())
}

func (m *Model) initializeSections() {
	events := ""
	serviceStatus := *m.ecsStatus.Ecs
	for _, event := range serviceStatus.Events {
		events = events + *event.Message + "\n"
	}
	log.Println(serviceStatus)
	taskString := foreground.Render(fmt.Sprintf("%d", *serviceStatus.RunningCount)) + "\n" + subtle.Render(fmt.Sprintf("desired: %d", *serviceStatus.DesiredCount))
	taskString = taskString + "\n" + subtle.Render(fmt.Sprintf("min: %d, max: %d", m.ecsStatus.Asg.Min, m.ecsStatus.Asg.Max))

	deploymentString := ""
	if len(serviceStatus.CapacityProviderStrategy) > 0 {
		deploymentString = *serviceStatus.CapacityProviderStrategy[0].CapacityProvider + "\n"
	}
	deploymentString = deploymentString + fmt.Sprintf("controller: %s\n", *serviceStatus.DeploymentController.Type)
	taskDef := utils.GetLastItemAfterSplit(*serviceStatus.TaskDefinition, "/")
	deploymentString = deploymentString + fmt.Sprintf("task def: %s\n", taskDef)

	m.sections = []section{
		{title: "task", content: taskString},
		{title: "status", content: *serviceStatus.Status},
		{title: "deployment", content: foreground.Render(deploymentString)},
		{title: "images", content: foreground.Align(lipgloss.Left).Render(taskDef + "\n" + strings.Join(m.ecsStatus.Images, "\n"))},
		{title: "events", content: events, size: largeSection},
	}
}
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	log.Printf("servicedetail update: %v\n", msg)
	switch msg := msg.(type) {
	case serviceMsg:
		log.Printf("servicemsg received: %v\n", *msg)
		m.ecsStatus = msg
		m.initializeSections()
		m.state = loaded
	case errMsg:
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

func (m Model) renderSection(index int) string {
	style := smallSectionStyle
	if m.sections[index].size == largeSection {
		style = largeSectionStyle
	}

	return style.Render(styles.Title.Render(m.sections[index].title) + "\n\n" + m.sections[index].content + "\n")
}
func (m Model) sectionsView() string {
	rows := []string{}
	i := 0
	log.Printf("total sections: %d\n", len(m.sections))
	for i < len(m.sections)/2+1 {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left, m.renderSection(i), m.renderSection(i+1)))
		i += 2
	}
	log.Printf("end of for: %d\n", i)
	if i < len(m.sections) {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left, m.renderSection(i)))
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
