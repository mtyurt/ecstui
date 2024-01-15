package service

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mtyurt/ecstui/spinnertui"
	"github.com/mtyurt/ecstui/tui/events"
	"github.com/mtyurt/ecstui/tui/taskset"
	"github.com/mtyurt/ecstui/types"
	"github.com/mtyurt/ecstui/utils"
)

type sessionState int

const (
	initial sessionState = iota
	loaded
	errorState
	eventsOnly
)

var (
	styles = list.DefaultStyles()

	smallSectionStyle = lipgloss.NewStyle().
				Width(28).
				Height(6).
				Margin(0, 1, 0, 0).
				Align(lipgloss.Center).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"})
	largeSectionStyle = lipgloss.NewStyle().
				Width(150).
				Height(10).
				Margin(0, 1, 0, 0).
				Align(lipgloss.Left, lipgloss.Center).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"})
	foreground             = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#383838", Dark: "#D9DCCF"})
	subtle                 = lipgloss.NewStyle().Foreground(lipgloss.Color("#9B9B9B")).Align(lipgloss.Left)
	helpStyle              = lipgloss.NewStyle().Foreground(lipgloss.Color("#9B9B9B")).AlignHorizontal(lipgloss.Right)
	helpStyleKey           = lipgloss.NewStyle().Foreground(lipgloss.Color("#9B9BCC")).Bold(true)
	helpStyleVal           = lipgloss.NewStyle().Foreground(lipgloss.Color("#9B9B9B"))
	lastUpdateSpinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	minWidth               = 150
	taskSetWidth           = 32
)

type Model struct {
	state               sessionState
	cluster, serviceArn string
	service             string
	spinner             spinnertui.Model
	ecsStatus           *types.ServiceStatus
	err                 error
	width, height       int
	eventsViewport      *events.Model
	taskSetView         *taskset.Model
	Focused             bool
	lastUpdateTime      time.Time
	ecsStatusFetcher    func(string, string) (*types.ServiceStatus, error)
	autoRefresh         bool
	footerSpinner       spinner.Model
	showFooterSpinner   bool
}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

type ServiceMsg *types.ServiceStatus

type TickMsg time.Time

func New(cluster, service, serviceArn string, ecsStatusFetcher func(string, string) (*types.ServiceStatus, error)) Model {
	return Model{cluster: cluster,
		serviceArn:        serviceArn,
		service:           service,
		spinner:           spinnertui.New(fmt.Sprintf("Fetching %s status...", service)),
		Focused:           true,
		ecsStatusFetcher:  ecsStatusFetcher,
		footerSpinner:     spinner.New(spinner.WithSpinner(spinner.Hamburger), spinner.WithStyle(lastUpdateSpinnerStyle)),
		showFooterSpinner: false,
	}
}

func (m Model) fetchServiceStatus() tea.Msg {
	log.Println("started fetching service status")
	defer log.Println("finished fetching service status")
	serviceConfig, err := m.ecsStatusFetcher(m.cluster, m.service)
	if err != nil {
		return errMsg{err}
	}
	return ServiceMsg(serviceConfig)

}

func (m *Model) SetSize(width, height int) {
	if width < minWidth {
		width = minWidth
	}
	m.width = width
	m.height = height
	if m.eventsViewport != nil {
		m.eventsViewport.SetSize(width, height-1)
	}
}

func doTick() tea.Cmd {
	return tea.Tick(time.Second*30, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	cmd := []tea.Cmd{m.fetchServiceStatus, m.spinner.SpinnerTick()}
	if m.autoRefresh {
		cmd = append(cmd, doTick())
	}
	return tea.Batch(cmd...)
}
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	case ServiceMsg:
		log.Println("servicedetail loaded")
		m.ecsStatus = msg
		m.lastUpdateTime = time.Now()
		m.initializeSections()
		m.state = loaded
		m.showFooterSpinner = false
	case errMsg:
		log.Println("servicedetail error")
		m.err = msg
		m.state = errorState
	case tea.KeyMsg:
		log.Printf("servicedetail update key: %s\n", msg)
		log.Println("servicedetail state: ", m.state)
		switch k := msg.String(); k {
		case "ctrl+e":
			eventsViewport := events.New(m.service, 200, 50, m.ecsStatus.Ecs.Events)
			m.eventsViewport = &eventsViewport
			m.state = eventsOnly
			m.Focused = false
		case "esc":
			if m.state != loaded && m.eventsViewport.Focused() {
				m.state = loaded
				m.Focused = true
				m.eventsViewport = nil
			}
		case "ctrl+t": // toggle auto refresh
			m.autoRefresh = !m.autoRefresh
			if m.autoRefresh {
				cmds = append(cmds, doTick())
			}
		case "ctrl+r": // refresh
			m.showFooterSpinner = true
			cmds = append(cmds, m.fetchServiceStatus, m.footerSpinner.Tick)
		}
	case TickMsg:
		log.Println("servicedetail tick autoRefresh:", m.autoRefresh)
		if m.autoRefresh && time.Now().Sub(m.lastUpdateTime) > time.Second*28 {
			m.showFooterSpinner = true
			cmds = append(cmds, m.fetchServiceStatus, m.footerSpinner.Tick)
		}
		cmds = append(cmds, doTick())
	default:
		log.Printf("servicedetail update msg type: %v\n", msg)
	}

	switch m.state {
	case initial:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	case eventsOnly:
		log.Println("servicedetail eventsOnly update with msg")
		eventsViewport, cmd := m.eventsViewport.Update(msg)
		m.eventsViewport = &eventsViewport
		cmds = append(cmds, cmd)
	}
	if m.taskSetView != nil {
		taskSetView, cmd := m.taskSetView.Update(msg)
		m.taskSetView = &taskSetView
		cmds = append(cmds, cmd)
	}
	if m.showFooterSpinner {
		m.footerSpinner, cmd = m.footerSpinner.Update(msg)
		cmds = append(cmds, cmd)
	}
	log.Println("servicedetail update returning", len(cmds), cmds)

	return m, tea.Batch(cmds...)
}
func (m *Model) initializeSections() {
	serviceStatus := *m.ecsStatus.Ecs

	if serviceStatus.TaskSets != nil && len(serviceStatus.TaskSets) > 0 {
		status := m.ecsStatus
		m.taskSetView = taskset.New(status.TaskSetImages, status.TaskSetConnections, status.TaskSetTasks, status.Ecs.TaskSets, m.width, m.height)
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
	deploymentString = deploymentString + fmt.Sprintf("controller: %s\nstatus: %s", *serviceStatus.DeploymentController.Type, *serviceStatus.Status)
	return m.renderSmallSection("deployment", deploymentString)
}

func (m Model) taskdefView() *string {
	serviceStatus := *m.ecsStatus.Ecs
	if serviceStatus.TaskDefinition == nil {
		return nil
	}
	taskDef := utils.GetLastItemAfterSplit(*serviceStatus.TaskDefinition, "/")

	content := fmt.Sprintf("%s\n %s", taskDef, utils.JoinImageNames(m.ecsStatus.Images))
	view := m.renderSmallSection("taskDef", content)
	return &view
}

func (m Model) tasksetsView() string {
	tsSection := "not configured"
	if m.taskSetView != nil {
		tsSection = m.taskSetView.View()
	}

	return m.renderLargeSection("tasksets", tsSection)
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
	return largeSectionStyle.Copy().Width(m.width - 19).Render(styles.Title.AlignHorizontal(lipgloss.Center).Render(title) + "\n\n" + content + "\n")
}

func (m Model) footerView() string {
	refreshStatus := "enabled"
	if !m.autoRefresh {
		refreshStatus = "disabled"
	}
	help := map[string]string{
		"ctrl+t": "auto refresh " + refreshStatus,
		"ctrl+r": "manual refresh",
		"ctrl+e": "events",
		"esc":    "back",
	}
	fields := []string{}
	for k, v := range help {
		fields = append(fields, fmt.Sprintf("%s %s", helpStyleKey.Render(k), helpStyleVal.Render(v)))
	}
	slices.Sort(fields)

	lastUpdate := fmt.Sprintf("%s: %s", helpStyleVal.Render("last update"), helpStyleKey.Render(m.lastUpdateTime.Format("15:04:05.000")))

	style := helpStyle.Copy().Width(m.width - 30)
	if m.showFooterSpinner {
		lastUpdate = m.footerSpinner.View() + " " + lastUpdate
	}

	return (lipgloss.JoinVertical(lipgloss.Right, style.Render(strings.Join(fields, " • ")), style.Render(lastUpdate)))
}
func (m Model) sectionsView() string {
	firstRow := lipgloss.JoinHorizontal(lipgloss.Center, m.taskView(), m.deploymentView())
	taskdef := m.taskdefView()
	if taskdef != nil {
		firstRow = lipgloss.JoinHorizontal(lipgloss.Center, firstRow, *taskdef)
	}
	rows := []string{}
	rows = append(rows, firstRow)
	rows = append(rows, m.tasksetsView())
	events := m.eventsView()
	if events != "" {
		rows = append(rows, events)
	}
	rows = append(rows, m.footerView())
	view := lipgloss.JoinVertical(lipgloss.Center, rows...)

	return lipgloss.NewStyle().
		Width(m.width).Height(m.height).
		AlignHorizontal(lipgloss.Center).Render(view)
}

func (m Model) View() string {
	switch m.state {
	case initial:
		return m.spinner.View()
	case loaded:
		return m.sectionsView()
	case errorState:
		return m.err.Error()
	case eventsOnly:
		return m.eventsViewport.View()
	default:
		return m.serviceArn
	}
}
