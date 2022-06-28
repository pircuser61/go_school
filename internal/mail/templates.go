package mail

type Template struct {
	Subject   string
	Text      string
	Variables interface{}
}

var TestTemplate = Template{
	Subject: "Тестовое уведомление",
	Text:    "Тестовое уведомление с переменной {{.Var}}",
	Variables: struct {
		Var string `json:"var"`
	}{
		Var: "variable value",
	},
}
