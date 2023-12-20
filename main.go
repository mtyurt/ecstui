package main

import (
	"fmt"
	"os"

	"github.com/mtyurt/ecstui/spinnertui"
	listtui "github.com/mtyurt/ecstui/tui/list"

	tea "github.com/charmbracelet/bubbletea"
)

type sessionState int

const (
	initialLoad sessionState = iota
	listView
	detailView
)

type mainModel struct {
	list        listtui.Model
	state       sessionState
	initialCall func() tea.Msg
	err         error
	spinner     spinnertui.Model
}

func (m mainModel) Init() tea.Cmd {
	return tea.Batch(m.initialCall, m.spinner.Spinner.Tick)
}

func (m mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:

		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)

	case serviceListMsg:
		m.list.SetItems(msg)
		m.state = listView
	case errMsg:
		m.err = msg
		return m, tea.Quit
	}

	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch m.state {
	case initialLoad:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	case listView:
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m mainModel) View() string {
	switch m.state {
	case initialLoad:
		return m.spinner.View()
	case listView:
		return m.list.View()
	default:
		return "View State Error"
	}
}

type serviceListMsg []listtui.ListItem

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

func newModel(initialCall func() tea.Msg) mainModel {
	return mainModel{spinner: spinnertui.New(), list: listtui.New(), state: initialLoad, initialCall: initialCall}
}
func main() {
	awsLayer := NewAWSInteractionLayer()
	initialCall := func() tea.Msg {
		services, err := awsLayer.FetchServiceList()
		if err != nil {
			return errMsg{err}
		}
		items := make([]listtui.ListItem, len(services))
		for i, service := range services {
			items[i] = listtui.NewListItem(service.Service, service.Cluster, service.Arn)
		}
		return serviceListMsg(items)
	}

	m := newModel(initialCall)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
