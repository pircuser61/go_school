package mail

import (
	"fmt"
	"strconv"
	"strings"

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

func NewApprovementSLATpl(id, name, sdUrl, status string) Template {
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

func NewApprovementHalfSLATpl(id, name, sdUrl, status string) Template {
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

func NewExecutionSLATpl(id, name, sdUrl string) Template {
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

func NewExecutiontHalfSLATpl(id, name, sdUrl string) Template {
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

func NewReworkSLATpl(id, sdUrl string, reworkSla int) Template {
	return Template{
		Subject: fmt.Sprintf("Заявка %s автоматически перенесена в архив", id),
		Text: `Уважаемый коллега, истек срок ожидания доработок по заявке {{.Id}}.</br>
Заявка автоматически перенесена в архив по истечении {{.Duration}} дней.</br>
Для просмотра заявки перейдите по <a href={{.Link}}>ссылке</a><br>`,
		Variables: struct {
			Id       string `json:"id"`
			Duration string `json:"duration"`
			Link     string `json:"link"`
		}{
			Id:       id,
			Duration: strconv.Itoa(reworkSla / 8),
			Link:     fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

func NewRequestExecutionInfoTpl(id, name, sdUrl string) Template {
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

func NewRequestFormExecutionInfoTpl(id, name, sdUrl string) Template {
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

func NewRequestApproverInfoTpl(id, name, sdUrl string) Template {
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

func NewAnswerApproverInfoTpl(id, name, sdUrl string) Template {
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

func NewAnswerExecutionInfoTpl(id, name, sdUrl string) Template {
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

func NewAppInitiatorStatusNotificationTpl(id, name, action, description, sdUrl string) Template {
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

type NewAppPersonStatusTpl struct {
	WorkNumber  string
	Name        string
	Status      string
	Action      string
	DeadLine    string
	Description string
	SdUrl       string
	Mailto      string

	BlockID                   string
	ExecutionDecisionExecuted string
	ExecutionDecisionRejected string

	// actions for approver
	ApproverActions []Action

	IsEditable bool
}

const (
	statusExecution = "processing"
)

func NewAppPersonStatusNotificationTpl(in *NewAppPersonStatusTpl) Template {
	actionName := getApprovementActionNameByStatus(in.Status, in.Action)

	buttons := ""
	if in.Status == statusExecution {
		buttons = getExecutionButtons(
			in.WorkNumber,
			in.Mailto,
			in.BlockID,
			in.ExecutionDecisionExecuted,
			in.ExecutionDecisionRejected,
			in.IsEditable,
		)
	}

	if in.Status == script.SettingStatusApprovement ||
		in.Status == script.SettingStatusApproveConfirm ||
		in.Status == script.SettingStatusApproveView ||
		in.Status == script.SettingStatusApproveInform ||
		in.Status == script.SettingStatusApproveSign {
		buttons = getApproverButtons(in.WorkNumber, in.Mailto, in.BlockID, in.ApproverActions, in.IsEditable)
	}

	return Template{
		Subject: fmt.Sprintf("Заявка %s ожидает %s", in.WorkNumber, actionName),
		Text: `Уважаемый коллега, заявка {{.Id}} <b>ожидает {{.Action}}</b><br>
				Для просмотра перейдите по <a href={{.Link}}>ссылке</a><br>
				Срок {{.Action}} до {{.Deadline}}<br>
				{{.Buttons}}
				Текст заявки:<br><br>
				<pre style="white-space: pre-wrap; word-break: keep-all; font-family: inherit;">{{.Description}}</pre>`,
		Variables: struct {
			Id          string `json:"id"`
			Name        string `json:"name"`
			Link        string `json:"link"`
			Action      string `json:"action"`
			Deadline    string `json:"deadline"`
			Description string `json:"description"`
			Buttons     string `json:"buttons"`
		}{
			Id:          in.WorkNumber,
			Name:        in.Name,
			Link:        fmt.Sprintf(TaskUrlTemplate, in.SdUrl, in.WorkNumber),
			Action:      actionName,
			Deadline:    in.DeadLine,
			Description: in.Description,
			Buttons:     buttons,
		},
	}
}

func NewAnswerSendToEditTpl(id, name, sdUrl string) Template {
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

func NewExecutionTakenInWorkTpl(dto *ExecutorNotifTemplate) Template {
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

func NewAddApproversTpl(id, name, sdUrl, status, mailto, blockId string, al []Action, isEditable bool) Template {
	actionName := getApprovementActionNameByStatus(status, defaultApprovementActionName)

	buttons := getApproverButtons(id, mailto, blockId, al, isEditable)

	return Template{
		Subject: fmt.Sprintf("Заявка %s ожидает %s", id, actionName),
		Text: `Уважаемый коллега, заявка {{.Id}} <b>ожидает {{.ActionName}}.</b><br>
				Для просмотра перейти по <a href={{.Link}}>ссылке</a> {{.Buttons}}`,
		Variables: struct {
			Id         string `json:"id"`
			Name       string `json:"name"`
			Link       string `json:"link"`
			ActionName string `json:"actionName"`
			Buttons    string `json:"buttons"`
		}{
			Id:         id,
			Name:       name,
			Link:       fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			ActionName: actionName,
			Buttons:    buttons,
		},
	}
}

func NewDecisionMadeByAdditionalApprover(id, fullname, decision, comment, sdUrl string) Template {
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

func NewDayBeforeRequestAddInfoSLABreached(id, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("По заявке №%s требуется дополнительная информация", id),
		Text: `Уважаемый коллега, по вашей заявке №{{.Id}} 
				необходимо предоставить дополнительную информацию в течение 
				одного рабочего дня с момента данного уведомления, 
				иначе заявка будет автоматически <b>закрыта</b>.</br> 
				Заявка доступна по <a href={{.Link}}>ссылке</a></p></br>`,
		Variables: struct {
			Id   string `json:"id"`
			Link string `json:"link"`
		}{
			Id:   id,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

func NewRequestAddInfoSLABreached(id, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("Заявка №%s автоматически отклонена", id),
		Text: `Уважаемый коллега, заявка №{{.Id}} 
				автоматически отклонена из-за отсутствия ответа 
				на запрос дополнительной информации в течении 3 дней 
				Заявка доступна по <a href={{.Link}}>ссылке</a></p></br>`,
		Variables: struct {
			Id   string `json:"id"`
			Link string `json:"link"`
		}{
			Id:   id,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
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

type Action struct {
	Id       string
	Title    string
	Decision string
}

const (
	subjectTpl = "step_name=%s|decision=%s|work_number=%s|action_name=%s"
	buttonTpl  = `<p><a href="mailto:%s?subject=%s&body=***Комментарий***" target="_blank">%s</a></p>`

	actionApproverSendEditApp   = "approver_send_edit_app"
	actionExecutorSendEditApp   = "executor_send_edit_app"
	taskUpdateActionExecution   = "execution"
	taskUpdateActionApprovement = "approvement"
)

func getApproverButtons(workNumber, mailto, blockId string, actions []Action, isEditable bool) string {
	buttons := make([]string, 0, len(actions))
	for i := range actions {
		if actions[i].Id == actionApproverSendEditApp {
			continue
		}
		subject := fmt.Sprintf(subjectTpl, blockId, actions[i].Decision, workNumber, taskUpdateActionApprovement)
		buttons = append(buttons, fmt.Sprintf(buttonTpl, mailto, subject, actions[i].Title))
	}

	if isEditable {
		sendEditAppSubject := fmt.Sprintf(subjectTpl, blockId, "", workNumber, actionApproverSendEditApp)
		sendEditAppBtn := fmt.Sprintf(buttonTpl, mailto, sendEditAppSubject, "Отправить на доработку")
		buttons = append(buttons, sendEditAppBtn)
	}

	return fmt.Sprintf("<p><b>Действия с заявкой</b></p> %s", strings.Join(buttons, ""))
}

func getExecutionButtons(workNumber, mailto, blockId, executed, rejected string, isEditable bool) string {
	executedSubject := fmt.Sprintf(subjectTpl, blockId, executed, workNumber, taskUpdateActionExecution)
	executedBtn := fmt.Sprintf(buttonTpl, mailto, executedSubject, "Исполнено")

	rejectedSubject := fmt.Sprintf(subjectTpl, blockId, rejected, workNumber, taskUpdateActionExecution)
	rejectedBtn := fmt.Sprintf(buttonTpl, mailto, rejectedSubject, "Не исполнено")

	buttons := []string{
		executedBtn,
		rejectedBtn,
	}

	if isEditable {
		sendEditAppSubject := fmt.Sprintf(subjectTpl, blockId, "", workNumber, actionExecutorSendEditApp)
		sendEditAppBtn := fmt.Sprintf(buttonTpl, mailto, sendEditAppSubject, "Отправить на доработку")
		buttons = append(buttons, sendEditAppBtn)
	}

	return fmt.Sprintf("<p><b>Действия с заявкой</b></p> %s", strings.Join(buttons, ""))
}
