package ui

import (
	"fmt"
	"os"
	"io"
	// "strings"
	"io/ioutil"
	"github.com/tidwall/gjson"
	// "github.com/tidwall/sjson"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const listHeight = 15
const defaultWidth = 20

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	quitTextStyle     = lipgloss.NewStyle().Margin(1, 0, 2, 4)
	focusedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle         = focusedStyle.Copy()
	noStyle             = lipgloss.NewStyle()
	cursorModeHelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	focusedButton = focusedStyle.Copy().Render("[ Submit ]")
	blurredButton = fmt.Sprintf("[ %s ]", blurredStyle.Render("Submit"))
)

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

type Model struct {
	modelType			string
	content     		[]byte
	listItems			[]map[string]interface{}
	list				list.Model
	listSelectedIndex 	int
	listSelected		string
	editMode			bool
	quit				bool

	inputItem			map[string]interface{}
	inputs				[]textinput.Model
	inputSelectedIndex  int
}

func (m Model) initialListModel(key string) Model {
	m.modelType = "list"
	
	gjson.Get(string(m.content), key).ForEach(func(k, v gjson.Result) bool {
		m.listItems = append(m.listItems, v.Value().(map[string]interface{}))
		return true
	})
	lItems := make([]list.Item, len(m.listItems))
	if key == "tags" {
		for i := 0; i < len(m.listItems); i++ {
			lItems[i] = listItem(m.listItems[i]["tag"].(string))
		}
		m.list = list.New(lItems, itemDelegate{}, defaultWidth, listHeight)
		m.list.Title = "choose tag of host to connect or edit..."
	} else if key == "passwords" {
		m.listSelected = gjson.Get(string(m.content), `passwords.#(default="true").password`).String()
		listSelectedIndex := 0
		for i := 0; i < len(m.listItems); i++ {
			password := m.listItems[i]["password"].(string)
			if password == m.listSelected {
				listSelectedIndex = i+1
			}
			lItems[i] = listItem(password)
		}
		m.list = list.New(lItems, itemDelegate{}, defaultWidth, listHeight)
		if listSelectedIndex != 0 {
			m.list.Select(listSelectedIndex)
		}
		m.list.Title = "set the selected content as default password..."
	}
	return m
}

func (m Model) initialInputModel(key string) Model {
	m.modelType = "input"
	new := m.listSelected == ""
	if key == "tags" {
		// 遍历找到编辑项
		if !new {
			for _, i := range m.listItems {
				if m.listSelected == i["tag"].(string) {
					m.inputItem = i
					break
				}
			}
			if len(m.inputItem) == 0 {
				return m.initialListModel(key)
			} 
		}
		m.inputs = make([]textinput.Model, 5)
		for i := range m.inputs {
			t := textinput.New()
			t.CursorStyle = cursorStyle
			t.CharLimit = 32
			switch i {
			case 0:
				t.Placeholder = "Tag"
				t.Focus()
				t.PromptStyle = focusedStyle
				t.TextStyle = focusedStyle
				t.CharLimit = 64
				if new == false {
					t.SetValue(m.inputItem["tag"].(string))
				}
			case 1:
				t.Placeholder = "Address"
				t.CharLimit = 64
				if new == false {
					t.SetValue(m.inputItem["address"].(string))
				}
			case 2:
				t.Placeholder = "Port"
				if new == false {
					t.SetValue(m.inputItem["port"].(string))
				}	
			case 3:
				t.Placeholder = "Username"
				if new == false {
					t.SetValue(m.inputItem["username"].(string))
				}
			case 4:
				t.Placeholder = "Password"
				t.EchoMode = textinput.EchoPassword
				t.EchoCharacter = '•'
				if new == false {
					t.SetValue(m.inputItem["password"].(string))
				}
			}
			m.inputs[i] = t
		}
	}else if key == "passwords" {
		if !new {
			for _, i := range m.listItems {
				if m.listSelected == i["password"].(string) {
					m.inputItem = i
					break
				}
			}
			if len(m.inputItem) == 0 {
				return m.initialListModel(key)
			} 
		}
		m.inputs = make([]textinput.Model, 1)
		for i := range m.inputs {
			t := textinput.New()
			t.CursorStyle = cursorStyle
			t.CharLimit = 32
			switch i {
			case 0:
				t.Placeholder = "Password"
				t.Focus()
				t.PromptStyle = focusedStyle
				t.TextStyle = focusedStyle
				t.CharLimit = 64
				if new == false {
					t.SetValue(m.inputItem["passowrd"].(string))
				}
			}
			m.inputs[i] = t
		}

	}
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.modelType == "list" {
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
					if ok {
						m.editMode = true
						return m, nil
					}
			}
		} 
	}else if m.modelType == "input" {
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
					if ok {
						m.listSelected = string(i)
					}
					m.editMode = true
					return m, nil
			}
		}
	} else {
		panic("a serious mistake has occurred...")
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

func main() {
	var m Model
	m.content, _ = ioutil.ReadFile(".axe2.config.json")
	if _, err := tea.NewProgram(m.initialListModel("tags")).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
