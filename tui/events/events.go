package events

import (
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	infoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return titleStyle.Copy().BorderStyle(b)
	}()

	timestampStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "236", Dark: "248"})
)

type Model struct {
	eventsView viewport.Model
	title      string
	events     []*ecs.ServiceEvent
}

func New(title string, width, height int, events []*ecs.ServiceEvent) Model {
	view := viewport.New(width, height)
	view.SetYOffset(1)
	view.SetContent(getAllServiceEvents(events, width))
	return Model{
		eventsView: view,
		title:      title,
		events:     events,
	}
}

func wrapEventMessage(message string, width, padding int) string {
	wrapPrefix := strings.Repeat(" ", padding)
	wrapped := wordwrap.String(message, width)
	lines := strings.Split(wrapped, "\n")
	return strings.Join(lines, "\n"+wrapPrefix)
}

func getAllServiceEvents(events []*ecs.ServiceEvent, width int) string {
	var summary []string
	for _, event := range events {
		msg := wrapEventMessage(*event.Message, width-25, 25)
		timestamp := timestampStyle.Render(event.CreatedAt.Format("2006-01-02 15:04:05.000"))
		summary = append(summary, fmt.Sprintf("%s %s", timestamp, msg))
	}
	return strings.Join(summary, "\n")
}

func (m Model) SetSize(width, height int) {
	m.eventsView.Width = width
	m.eventsView.Height = height
}
func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.eventsView, cmd = m.eventsView.Update(msg)
	log.Println("events view update", msg)
	return m, cmd
}

func (m Model) headerView() string {
	title := titleStyle.Render(m.title)
	line := strings.Repeat("─", max(0, m.eventsView.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m Model) footerView() string {
	info := infoStyle.Render(fmt.Sprintf("%3.f%%", m.eventsView.ScrollPercent()*100))
	line := strings.Repeat("─", max(0, m.eventsView.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func (m Model) View() string {
	return lipgloss.NewStyle().Margin(5, 1).Render(
		fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.eventsView.View(), m.footerView()))
}
