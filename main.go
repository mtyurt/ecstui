package main

import (
	"fmt"
	"os"

	"github.com/mtyurt/ecstui/spinnertui"

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
	initialLoad sessionState = iota
	listView
	detailView
)

type mainModel struct {
	list             list.Model
	serviceDetailArn string
	state            sessionState
	initialCall      func() tea.Msg
	err              error
	spinner          spinnertui.Model
}

func (m mainModel) Init() tea.Cmd {
	return tea.Batch(m.initialCall, m.spinner.Spinner.Tick)
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
	case serviceListMsg:
		m.list.SetItems(msg)
		m.state = listView
	case errMsg:
		m.err = msg
		return m, tea.Quit
	}

	var cmd tea.Cmd
	switch m.state {
	case initialLoad:
		updateSpinner, spinnerCmd := m.spinner.Update(msg)
		switch updateSpinner := updateSpinner.(type) {
		case spinnertui.Model:
			m.spinner = updateSpinner
			cmd = spinnerCmd
		}
	case listView:
		m.list, cmd = m.list.Update(msg)
	}
	return m, cmd
}

func (m mainModel) View() string {
	switch m.state {
	case initialLoad:
		return m.spinner.View()
	case listView:
		return docStyle.Render(m.list.View())
	default:
		return "View State Error"
	}
}

type serviceListMsg []list.Item

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

func newModel(initialCall func() tea.Msg) mainModel {
	listModel := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	return mainModel{spinner: spinnertui.New(), list: listModel, state: initialLoad, initialCall: initialCall}
}
func main() {
	awsLayer := NewAWSInteractionLayer()
	initialCall := func() tea.Msg {
		services, err := awsLayer.FetchServiceList()
		if err != nil {
			return errMsg{err}
		}
		items := make([]list.Item, len(services))
		for i, service := range services {
			items[i] = serviceItem(service)
		}
		return serviceListMsg(items)
	}

	m := newModel(initialCall)
	m.list.Title = "ECS services"
	m.list.SetFilteringEnabled(true)

	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
