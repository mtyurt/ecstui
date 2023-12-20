package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type serviceItem struct {
	title, desc string
	arn         string
}

func (i serviceItem) Title() string       { return i.title }
func (i serviceItem) Description() string { return i.desc }
func (i serviceItem) FilterValue() string { return i.title }

type sessionState int

const (
	listView sessionState = iota
	detailView
)

type mainModel struct {
	list             list.Model
	serviceDetailArn string
	state            sessionState
}

func (m mainModel) Init() tea.Cmd {
	return nil
}

func (m mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		} else if msg.Type == tea.KeyEnter {
			if item, ok := m.list.SelectedItem().(serviceItem); ok {
				m.serviceDetailArn = item.arn
			}
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m mainModel) View() string {
	return docStyle.Render(m.list.View())
}

func main() {
	awsLayer := NewAWSInteractionLayer()
	serviceList, err := awsLayer.FetchServiceList()
	if err != nil {
		fmt.Println("Error fetching service list:", err)
		os.Exit(1)
	}

	items := make([]list.Item, len(serviceList))
	for i, service := range serviceList {
		items[i] = serviceItem(service)
	}
	m := mainModel{list: list.New(items, list.NewDefaultDelegate(), 0, 0), state: listView}
	m.list.Title = "ECS services"
	m.list.SetFilteringEnabled(true)

	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
