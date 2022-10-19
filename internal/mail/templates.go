package mail

import (
	"fmt"
)

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

type ExecutorNotifTemplate struct {
	Id, SdUrl, ExecutorName, Initiator, Description string
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

func NewRequestExecutionInfoTemplate(id, name, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("Заявка %s запрос дополнительной информации", id),
		Text: `Уважаемый коллега, по заявке {{.Id}} требуется дополнительная информация<br>
				Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>`,
		Variables: struct {
			Id   string `json:"id"`
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			Id:   id,
			Name: name,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

func NewRequestFormExecutionTemplate(id, name, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("Заявка %s запрос дополнительной информации", id),
		Text: `Уважаемый коллега, по заявке {{.Id}} требуется дополнительная информация<br>
				Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>`,
		Variables: struct {
			Id   string `json:"id"`
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			Id:   id,
			Name: name,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

func NewRequestApproverInfoTemplate(id, name, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("Заявка %s запрос дополнительной информации", id),
		Text: `Уважаемый коллега, по заявке № {{.Id}} требуется дополнительная информация<br>
				Для просмотра перейдите по <a href={{.Link}}>ссылке</a>`,
		Variables: struct {
			Id   string `json:"id"`
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			Id:   id,
			Name: name,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

func NewAnswerApproverInfoTemplate(id, name, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("Заявка %s запрос дополнительной информации", id),
		Text: `Уважаемый коллега, по заявке № {{.Id}} была получена дополнительная информация<br>
				Для просмотра перейдите по <a href={{.Link}}>ссылке</a>`,
		Variables: struct {
			Id   string `json:"id"`
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			Id:   id,
			Name: name,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

func NewAnswerExecutionInfoTemplate(id, name, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("Заявка %s  получена дополнительная информация", id),
		Text: `Уважаемый коллега, по заявке {{.Id}} была получена дополнительная информация<br>
				Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>`,
		Variables: struct {
			Id   string `json:"id"`
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			Id:   id,
			Name: name,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

func NewApplicationInitiatorStatusNotification(id, name, action, description, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("Заявка %s %s", id, action),
		Text: `Уважаемый коллега, заявка {{.Id}} <b>{{.Action}}</b><br>
				Для просмотра перейдите по <a href={{.Link}}>ссылке</a><br>
				Текст заявки:<br><br>
				<pre style="white-space: pre-wrap; word-break: keep-all; font-family: inherit;">{{.Description}}</pre>`,
		Variables: struct {
			Id          string `json:"id"`
			Name        string `json:"name"`
			Link        string `json:"link"`
			Action      string `json:"action"`
			Description string `json:"description"`
		}{
			Id:          id,
			Name:        name,
			Link:        fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			Action:      action,
			Description: description,
		},
	}
}

func NewApplicationPersonStatusNotification(id, name, action, deadline, description, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("Заявка %s ожидает %s", id, action),
		Text: `Уважаемый коллега, заявка {{.Id}} <b>ожидает {{.Action}}</b><br>
				Для просмотра перейдите по <a href={{.Link}}>ссылке</a><br>
				Срок {{.Action}} до {{.Deadline}}<br>
				Текст заявки:<br><br>
				<pre style="white-space: pre-wrap; word-break: keep-all; font-family: inherit;">{{.Description}}</pre>`,
		Variables: struct {
			Id          string `json:"id"`
			Name        string `json:"name"`
			Link        string `json:"link"`
			Action      string `json:"action"`
			Deadline    string `json:"deadline"`
			Description string `json:"description"`
		}{
			Id:          id,
			Name:        name,
			Link:        fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			Action:      action,
			Deadline:    deadline,
			Description: description,
		},
	}
}

func NewAnswerSendToEditTemplate(id, name, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("Заявка %s  требует доработки", id),
		Text: `Уважаемый коллега, заявка {{.Id}} <b>требует доработки.</b><br>
				Для просмотра перейти по <a href={{.Link}}>ссылке</a>`,
		Variables: struct {
			Id   string `json:"id"`
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			Id:   id,
			Name: name,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

func NewExecutionTakenInWork(dto *ExecutorNotifTemplate) Template {
	return Template{
		Subject: fmt.Sprintf("Заявка №%s взята в работу пользователем %s", dto.Id, dto.ExecutorName),
		Text: `<p>Уважаемый коллега, заявка {{.Id}} <b>взята в работу</b> пользователем <b>{{.Executor}}</b></br>
 <b>Инициатор: </b>{{.Initiator}}</br>
 <b>Ссылка на заявку: </b><a href={{.Link}}>{{.Link}}</a></br>
 ------------ Описание ------------  </br>
<pre style="white-space: pre-wrap; word-break: keep-all; font-family: inherit;">{{.Description}}</pre>

<style>
    p {
        font-family: : Arial;
        font-size: 11px;
        margin-bottom: -20px;
    }
</style>`,
		Variables: struct {
			Id          string `json:"id"`
			Executor    string `json:"executor"`
			Link        string `json:"link"`
			Initiator   string `json:"initiator"`
			Description string `json:"description"`
		}{
			Id:          dto.Id,
			Executor:    dto.ExecutorName,
			Link:        fmt.Sprintf(TaskUrlTemplate, dto.SdUrl, dto.Id),
			Initiator:   dto.Initiator,
			Description: dto.Description,
		},
	}
}
