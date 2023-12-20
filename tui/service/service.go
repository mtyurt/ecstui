package service

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/service/ecs"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mtyurt/ecstui/spinnertui"
)

type sessionState int

const (
	initial sessionState = iota
	loaded
	errorState
)

type Model struct {
	state               sessionState
	cluster, serviceArn string
	serviceFetcher      func() tea.Msg
	spinner             spinnertui.Model
	ecsStatus           *ecs.Service
	err                 error
}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

type serviceMsg *ecs.Service

func New(cluster, service, serviceArn string, ecsStatusFetcher func(string, string) (*ecs.Service, error)) Model {
	serviceFetcher := func() tea.Msg {
		log.Println("started fetching service status")
		defer log.Println("finished fetching service status")
		serviceConfig, err := ecsStatusFetcher(cluster, service)
		if err != nil {
			return errMsg{err}
		}
		return serviceMsg(serviceConfig)
	}
	return Model{cluster: cluster,
		serviceArn:     serviceArn,
		serviceFetcher: serviceFetcher,
		spinner:        spinnertui.New(fmt.Sprintf("Fetching %s status...", service))}
}

func (m *Model) SetSize(width, height int) {
}
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.serviceFetcher, m.spinner.SpinnerTick())
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	log.Printf("servicedetail update: %v\n", msg)
	switch msg := msg.(type) {
	case serviceMsg:
		log.Printf("servicemsg received: %v\n", *msg)
		m.state = loaded
		m.ecsStatus = msg
	case errMsg:
		m.err = msg
		m.state = errorState
	default:
		log.Printf("servicedetail update msg type: %s\n", msg)
	}

	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch m.state {
	case initial:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	default:
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	switch m.state {
	case initial:
		return m.spinner.View()
	case loaded:
		return fmt.Sprintf("%+v", m.ecsStatus)
	case errorState:
		return m.err.Error()
	default:
		return m.serviceArn
	}
}
