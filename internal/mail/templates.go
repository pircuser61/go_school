package mail

import "fmt"

const (
	TaskUrlTemplate = "%s/applications/details/%s"
)

type Template struct {
	Subject   string
	Text      string
	Variables interface{}
}

type Notification struct {
	Template Template
	To       []string
}

func NewApprovementSLATemplate(id, name, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("По заявке %s %s истекло время согласования", id, name),
		Text:    "Истекло время согласования заявки {{.Name}}<br>Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>",
		Variables: struct {
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			Name: name,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

func NewExecutionSLATemplate(id, name, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("По заявке %s %s истекло время исполнения", id, name),
		Text:    "Истекло время исполнения заявки {{.Name}}<br>Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>",
		Variables: struct {
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			Name: name,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}
