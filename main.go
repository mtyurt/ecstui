package main

import (
	"fmt"
	"log"
	"os"

	"github.com/mtyurt/ecstui/spinnertui"
	listtui "github.com/mtyurt/ecstui/tui/list"
	servicetui "github.com/mtyurt/ecstui/tui/service"

	tea "github.com/charmbracelet/bubbletea"
)

type sessionState int

const (
	initialLoad sessionState = iota
	listView
	detailView
	fatalError
)

type mainModel struct {
	state         sessionState
	list          listtui.Model
	spinner       spinnertui.Model
	serviceDetail *servicetui.Model
	initialCall   func() tea.Msg
	err           error
	awsLayer      *AWSInteractionLayer
	logFile       *os.File
	width, height int
}

func (m mainModel) Init() tea.Cmd {
	return tea.Batch(m.initialCall, m.spinner.SpinnerTick())
}

func (m mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	newServiceDetail := false
	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Printf("keymsg: %v\n", msg)
		if m.state == fatalError {
			return m, tea.Quit
		}
		k := msg.String()
		if k == "ctrl+c" {
			return m, tea.Quit
		} else if msg.Type == tea.KeyEnter && m.state == listView {
			selectedService := m.list.GetSelectedServiceArn()
			m.state = detailView
			serviceDetail := servicetui.New(selectedService.Cluster(),
				selectedService.Service(),
				selectedService.ServiceArn(),
				m.awsLayer.FetchServiceStatus)
			serviceDetail.SetSize(m.width, m.height)
			m.serviceDetail = &serviceDetail
			cmds = append(cmds, m.serviceDetail.Init())
			newServiceDetail = true
		} else if m.state == detailView && (k == "esc" || k == "backspace") {
			m.state = listView
			m.serviceDetail = nil
		}
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
		if m.serviceDetail != nil {
			m.serviceDetail.SetSize(msg.Width, msg.Height)
		}

	case serviceListMsg:
		m.list.SetItems(msg)
		log.Println("serviceListMsg, setting state to list")
		m.state = listView
	case errMsg:
		m.err = msg
		m.state = fatalError
		return m, nil
	}

	var cmd tea.Cmd
	switch m.state {
	case initialLoad:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	case listView:
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	case detailView:
		log.Printf("update detailview with %v\n", msg)
		if !newServiceDetail {
			serviceDetail, cmd := m.serviceDetail.Update(msg)
			m.serviceDetail = &serviceDetail
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

func (m mainModel) View() string {
	switch m.state {
	case fatalError:
		return m.err.Error()
	case initialLoad:
		return m.spinner.View()
	case listView:
		return m.list.View()
	case detailView:
		return m.serviceDetail.View()
	default:
		return "View State Error"
	}
}

type serviceListMsg []listtui.ListItem

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

func newModel(logFile *os.File, initialCall func() tea.Msg, awsLayer *AWSInteractionLayer) mainModel {
	return mainModel{spinner: spinnertui.New("Loading Services..."),
		list:        listtui.New(),
		state:       initialLoad,
		initialCall: initialCall,
		logFile:     logFile,
		awsLayer:    awsLayer,
	}

}
func main() {
	awsLayer := NewAWSInteractionLayer()
	initialCall := func() tea.Msg {
		services, err := awsLayer.FetchServiceList()
		if err != nil {
			log.Println("error fetching service list")
			return errMsg{err}
		}
		items := make([]listtui.ListItem, len(services))
		for i, service := range services {
			items[i] = listtui.NewListItem(service.Service, service.Cluster, service.Arn)
		}
		return serviceListMsg(items)
	}

	f, _ := tea.LogToFile("log.txt", "debug")
	defer f.Close()
	m := newModel(f, initialCall, awsLayer)
	log.SetOutput(f)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
