package service

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mtyurt/ecstui/spinnertui"
)

type sessionState int

const (
	initial sessionState = iota
	loaded
	errorState
)

var (
	styles = list.DefaultStyles()
)

type section struct {
	content string
	title   string
}

type Model struct {
	state               sessionState
	cluster, serviceArn string
	serviceFetcher      func() tea.Msg
	spinner             spinnertui.Model
	ecsStatus           *ecs.Service
	err                 error
	sections            []section
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

func (m *Model) initializeSections() {
	events := ""
	for _, event := range m.ecsStatus.Events {
		events = events + fmt.Sprintf("%s %s\n", *event.CreatedAt, *event.Message)
	}
	m.sections = []section{
		{title: "task count", content: fmt.Sprintf("%d", *m.ecsStatus.RunningCount)},
		{title: "status", content: *m.ecsStatus.Status},
		{title: "events", content: events},
	}
}
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	log.Printf("servicedetail update: %v\n", msg)
	switch msg := msg.(type) {
	case serviceMsg:
		log.Printf("servicemsg received: %v\n", *msg)
		m.ecsStatus = msg
		m.initializeSections()
		m.state = loaded
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

func renderSection(title, content string) string {
	return styles.Title.Render(title) + "\n\n" + content + "\n"
}
func (m Model) gridView() string {

}
func (m Model) sectionsView() string {
	rows := [][]string{}
	i := 0
	for i < len(m.sections)/2 {
		// section1 :=
		// 	lipgloss.JoinHorizontal(lipgloss.Left)
		rows = append(rows, []string{renderSection(m.sections[i].title, m.sections[i].content)})
		rows = append(rows, []string{renderSection(m.sections[i+1].title, m.sections[i+1].content)})
		i += 2
	}
	if i < len(m.sections) {
		rows = append(rows, []string{renderSection(m.sections[i].title, m.sections[i].content)})
	}
	log.Println("rows")
	log.Println(rows)

	log.Println("sections")
	log.Println(m.sections)
	table := table.New().BorderRow(true).BorderColumn(true).Border(lipgloss.RoundedBorder()).Rows(rows...).String()
	log.Println("table")
	log.Println(table)
	return table
}

func (m Model) View() string {
	switch m.state {
	case initial:
		return m.spinner.View()
	case loaded:
		return m.sectionsView()
	case errorState:
		return m.err.Error()
	default:
		return m.serviceArn
	}
}
