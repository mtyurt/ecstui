package deployment

import (
	"fmt"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	humanizer "github.com/dustin/go-humanize"
	"github.com/mtyurt/ecstui/logger"
	"github.com/mtyurt/ecstui/spinnertui"
	"github.com/mtyurt/ecstui/types"
	"github.com/mtyurt/ecstui/utils"
)

var (
	styles = list.DefaultStyles()

	sectionWidth      = 40
	smallSectionStyle = lipgloss.NewStyle().
				Width(40).
				Height(6).
				Margin(0, 1, 0, 0).
				PaddingLeft(4).
				Align(lipgloss.Center).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"})
	subtle              = lipgloss.NewStyle().Foreground(lipgloss.Color("#9B9B9B"))
	bold                = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFBF00"))
	healthy             = lipgloss.NewStyle().Foreground(lipgloss.Color("#80C904"))
	unhealthy           = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBF00"))
	tgNameStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#ADD8E6")) // Amber
	refreshSpinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
)

type sessionState int

const (
	initial sessionState = iota
	loaded
	failed
)

type Model struct {
	images             map[string][]string
	connections        []types.ConnectionConfig
	tasks              map[string][]*ecs.Task
	deploymentMap      map[string]ecs.Deployment
	deployments        []*ecs.Deployment
	statusFetcher      DeploymentsFetcher
	width, height      int
	state              sessionState
	err                error
	spinner            spinnertui.Model
	refreshSpinner     spinner.Model
	showRefreshSpinner bool
}

type DeploymentsFetcher func(deployments []*ecs.Deployment) (*types.DeploymentStatus, error)

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

type StatusMsg *types.DeploymentStatus
type RefreshMsg struct{}

func New(statusFetcher DeploymentsFetcher, deployments []*ecs.Deployment, width, height int) *Model {
	deploymentMap := make(map[string]ecs.Deployment)
	for _, d := range deployments {
		deploymentMap[*d.Id] = *d
	}

	m := &Model{
		deployments:        deployments,
		deploymentMap:      deploymentMap,
		statusFetcher:      statusFetcher,
		state:              initial,
		spinner:            spinnertui.New("Loading deployments"),
		refreshSpinner:     spinner.New(spinner.WithSpinner(spinner.Hamburger), spinner.WithStyle(refreshSpinnerStyle)),
		showRefreshSpinner: false,
	}
	m.SetSize(width, height)
	return m
}

func (m *Model) SetSize(width, height int) {
	if width%2 == 0 {
		width = width - 1
	}

	m.width = width
	m.height = height
}

func (m Model) fetchStatus() tea.Msg {
	logger.Println("started fetching taskset status")
	defer logger.Println("finished fetching taskset status")
	taskSetStatus, err := m.statusFetcher(m.deployments)
	if err != nil {
		return errMsg{err}
	}
	return StatusMsg(taskSetStatus)
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetchStatus, m.spinner.SpinnerTick())
}

func (m Model) Refresh() (Model, tea.Cmd) {
	logger.Println("refreshing taskset status")
	m.showRefreshSpinner = true
	return m, tea.Batch(m.fetchStatus, m.refreshSpinner.Tick)

}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case StatusMsg:
		logger.Println("deployment status fetched")
		m.images = msg.DeploymentImages
		m.connections = msg.DeploymentConnections
		m.tasks = msg.DeploymentTasks
		m.state = loaded
		m.showRefreshSpinner = false
	case errMsg:
		m.err = msg.err
		m.state = failed
	}

	switch m.state {
	case initial:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.showRefreshSpinner {
		m.refreshSpinner, cmd = m.refreshSpinner.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	switch m.state {
	case initial:
		return m.spinner.View()
	case loaded:
		return m.renderView()
	case failed:
		return m.err.Error()
	default:
		return m.spinner.View()

	}
}

func (m *Model) renderView() string {
	deployments := m.renderDeployments()
	connections := m.renderConnections()
	return lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(lipgloss.JoinVertical(lipgloss.Center, deployments, connections))
}

func (m Model) renderDeployments() string {
	deployments := m.deployments
	slices.SortFunc(deployments, func(i, j *ecs.Deployment) int {
		return strings.Compare(*i.Id, *j.Id)
	})

	views := []string{}

	for _, d := range deployments {
		views = append(views, m.renderDeploymentDetails(*d))
	}

	return lipgloss.NewStyle().Width(m.width - 10).Align(lipgloss.Center).Render(lipgloss.JoinHorizontal(lipgloss.Center, views...))
}

func (m Model) renderConnections() string {
	connections := m.connections
	slices.SortFunc(connections, func(i, j types.ConnectionConfig) int {
		return strings.Compare(i.LBName, j.LBName)
	})

	views := []string{}
	for _, c := range connections {
		views = append(views, m.renderConnectionDetails(c))
	}

	return smallSectionStyle.Copy().Width(m.width - 10).Render(lipgloss.JoinHorizontal(lipgloss.Center, views...))

}
func truncateTo(s string, max int) string {
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}

func simpleAttachmentView() string {
	return `
▲
|
|
|

`
}

func (m Model) renderConnectionDetails(conn types.ConnectionConfig) string {
	lbName := smallSectionStyle.Copy().
		Width(sectionWidth + 20).
		Height(1).
		Render(styles.Title.AlignHorizontal(lipgloss.Center).Render(conn.LBName))

	tgInfo := getTGNameAndHealth(conn, sectionWidth-3)
	tgInfo = lipgloss.NewStyle().Height(1).AlignHorizontal(lipgloss.Center).Width(sectionWidth).Render(tgInfo)
	return lipgloss.JoinVertical(lipgloss.Center, tgInfo, lbName)
}

func getTGNameAndHealth(connConfig types.ConnectionConfig, width int) string {
	tgName := truncateTo(connConfig.TGName, width)
	tgName = tgNameStyle.Render(tgName)

	healths := []string{}
	azByState := make(map[string][]string)
	states := []string{}

	for _, health := range connConfig.TGHealth {
		az := *health.Target.AvailabilityZone
		state := *health.TargetHealth.State
		if _, ok := azByState[state]; !ok {
			azByState[state] = []string{utils.GetLastItemAfterSplit(az, "-")}
			states = append(states, state)
		} else {
			azByState[state] = append(azByState[state], utils.GetLastItemAfterSplit(az, "-"))
		}
	}
	slices.Sort(states)
	for _, state := range states {
		style := unhealthy
		if state == "healthy" {
			style = healthy
		}
		slices.Sort(azByState[state])
		azByState[state] = utils.UniqueStrings(azByState[state])
		healths = append(healths, style.Render(fmt.Sprintf("%s: %s", state, strings.Join(azByState[state], ", "))))
	}
	return tgName + "\n" + strings.Join(healths, " ")

}

func (m Model) renderDeploymentDetails(d ecs.Deployment) string {
	taskCreation := *d.CreatedAt
	taskDefinition := utils.GetLastItemAfterSplit(*d.TaskDefinition, "/")
	status := *d.Status
	taskIds := []table.Row{}
	for _, task := range m.tasks[*d.Id] {
		taskIds = append(taskIds, table.Row{utils.GetLastItemAfterSplit(*task.TaskArn, "/"), utils.MapTaskStatusToLabel(*task.LastStatus)})
	}
	tableStyles := table.DefaultStyles()
	tableStyles.Selected = lipgloss.NewStyle()
	tableStyles.Header = tableStyles.Header.Copy().PaddingLeft(1)
	taskTable := table.New(
		table.WithColumns([]table.Column{{Title: "id", Width: 10}, {Title: "status", Width: 30}}),
		table.WithRows(taskIds),
		table.WithHeight(len(taskIds)),
		table.WithFocused(false),
		table.WithStyles(tableStyles),
	)

	title := styles.Title.Copy().Padding(0).MarginBottom(0).Render(truncateTo(*d.Id, sectionWidth-2))
	if m.showRefreshSpinner {
		space := sectionWidth - lipgloss.Width(title) - lipgloss.Width(m.refreshSpinner.View())
		title = title + strings.Repeat(" ", space) + m.refreshSpinner.View()
	}
	title = title + "\n"
	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"created "+humanizer.Time(taskCreation),
		fmt.Sprintf("%s: %s", bold.Render("status"), status),
		fmt.Sprintf("%s: %s", bold.Render("rollout"), *d.RolloutState),
		fmt.Sprintf("\n%s: %s", bold.Render("taskdef"), taskDefinition), utils.JoinImageNames(m.images[*d.Id]),
		bold.Render("tasks:")+"\n"+taskTable.View(),
	)

	content = smallSectionStyle.Copy().Height(10).Width(sectionWidth).AlignHorizontal(lipgloss.Left).Render(content)
	attachment := "\n\n\n\n"
	if *d.Status == "PRIMARY" {
		attachment = simpleAttachmentView()

	}

	return lipgloss.JoinVertical(lipgloss.Center, content, attachment)
}
