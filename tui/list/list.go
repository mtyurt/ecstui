package list

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ListItem struct {
	title, desc string
	arn         string
}

func NewListItem(title, desc, arn string) ListItem {
	return ListItem{title: title, desc: desc, arn: arn}
}

var docStyle = lipgloss.NewStyle().Margin(1, 2)

func (i ListItem) Title() string       { return i.title }
func (i ListItem) Description() string { return i.desc }
func (i ListItem) FilterValue() string { return i.title }

type Model struct {
	list list.Model
}

func New() Model {
	list := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	list.Title = "ECS Services"
	list.SetFilteringEnabled(true)
	return Model{list}
}
func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	return docStyle.Render(m.list.View())
}

func (m *Model) SetSize(width, height int) {
	h, v := docStyle.GetFrameSize()
	m.list.SetSize(width-h, height-v)
}
func (m *Model) SetItems(services []ListItem) {
	items := make([]list.Item, len(services))
	for i, service := range services {
		items[i] = ListItem(service)
	}
	m.list.SetItems(items)
}
