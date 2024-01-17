package taskset

import (
	"fmt"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/charmbracelet/bubbles/list"
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

	taskSetWidth      = 35
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
	bold        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFBF00"))
	healthy     = lipgloss.NewStyle().Foreground(lipgloss.Color("#80C904"))
	unhealthy   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBF00"))
	tgNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ADD8E6")) // Amber

)

type sessionState int

const (
	initial sessionState = iota
	loaded
	failed
)

type Model struct {
	images        map[string][]string
	connections   map[string][]types.ConnectionConfig
	tasks         map[string][]*ecs.Task
	taskSetMap    map[string]ecs.TaskSet
	taskSets      []*ecs.TaskSet
	statusFetcher StatusFetcher
	width, height int
	state         sessionState
	err           error
	spinner       spinnertui.Model
}

type StatusFetcher func(taskSets []*ecs.TaskSet) (*types.TaskSetStatus, error)

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

type StatusMsg *types.TaskSetStatus

func New(statusFetcher StatusFetcher, taskSets []*ecs.TaskSet, width, height int) *Model {
	taskSetMap := make(map[string]ecs.TaskSet)
	for _, ts := range taskSets {
		taskSetMap[*ts.Id] = *ts
	}

	m := &Model{
		taskSets:      taskSets,
		taskSetMap:    taskSetMap,
		statusFetcher: statusFetcher,
		state:         initial,
		spinner:       spinnertui.New("Loading tasksets"),
	}
	m.SetSize(width, height)
	return m
}

func (m *Model) SetSize(width, height int) {
	if width%2 == 1 {
		width = width - 1
	}

	m.width = width
	m.height = height
}

func (m Model) fetchStatus() tea.Msg {
	logger.Println("started fetching taskset status")
	defer logger.Println("finished fetching taskset status")
	taskSetStatus, err := m.statusFetcher(m.taskSets)
	if err != nil {
		return errMsg{err}
	}
	return StatusMsg(taskSetStatus)

}
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetchStatus, m.spinner.SpinnerTick())
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	logger.Printf("taskset update %v\n", msg)
	switch msg := msg.(type) {
	case StatusMsg:
		m.images = msg.TaskSetImages
		m.connections = msg.TaskSetConnections
		m.tasks = msg.TaskSetTasks
		m.state = loaded
	case errMsg:
		m.err = msg.err
		m.state = failed
	}

	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch m.state {
	case initial:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	switch m.state {
	case initial:
		return m.spinner.View()
	case loaded:
		return m.renderTaskSetsWithConnections()
	case failed:
		return m.err.Error()
	default:
		return m.spinner.View()

	}
}

type taskSetView struct {
	tsID string
	view string
}

func (m *Model) renderTaskSetsWithConnections() string {
	viewByConn := make(map[string][]taskSetView)
	connByTaskSet := make(map[string]*types.ConnectionConfig)
	for taskSetID, conns := range m.connections {
		priorities := []string{}
		for _, conn := range conns {
			if conn.LBName != "" && conn.Priority != "" {
				priorities = append(priorities, conn.Priority)
			}
		}
		connByTaskSet[taskSetID] = &types.ConnectionConfig{
			LBName:    conns[0].LBName,
			Priority:  strings.Join(priorities, ","),
			TGName:    conns[0].TGName,
			TGWeigth:  conns[0].TGWeigth,
			TaskSetID: taskSetID,
			TGHealth:  conns[0].TGHealth,
		}
	}

	unattachedTaskSets := []taskSetView{}
	for _, conn := range connByTaskSet {

		if conn.LBName != "" {
			view := m.renderAttachedTaskSet(*conn)

			if _, ok := viewByConn[conn.LBName]; !ok {
				viewByConn[conn.LBName] = []taskSetView{{tsID: conn.TaskSetID, view: view}}
			} else {
				viewByConn[conn.LBName] = append(viewByConn[conn.LBName], taskSetView{tsID: conn.TaskSetID, view: view})
			}
		} else {
			view := m.renderUnattachedTaskSet(*conn)
			unattachedTaskSets = append(unattachedTaskSets, taskSetView{tsID: conn.TaskSetID, view: view})
		}
	}
	connViews := make(map[string]string)
	for lbName, lbTaskSets := range viewByConn {
		bottom := smallSectionStyle.Copy().Width(len(lbTaskSets)*taskSetWidth + 2).Height(1).Render(styles.Title.AlignHorizontal(lipgloss.Center).Render(lbName))
		slices.SortFunc(lbTaskSets, func(i, j taskSetView) int {
			return strings.Compare(i.tsID, j.tsID)
		})

		viewStrings := []string{}
		for _, lbView := range lbTaskSets {
			viewStrings = append(viewStrings, lbView.view)
		}

		tgView := lipgloss.JoinVertical(lipgloss.Top, lipgloss.JoinHorizontal(lipgloss.Top, viewStrings...), bottom)
		connViews[lbName] = tgView
	}
	conns := make([]string, 0, len(viewByConn))
	for _, conn := range connViews {
		conns = append(conns, conn)
	}
	for _, unattachedTaskSet := range unattachedTaskSets {
		conns = append(conns, unattachedTaskSet.view)
	}

	return lipgloss.NewStyle().Width(m.width).AlignHorizontal(lipgloss.Center).AlignVertical(lipgloss.Top).Render(lipgloss.JoinHorizontal(lipgloss.Right, conns...))
}

func truncateTo(s string, max int) string {
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}

func (m Model) renderAttachedTaskSet(connConfig types.ConnectionConfig) string {
	ts := m.taskSetMap[connConfig.TaskSetID]
	attachmentTemplate := `▲
|
%d%%
|
%s
priority: %s`

	attachment := fmt.Sprintf(attachmentTemplate, connConfig.TGWeigth, getTGNameAndHealth(connConfig), connConfig.Priority)
	return m.renderTaskSetWithAttachment(ts, attachment)
}

func (m Model) renderUnattachedTaskSet(connConfig types.ConnectionConfig) string {
	ts := m.taskSetMap[connConfig.TaskSetID]
	attachmentTemplate := `▲
|
|
|
%s
(unattached)

`

	attachment := fmt.Sprintf(attachmentTemplate, getTGNameAndHealth(connConfig))

	return m.renderTaskSetWithAttachment(ts, attachment)
}

func getTGNameAndHealth(connConfig types.ConnectionConfig) string {
	tgName := truncateTo(connConfig.TGName, taskSetWidth)
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
		healths = append(healths, style.Render(fmt.Sprintf("%s: %s", state, strings.Join(azByState[state], ", "))))
	}
	return tgName + "\n" + strings.Join(healths, " ")

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
	for _, task := range m.tasks[*ts.Id] {
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

	content := lipgloss.JoinVertical(lipgloss.Left,
		styles.Title.Copy().Padding(0).MarginBottom(1).Render(truncateTo(*ts.Id, taskSetWidth)),
		"created "+humanizer.Time(taskCreation),
		fmt.Sprintf("%s: %s", bold.Render("status"), status),
		fmt.Sprintf("%s: %s", bold.Render("steady"), *ts.StabilityStatus),
		fmt.Sprintf("\n%s: %s", bold.Render("taskdef"), taskDefinition), utils.JoinImageNames(m.images[*ts.Id]),
		bold.Render("tasks:")+"\n"+taskTable.View(),
	)

	return content
}
