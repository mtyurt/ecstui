package events

import (
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
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

	filterPrompt = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#ECFD65"})

	filterCursor = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"})
)

type Model struct {
	eventsView    viewport.Model
	title         string
	events        []*ecs.ServiceEvent
	filterInput   textinput.Model
	filterEnabled bool
}

func New(title string, width, height int, events []*ecs.ServiceEvent) Model {
	view := viewport.New(width, height)
	view.SetYOffset(1)
	filterInput := textinput.New()
	filterInput.Prompt = "Filter: "
	filterInput.Width = width - 10 - len(filterInput.Prompt)
	filterInput.Cursor.Blink = false
	filterInput.Cursor.SetMode(cursor.CursorHide)
	filterInput.Focus()
	filterInput.KeyMap.DeleteCharacterBackward = key.NewBinding(key.WithKeys("backspace", "ctrl+h"))

	m := Model{
		eventsView: view,
		title:      title,
		events:     events,
	}

	m.updateContent()
	return m
}

func wrapEventMessage(message string, width, padding int) string {
	wrapPrefix := strings.Repeat(" ", padding)
	wrapped := wordwrap.String(message, width)
	lines := strings.Split(wrapped, "\n")
	return strings.Join(lines, "\n"+wrapPrefix)
}

func (m *Model) updateContent() {
	log.Println("filterinput updating content")
	width := m.eventsView.Width
	var summary []string
	for _, event := range m.events {
		if m.filterEnabled && !strings.Contains(strings.ToLower(*event.Message), strings.ToLower(m.filterInput.Value())) {
			continue
		}

		msg := wrapEventMessage(*event.Message, width-25, 24)
		timestamp := timestampStyle.Render(event.CreatedAt.Format("2006-01-02 15:04:05.000"))
		summary = append(summary, fmt.Sprintf("%s %s", timestamp, msg))
	}
	content := strings.Join(summary, "\n")
	m.eventsView.SetContent(content)
}

func (m Model) SetSize(width, height int) {
	m.eventsView.Width = width
	m.eventsView.Height = height
}
func (m Model) Focused() bool {
	return !m.filterEnabled
}
func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) clearFilter() {
	m.filterInput.SetValue("")
	m.updateContent()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := []tea.Cmd{}

	newFilter := false
	var keyMsg tea.KeyMsg
	if msg, ok := msg.(tea.KeyMsg); ok {
		keyMsg = msg
		if !m.filterEnabled {
			switch msg.String() {
			case "/":
				m.filterEnabled = true
				m.filterInput.Cursor.SetMode(cursor.CursorHide)
				m.filterInput.Focus()
				m.filterInput.KeyMap.DeleteCharacterBackward.SetEnabled(true)

				newFilter = true
			}
		} else {
			switch msg.String() {
			case "esc":
				m.filterEnabled = false
				m.clearFilter()
			case "ctrl+l":
				m.clearFilter()
			// case "backspace":
			// 	m.filterInput.DeleteCharacterBackward()
			default:
				m.updateContent()
			}
		}
	}

	if m.filterEnabled && !newFilter {
		log.Println("m.filterInput.Update", msg)
		log.Println("filterinput position", m.filterInput.Position())
		log.Println("filterinput value", m.filterInput.Value())
		log.Println("filterinput focus", m.filterInput.Focused())
		log.Println("filterinput keymsg", keyMsg)
		log.Println("backspace key", m.filterInput.KeyMap.DeleteCharacterBackward)
		log.Println("backspace keymap", m.filterInput.KeyMap)
		log.Println("key matches backspace", key.Matches(keyMsg, m.filterInput.KeyMap.DeleteCharacterBackward))
		m.filterInput.KeyMap.DeleteCharacterBackward.SetEnabled(true)
		log.Println("key matches backspace enabled", m.filterInput.KeyMap.DeleteCharacterBackward.Enabled())
		newFilterInputModel, inputCmd := m.filterInput.Update(msg)
		m.filterInput = newFilterInputModel
		cmds = append(cmds, inputCmd)
	} else {
		m.eventsView, cmd = m.eventsView.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) headerView() string {
	if m.filterEnabled {
		view := titleStyle.Render("Filter: " + m.filterInput.View())
		line := strings.Repeat("─", max(0, m.eventsView.Width-lipgloss.Width(view)))
		return lipgloss.JoinHorizontal(lipgloss.Center, view, line)
	}
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
