package mail

import (
	"fmt"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

const (
	defaultApprovementActionName = "согласования"

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

func NewApprovementSLATemplate(id, name, sdUrl, status string) Template {
	actionName := getApprovementActionNameByStatus(status, defaultApprovementActionName)
	return Template{
		Subject: fmt.Sprintf("По заявке %s %s истекло время %s", id, name, actionName),
		Text:    "Истекло время {{.ActionName}} заявки {{.Name}}<br>Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>",
		Variables: struct {
			Name       string `json:"name"`
			Link       string `json:"link"`
			ActionName string `json:"actionName"`
		}{
			Name:       name,
			Link:       fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			ActionName: actionName,
		},
	}
}

func NewApprovementHalfSLATemplate(id, name, sdUrl, status string) Template {
	actionName := getApprovementActionNameByStatus(status, defaultApprovementActionName)
	return Template{
		Subject: fmt.Sprintf("По заявке %s %s истекает время %s", id, name, actionName),
		Text:    "Истекает время {{.ActionName}} заявки {{.Name}}<br>Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>",
		Variables: struct {
			Name       string `json:"name"`
			Link       string `json:"link"`
			ActionName string `json:"actionName"`
		}{
			Name:       name,
			Link:       fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			ActionName: actionName,
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

func NewExecutiontHalfSLATemplate(id, name, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("По заявке %s %s истекает время исполнения", id, name),
		Text:    "Истекает время исполнения заявки {{.Name}}<br>Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>",
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

func NewRequestFormExecutionInfoTemplate(id, name, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("Заявка №%s - Необходимо предоставить информацию", id),
		Text: `Уважаемый коллега, по заявке {{.Id}} необходимо предоставить информацию.<br>
				Для просмотра и заполнения полей заявки перейдите по <a href={{.Link}}>ссылке</a>`,
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
	subject := fmt.Sprintf("Заявка %s %s", id, action)
	textPart := `Уважаемый коллега, заявка {{.Id}} <b>{{.Action}}</b><br>`

	if action == "ознакомлено" {
		subject = fmt.Sprintf("Ознакомление по заявке %s", id)
		textPart = `Уважаемый коллега, заявка {{.Id}} получена виза <b>Ознакомлен</b><br>`
	}

	if action == "проинформировано" {
		subject = fmt.Sprintf("Информирование по заявке %s", id)
		textPart = `Уважаемый коллега, заявка {{.Id}} получена виза <b>Проинформирован</b><br>`
	}

	return Template{
		Subject: subject,
		Text: textPart + `Для просмотра перейдите по <a href={{.Link}}>ссылке</a><br>
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

func NewApplicationPersonStatusNotification(id, name, status, action, deadline, description, sdUrl string) Template {
	actionName := getApprovementActionNameByStatus(status, action)
	return Template{
		Subject: fmt.Sprintf("Заявка %s ожидает %s", id, actionName),
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
			Action:      actionName,
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

func NewAddApproversTemplate(id, name, sdUrl, status string) Template {
	actionName := getApprovementActionNameByStatus(status, defaultApprovementActionName)
	return Template{
		Subject: fmt.Sprintf("Заявка %s ожидает %s", id, actionName),
		Text: `Уважаемый коллега, заявка {{.Id}} <b>ожидает {{.ActionName}}.</b><br>
				Для просмотра перейти по <a href={{.Link}}>ссылке</a>`,
		Variables: struct {
			Id         string `json:"id"`
			Name       string `json:"name"`
			Link       string `json:"link"`
			ActionName string `json:"actionName"`
		}{
			Id:         id,
			Name:       name,
			Link:       fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			ActionName: actionName,
		},
	}
}

func NewDecisionMadeByAdditionalApproverTemplate(id, fullname, decision, comment, sdUrl string) Template {
	if comment != "" {
		comment = ": " + comment
	}
	return Template{
		Subject: fmt.Sprintf("Получена рецензия по Заявке №%s", id),
		Text: `<p>Уважаемый коллега, получена рецензия по заявке №{{.Id}}</p></br>
				<p>{{.Fullname}} {{.Decision}}{{.Comment}}</p></br>
				<p>Для просмотра перейдите по <a href={{.Link}}>ссылке</a></p></br>
				
				<style>
					p {
						font-family: Arial;
						font-size: 11px;
						margin-bottom: -20px;
					}
				</style>`,
		Variables: struct {
			Id       string `json:"id"`
			Fullname string `json:"fullname"`
			Decision string `json:"decision"`
			Comment  string `json:"comment"`
			Link     string `json:"link"`
		}{
			Id:       id,
			Fullname: fullname,
			Decision: decision,
			Comment:  comment,
			Link:     fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

func getApprovementActionNameByStatus(status, defaultActionName string) (res string) {
	switch status {
	case script.SettingStatusApprovement:
		return "согласования"
	case script.SettingStatusApproveConfirm:
		return "утверждения"
	case script.SettingStatusApproveView:
		return "ознакомления"
	case script.SettingStatusApproveInform:
		return "подтверждения об информировании"
	case script.SettingStatusApproveSign:
		return "подписания"
	default:
		return defaultActionName
	}
}
