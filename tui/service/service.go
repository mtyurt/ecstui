package service

import (
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	humanizer "github.com/dustin/go-humanize"
	"github.com/mtyurt/ecstui/spinnertui"
	"github.com/mtyurt/ecstui/tui/events"
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
	styles            = list.DefaultStyles()
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
	foreground   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#383838", Dark: "#D9DCCF"})
	subtle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#9B9B9B"))
	minWidth     = 150
	taskSetWidth = 32
)

type Model struct {
	state               sessionState
	cluster, serviceArn string
	service             string
	serviceFetcher      func() tea.Msg
	spinner             spinnertui.Model
	ecsStatus           *types.ServiceStatus
	err                 error
	taskSetMap          map[string]ecs.TaskSet
	taskdefImageFetcher func() ([]string, error)
	width, height       int
	eventsViewport      *events.Model
	Focused             bool
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
		serviceArn:     serviceArn,
		service:        service,
		serviceFetcher: serviceFetcher,
		spinner:        spinnertui.New(fmt.Sprintf("Fetching %s status...", service)),
		taskSetMap:     make(map[string]ecs.TaskSet),
		Focused:        true,
	}
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
	case tea.KeyMsg:
		log.Printf("servicedetail update key: %s\n", msg)
		log.Println("servicedetail state: ", m.state)
		k := msg.String()
		if k == "ctrl+e" {
			eventsViewport := events.New(m.service, 200, 70, m.ecsStatus.Ecs.Events)

			m.eventsViewport = &eventsViewport

			m.state = eventsOnly
			m.Focused = false
		} else if k == "esc" && m.state != loaded && m.eventsViewport.Focused() {
			m.state = loaded
			m.Focused = true
			m.eventsViewport = nil
		}

	default:
		log.Printf("servicedetail update msg type: %v\n", msg)
	}

	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch m.state {
	case initial:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	case eventsOnly:
		log.Println("servicedetail eventsOnly update with msg")
		eventsViewport, cmd := m.eventsViewport.Update(msg)
		m.eventsViewport = &eventsViewport
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
			m.taskSetMap[*ts.Id] = *ts
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
	deploymentString = deploymentString + fmt.Sprintf("controller: %s\nstatus: %s", *serviceStatus.DeploymentController.Type, *serviceStatus.Status)
	return m.renderSmallSection("deployment", deploymentString)
}

func (m Model) taskdefView() *string {
	serviceStatus := *m.ecsStatus.Ecs
	if serviceStatus.TaskDefinition == nil {
		return nil
	}
	taskDef := utils.GetLastItemAfterSplit(*serviceStatus.TaskDefinition, "/")

	content := fmt.Sprintf("%s\n %s", taskDef, strings.Join(m.ecsStatus.Images, "\n "))
	view := m.renderSmallSection("taskDef", content)
	return &view
}

func (m Model) tasksetsView() string {
	tsSection := "not configured"
	if m.ecsStatus.TaskSetConnections != nil && len(m.ecsStatus.TaskSetConnections) > 0 {
		tsSection = m.renderLbConfigs(m.ecsStatus.TaskSetConnections)
	}

	return m.renderLargeSection("tasksets", tsSection)
}
func (m *Model) renderLbConfigs(lbConfig map[string][]types.LbConfig) string {
	viewByLb := make(map[string][]string)
	cfgByTaskSet := make(map[string]*types.LbConfig)
	for taskSetID, lbs := range lbConfig {
		priority := ""
		for _, lb := range lbs {
			if lb.LBName != "" && lb.Priority != "" {
				priority = lb.Priority
			}
		}
		cfgByTaskSet[taskSetID] = &types.LbConfig{
			LBName:    lbs[0].LBName,
			Priority:  priority,
			TGName:    lbs[0].TGName,
			TGWeigth:  lbs[0].TGWeigth,
			TaskSetID: taskSetID,
		}
	}

	unattachedTaskSets := []string{}
	for _, lb := range cfgByTaskSet {

		if lb.LBName != "" {
			if _, ok := viewByLb[lb.LBName]; !ok {
				viewByLb[lb.LBName] = []string{m.renderTaskSetThroughLb(*lb)}
			} else {
				viewByLb[lb.LBName] = append(viewByLb[lb.LBName], m.renderTaskSetThroughLb(*lb))
			}
		} else {
			unattachedTaskSets = append(unattachedTaskSets, m.renderUnattachedTaskSet(*lb))
		}

	}
	lbs := make([]string, 0, len(viewByLb))
	for lbName, lbViews := range viewByLb {
		bottom := smallSectionStyle.Copy().Width(len(lbViews)*taskSetWidth + 2).Height(1).Render(styles.Title.AlignHorizontal(lipgloss.Center).Render(lbName))
		tgView := lipgloss.JoinVertical(lipgloss.Center, lipgloss.JoinHorizontal(lipgloss.Center, lbViews...), bottom)
		lbs = append(lbs, tgView)
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, lbs...)
}

func truncateTo(s string, max int) string {
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}

func (m Model) renderTaskSetThroughLb(lbConfig types.LbConfig) string {
	sectionWidth := taskSetWidth
	ts := m.taskSetMap[lbConfig.TaskSetID]
	attachmentTemplate := `▲
|
%d%%
|
%s
priority: %s`

	attachment := fmt.Sprintf(attachmentTemplate, lbConfig.TGWeigth, truncateTo(lbConfig.TGName, sectionWidth), lbConfig.Priority)
	return m.renderTaskSetWithAttachment(ts, attachment)
}

func (m Model) renderUnattachedTaskSet(lbConfig types.LbConfig) string {
	ts := m.taskSetMap[lbConfig.TaskSetID]
	attachmentTemplate := `▲
|
|
|
%s
(unattached)`

	attachment := fmt.Sprintf(attachmentTemplate, truncateTo(lbConfig.TGName, taskSetWidth))

	return m.renderTaskSetWithAttachment(ts, attachment)
}

func (m Model) renderTaskSetWithAttachment(ts ecs.TaskSet, attachment string) string {
	content := m.renderTaskSetDetails(ts)

	attachment = lipgloss.NewStyle().AlignHorizontal(lipgloss.Center).Render(attachment)
	return lipgloss.JoinVertical(lipgloss.Center, smallSectionStyle.Copy().Height(10).Width(taskSetWidth).AlignHorizontal(lipgloss.Left).Render(content), attachment)

}

func (m Model) renderTaskSetDetails(ts ecs.TaskSet) string {
	taskCreation := *ts.CreatedAt
	taskDefinition := utils.GetLastItemAfterSplit(*ts.TaskDefinition, "/")
	status := *ts.Status
	taskIds := []table.Row{}
	for _, task := range m.ecsStatus.TaskSetTasks[*ts.Id] {
		taskIds = append(taskIds, table.Row{utils.GetLastItemAfterSplit(*task.TaskArn, "/"), *task.LastStatus})
	}
	tableStyles := table.DefaultStyles()
	tableStyles.Selected = lipgloss.NewStyle()
	taskTable := table.New(
		table.WithColumns([]table.Column{{Title: "id", Width: 10}, {Title: "status", Width: 10}}),
		table.WithRows(taskIds),
		table.WithHeight(len(taskIds)),
		table.WithFocused(false),
		table.WithStyles(tableStyles),
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		styles.Title.Copy().Padding(0).MarginBottom(1).Render(truncateTo(*ts.Id, taskSetWidth)),
		"created "+humanizer.Time(taskCreation),
		fmt.Sprintf("status: %s", status),
		fmt.Sprintf("steady: %s", *ts.StabilityStatus),
		fmt.Sprintf("\ntaskdef: %s", taskDefinition), strings.Join(m.ecsStatus.TaskSetImages[*ts.Id], "\n - "),
		"tasks:\n"+taskTable.View(),
	)

	return content
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
