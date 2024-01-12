package taskset

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	humanizer "github.com/dustin/go-humanize"
	"github.com/mtyurt/ecstui/types"
	"github.com/mtyurt/ecstui/utils"
)

var (
	styles = list.DefaultStyles()

	taskSetWidth      = 32
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
	bold = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFBF00"))
)

type Model struct {
	images        map[string][]string
	connections   map[string][]types.LbConfig
	tasks         map[string][]*ecs.Task
	taskSetMap    map[string]ecs.TaskSet
	width, height int
}

func New(images map[string][]string, connections map[string][]types.LbConfig, tasks map[string][]*ecs.Task, taskSets []*ecs.TaskSet, width, height int) *Model {
	taskSetMap := make(map[string]ecs.TaskSet)
	for _, ts := range taskSets {
		taskSetMap[*ts.Id] = *ts
	}

	return &Model{
		images:      images,
		connections: connections,
		tasks:       tasks,
		taskSetMap:  taskSetMap,
		width:       width,
		height:      height,
	}
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}
func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	return m, nil
}

func (m Model) View() string {
	return m.renderLbConfigs(m.connections)
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
	lbs = append(lbs, unattachedTaskSets...)

	return lipgloss.JoinHorizontal(lipgloss.Top, lbs...)
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
(unattached)

`

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
