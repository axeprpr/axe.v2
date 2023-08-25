package ui

import (
	"fmt"
	"os"
	"io"
	"io/ioutil"
	"github.com/tidwall/gjson"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const listHeight = 15
const defaultWidth = 20

type listItem string
func (i listItem) FilterValue() string { return "" }

type itemDelegate struct{}
func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, lt list.Item) {
	i, ok := lt.(listItem)
	if !ok {
		return
	}
	str := fmt.Sprintf("%d. %s", index+1, i)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s string) string {
			return selectedItemStyle.Render("> " + s + " ")
		}
	}
	fmt.Fprint(w, fn(string(str)))
}

type tagListModel struct {
	content     		[]byte
	state				string
	listItems			[]map[string]interface{}
	list				list.Model
	selectedItem		map[string]interface{}
	editMode			bool
	quit				bool
}

func (m tagListModel) initialModel(key string) Model {
	m.modelType = "list"
	
	gjson.Get(string(m.content), key).ForEach(func(k, v gjson.Result) bool {
		m.listItems = append(m.listItems, v.Value().(map[string]interface{}))
		return true
	})
	lItems := make([]list.Item, len(m.listItems))
	
	for i := 0; i < len(m.listItems); i++ {
		lItems[i] = listItem(m.listItems[i]["tag"].(string))
	}
	m.list = list.New(lItems, itemDelegate{}, defaultWidth, listHeight)
	m.list.Title = "choose tag of host to connect or edit..."
	return m
}

func (m tagListModel) Init() tea.Cmd {
	return nil
}

func (m tagListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c":
			m.quit = true
			return m, tea.Quit

		case "e":
			i, ok := m.list.SelectedItem().(listItem)
			m.edit = true
			return m, tea.Quit
		}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.quit {
		return quitTextStyle.Render("bye")
	}
	if m.listSelected != "" && m.editMode == true {
	var b strings.Builder
	m.initialInputModel(m.key, false)
	
	for i := range m.inputs {
		b.WriteString(m.inputs[i].View())
		if i < len(m.inputs)-1 {
			b.WriteRune('\n')
		}
	}
	button := &blurredButton
	if m.focusIndex == len(m.inputs) {
		button = &focusedButton
	}
	fmt.Fprintf(&b, "\n\n%s\n\n", *button)
	return b.String()
	}
	return m.list.View()
}

func (m Model) Run() {
	var m Model
	m.content, _ = ioutil.ReadFile(".axe2.config.json")
	if _, err := tea.NewProgram(m.initialListModel("tags")).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
