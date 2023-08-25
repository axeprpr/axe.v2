package ui

const (
	tagsList modelType = iota
	tagEdit
)

type tagModel struct {
	modelType 			modelType
	content     		[]byte
	listItems			[]map[string]interface{}
	listSelected		string
	list				list.Model
	
	inputItem			map[string]interface{}
	newItem				bool
	inputs				[]textinput.Model
}

func (m tagModel) initialListModel() tagModel {
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


func (m tagModel) initialEditModel() tagModel {
	new := m.newItem
	if new == false {
		for _, i := range m.listItems {
			if m.listSelected == i["tag"].(string) {
				m.inputItem = i
				break
			}
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
}

func (m tagModel) initialModel() tagModel {
	switch m.modelType {
	case tagsList:
		return m.initialListModel()

	case tagEdit:
		return m.initialInputModel()

	default:
		return m
	}
}

func (m tagModel) listUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
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
}

func (m tagModel) editUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
}

func (m tagModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.modelType {
	case tagsList:
		return m.listUpdate(msg, m)

	case tagEdit:
		return m.editUpdate(msg, m)

	default:
		return m, nil
	}
}

func (m tagModel) View() string {
	switch m.modelType {
	case tagsList:
		return m.listView()

	case tagEdit:
		return m.editView()

	default:
		return ""
	}
}

func (m tagModel) Run(content []byte) {
	m.content = content
	if _, err := tea.NewProgram(m.initialModel()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}