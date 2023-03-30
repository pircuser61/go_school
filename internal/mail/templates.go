package mail

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
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
	WorkNumber   string
	SdUrl        string
	ExecutorName string
	Initiator    string
	Description  string
	BlockID      string
	Mailto       string
	Login        string
	LastWorks    []*entity.EriusTask
}

type LastWork struct {
	DaysAgo int    `json:"days_ago"`
	WorkURL string `json:"work_url"`
}

type LastWorks []*LastWork

//nolint:dupl // not duplicate
func NewApprovementSLATpl(id, name, sdUrl, status string, lastWorks []*entity.EriusTask) Template {
	actionName := getApprovementActionNameByStatus(status, defaultApprovementActionName)

	lastWorksTemplate := getLastWorksForTemplate(lastWorks, sdUrl)

	return Template{
		Subject: fmt.Sprintf("По заявке %s %s истекло время %s", id, name, actionName),
		Text: "{{range .LastWorks}}Внимание! Предыдущая заявка была подана {{.DaysAgo}} дней назад. {{.WorkURL}}\n{{end}}" +
			"Истекло время {{.ActionName}} заявки {{.Name}}<br>Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>",
		Variables: struct {
			Name       string    `json:"name"`
			Link       string    `json:"link"`
			ActionName string    `json:"actionName"`
			LastWorks  LastWorks `json:"last_works"`
		}{
			Name:       name,
			Link:       fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			ActionName: actionName,
			LastWorks:  lastWorksTemplate,
		},
	}
}

//nolint:dupl // not duplicate
func NewApprovementHalfSLATpl(id, name, sdUrl, status string, lastWorks []*entity.EriusTask) Template {
	actionName := getApprovementActionNameByStatus(status, defaultApprovementActionName)

	lastWorksTemplate := getLastWorksForTemplate(lastWorks, sdUrl)

	return Template{
		Subject: fmt.Sprintf("По заявке %s %s истекает время %s", id, name, actionName),
		Text: "{{range .LastWorks}}Внимание! Предыдущая заявка была подана {{.DaysAgo}} дней назад. {{.WorkURL}}\n{{end}}" +
			"Истекает время {{.ActionName}} заявки {{.Name}}<br>Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>",
		Variables: struct {
			Name       string    `json:"name"`
			Link       string    `json:"link"`
			ActionName string    `json:"actionName"`
			LastWorks  LastWorks `json:"last_works"`
		}{
			Name:       name,
			Link:       fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			ActionName: actionName,
			LastWorks:  lastWorksTemplate,
		},
	}
}

func NewExecutionSLATpl(id, name, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("По заявке %s %s истекло время исполнения", id, name),
		Text: `Истекло время исполнения заявки {{.Name}}<br>
Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>`,
		Variables: struct {
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			Name: name,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

func NewFormSLATpl(id, name, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("По заявке №%s %s истекло время предоставления дополнительной информации", id, name),
		Text: `Истекло время предоставление дополнительной информации по заявке {{.Name}}<br>
Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>`,
		Variables: struct {
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			Name: name,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}
func NewExecutiontHalfSLATpl(id, name, sdUrl string, lastWorks []*entity.EriusTask) Template {
	lastWorksTemplate := getLastWorksForTemplate(lastWorks, sdUrl)

	return Template{
		Subject: fmt.Sprintf("По заявке %s %s истекает время исполнения", id, name),
		Text: "{{range .LastWorks}}Внимание! Предыдущая заявка была подана {{.DaysAgo}} дней назад. {{.WorkURL}}\n{{end}}" +
			"Истекает время исполнения заявки {{.Name}}<br>Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>",
		Variables: struct {
			Name      string    `json:"name"`
			Link      string    `json:"link"`
			LastWorks LastWorks `json:"last_works"`
		}{
			Name:      name,
			Link:      fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			LastWorks: lastWorksTemplate,
		},
	}
}

func NewFormDayHalfSLATpl(id, name, sdUrl string, lastWorks []*entity.EriusTask) Template {
	lastWorksTemplate := getLastWorksForTemplate(lastWorks, sdUrl)

	return Template{
		Subject: fmt.Sprintf("По заявке №%s %s истекает время предоставления информации", id, name),
		Text: "{{range .LastWorks}}Внимание! Предыдущая заявка была подана {{.DaysAgo}} дней назад. {{.WorkURL}}\n{{end}}" +
			"Уважаемый коллега, время предоставления информации по {{.Name}} заявке № {{.Id}} истекает " +
			"\nДля просмотра перейдите по <a href={{.Link}}>заявке</a>",
		Variables: struct {
			Name      string    `json:"name"`
			Id        string    `json:"id"`
			Link      string    `json:"link"`
			LastWorks LastWorks `json:"last_works"`
		}{
			Name:      name,
			Id:        id,
			Link:      fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			LastWorks: lastWorksTemplate,
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

func NewAppInitiatorStatusNotificationTpl(id, action, description, sdUrl string) Template {
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

	textPart += `Для просмотра перейдите по <a href={{.Link}}>ссылке</a><br>`

	if description != "" {
		textPart += `Текст заявки:<br><br>
			<pre style="white-space: pre-wrap; word-break: keep-all; font-family: inherit;">{{.Description}}</pre>`
	}

	return Template{
		Subject: subject,
		Text:    textPart,
		Variables: struct {
			Id          string `json:"id"`
			Link        string `json:"link"`
			Description string `json:"description"`
			Action      string `json:"action"`
		}{
			Id:          id,
			Link:        fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			Description: description,
			Action:      action,
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
	Login       string

	BlockID                   string
	ExecutionDecisionExecuted string
	ExecutionDecisionRejected string

	// actions for approver
	ApproverActions []Action

	IsEditable bool

	LastWorks []*entity.EriusTask
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
			in.Login,
			in.IsEditable,
		)
	}

	if in.Status == script.SettingStatusApprovement ||
		in.Status == script.SettingStatusApproveConfirm ||
		in.Status == script.SettingStatusApproveView ||
		in.Status == script.SettingStatusApproveInform ||
		in.Status == script.SettingStatusApproveSign {
		buttons = getApproverButtons(in.WorkNumber, in.Mailto, in.BlockID, in.Login, in.ApproverActions, in.IsEditable)
	}

	lastWorksTemplate := getLastWorksForTemplate(in.LastWorks, in.SdUrl)

	textPart := `{range .LastWorks}}Внимание! Предыдущая заявка была подана {{.DaysAgo}} дней назад. {{.WorkURL}}{{end}}
				Уважаемый коллега, заявка {{.Id}} <b>ожидает {{.Action}}</b><br/>
				Для просмотра перейдите по <a href={{.Link}}>ссылке</a><br/>
				Срок {{.Action}} до {{.Deadline}}<br/>
				{{.Buttons}}`

	if in.Description != "" {
		textPart = fmt.Sprintf("%s\n%s", textPart, `Текст заявки:<br/>
<pre style="white-space: pre-wrap; word-break: keep-all; font-family: inherit;">{{.Description}}</pre>`)
	}

	return Template{
		Subject: fmt.Sprintf("Заявка %s ожидает %s", in.WorkNumber, actionName),
		Text:    textPart,
		Variables: struct {
			Id          string
			Link        string
			Action      string
			Description string
			Deadline    string
			Buttons     string
			LastWorks   LastWorks
		}{
			Id:          in.WorkNumber,
			Link:        fmt.Sprintf(TaskUrlTemplate, in.SdUrl, in.WorkNumber),
			Action:      actionName,
			Description: in.Description,
			Deadline:    in.DeadLine,
			Buttons:     buttons,
			LastWorks:   lastWorksTemplate,
		},
	}
}

func NewAnswerSendToEditTpl(id, name, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("Заявка %s требует доработки", id),
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

func NewExecutionNeedTakeInWorkTpl(dto *ExecutorNotifTemplate) Template {
	actionSubject := fmt.Sprintf(subjectTpl, dto.BlockID, "", dto.WorkNumber, executionStartWorkAction, dto.Login)
	actionSubject = strings.ReplaceAll(actionSubject, " ", "")
	actionBtn := getButton(dto.Mailto, actionSubject, "Взять в работу")

	textPart := `{{range .LastWorks}}Внимание! Предыдущая заявка была подана {{.DaysAgo}} дней назад. {{.WorkURL}}
{{end}}Уважаемый коллега, заявка {{.Id}} <b>назначена на Группу исполнителей</b><br>
 Для просмотра перейти по <a href={{.Link}}>ссылке</a></br>
 <b>Действия с заявкой</b><br>
 {{.ActionBtn}}<br>`

	if dto.Description != "" {
		textPart += ` ------------ Описание ------------  </br>
<pre style="white-space: pre-wrap; word-break: keep-all; font-family: inherit;">{{.Description}}</pre>`
	}

	lastWorksTemplate := getLastWorksForTemplate(dto.LastWorks, dto.SdUrl)

	return Template{
		Subject: fmt.Sprintf("Заявка №%s назначена на Группу исполнителей", dto.WorkNumber),
		Text: textPart + `<style>
    p {
        font-family: Arial;
        font-size: 11px;
        margin-bottom: -20px;
    }
</style>`,
		Variables: struct {
			Id          string
			Link        string
			Description string
			ActionBtn   string
			LastWorks   LastWorks
		}{
			Id:          dto.WorkNumber,
			Link:        fmt.Sprintf(TaskUrlTemplate, dto.SdUrl, dto.WorkNumber),
			Description: dto.Description,
			ActionBtn:   actionBtn,
			LastWorks:   lastWorksTemplate,
		},
	}
}

func NewExecutionTakenInWorkTpl(dto *ExecutorNotifTemplate) Template {
	textPart := `{{range .LastWorks}}Внимание! Предыдущая заявка была подана {{.DaysAgo}} дней назад. {{.WorkURL}}
{{end}}Уважаемый коллега, заявка {{.Id}} <b>взята в работу</b> пользователем <b>{{.Executor}}</b><br>
 <b>Инициатор: </b>{{.Initiator}}</br>
 <b>Ссылка на заявку: </b><a href={{.Link}}>{{.Link}}</a></br>`

	if dto.Description != "" {
		textPart += `------------ Описание ------------  </br>
<pre style="white-space: pre-wrap; word-break: keep-all; font-family: inherit;">{{.Description}}</pre>`
	}

	lastWorksTemplate := getLastWorksForTemplate(dto.LastWorks, dto.SdUrl)

	return Template{
		Subject: fmt.Sprintf("Заявка №%s взята в работу пользователем %s", dto.WorkNumber, dto.ExecutorName),
		Text: textPart + `<style>
    p {
        font-family: Arial;
        font-size: 11px;
        margin-bottom: -20px;
    }
</style>`,
		Variables: struct {
			Id        string    `json:"id"`
			Executor  string    `json:"executor"`
			Link      string    `json:"link"`
			Initiator string    `json:"initiator"`
			LastWorks LastWorks `json:"last_works"`
		}{
			Id:        dto.WorkNumber,
			Executor:  dto.ExecutorName,
			Link:      fmt.Sprintf(TaskUrlTemplate, dto.SdUrl, dto.WorkNumber),
			Initiator: dto.Initiator,
			LastWorks: lastWorksTemplate,
		},
	}
}

func NewAddApproversTpl(id, name, sdUrl, status string) Template {
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

func NewDecisionMadeByAdditionalApprover(id, fullname, decision, comment, sdUrl string) Template {
	if comment != "" {
		comment = ": " + comment
	}
	return Template{
		Subject: fmt.Sprintf("Получена рецензия по Заявке №%s", id),
		Text: `Уважаемый коллега, получена рецензия по заявке №{{.Id}}<br>
				{{.Fullname}} {{.Decision}}{{.Comment}}<br>
				Для просмотра перейдите по <a href={{.Link}}>ссылке</a><br>
				
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
				иначе заявка будет автоматически <b>перенесена в архив</b>.</br> 
				Заявка доступна по <a href={{.Link}}>ссылке</a></br>`,
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
		Subject: fmt.Sprintf("Заявка №%s автоматически перенесена в архив", id),
		Text: `Уважаемый коллега, заявка №{{.Id}} 
				автоматически перенесена в архив из-за отсутствия ответа 
				на запрос дополнительной информации в течение 3 дней <br>
				Заявка доступна по <a href={{.Link}}>ссылке</a></br>`,
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
	Title              string
	InternalActionName string
}

func getButton(to, subject, title string) string {
	subject = strings.ReplaceAll(subject, " ", "")
	return "<a href='mailto:" + to +
		"?subject=" + subject +
		"&body=***КОММЕНТАРИЙ НИЖЕ***%0D%0A%0D%0A***ОБЩИЙ РАЗМЕР ВЛОЖЕНИЙ НЕ БОЛЕЕ 40МБ***' target='_blank'>" + title +
		"</a><br>"
}

const (
	subjectTpl = "step_name=%s|decision=%s|work_number=%s|action_name=%s|login=%s"

	actionApproverSendEditApp   = "approver_send_edit_app"
	actionExecutorSendEditApp   = "executor_send_edit_app"
	taskUpdateActionExecution   = "execution"
	taskUpdateActionApprovement = "approvement"
	executionStartWorkAction    = "executor_start_work"
)

func getApproverButtons(workNumber, mailto, blockId, login string, actions []Action, isEditable bool) string {
	buttons := make([]string, 0, len(actions))
	for i := range actions {
		if actions[i].InternalActionName == actionApproverSendEditApp {
			continue
		}
		subject := fmt.Sprintf(
			subjectTpl,
			blockId,
			actions[i].InternalActionName,
			workNumber,
			taskUpdateActionApprovement,
			login,
		)

		buttons = append(buttons, getButton(mailto, subject, actions[i].Title))
	}

	if len(buttons) == 0 {
		approveAppSubject := fmt.Sprintf(subjectTpl, blockId, "approve", workNumber, taskUpdateActionApprovement, login)
		approveAppBtn := getButton(mailto, approveAppSubject, "Согласовать")
		buttons = append(buttons, approveAppBtn)

		rejectAppSubject := fmt.Sprintf(subjectTpl, blockId, "reject", workNumber, taskUpdateActionApprovement, login)
		rejectAppBtn := getButton(mailto, rejectAppSubject, "Отклонить")
		buttons = append(buttons, rejectAppBtn)
	}

	if isEditable {
		sendEditAppSubject := fmt.Sprintf(subjectTpl, blockId, "", workNumber, actionApproverSendEditApp, login)
		sendEditAppBtn := getButton(mailto, sendEditAppSubject, "Вернуть на доработку")
		buttons = append(buttons, sendEditAppBtn)
	}

	return fmt.Sprintf("<b>Действия с заявкой</b><br>%s", strings.Join(buttons, ""))
}

func getExecutionButtons(workNumber, mailto, blockId, executed, rejected, login string, isEditable bool) string {
	executedSubject := fmt.Sprintf(subjectTpl, blockId, executed, workNumber, taskUpdateActionExecution, login)
	executedBtn := getButton(mailto, executedSubject, "Решить")

	rejectedSubject := fmt.Sprintf(subjectTpl, blockId, rejected, workNumber, taskUpdateActionExecution, login)
	rejectedBtn := getButton(mailto, rejectedSubject, "Отклонить")

	buttons := []string{
		executedBtn,
		rejectedBtn,
	}

	if isEditable {
		sendEditAppSubject := fmt.Sprintf(subjectTpl, blockId, "", workNumber, actionExecutorSendEditApp, login)
		sendEditAppBtn := getButton(mailto, sendEditAppSubject, "Вернуть на доработку")
		buttons = append(buttons, sendEditAppBtn)
	}

	return fmt.Sprintf("<b>Действия с заявкой</b><br> %s", strings.Join(buttons, ""))
}

func getLastWorksForTemplate(lastWorks []*entity.EriusTask, sdUrl string) LastWorks {
	lastWorksTemplate := make(LastWorks, 0, len(lastWorks))

	for _, task := range lastWorks {
		lastWorksTemplate = append(lastWorksTemplate, &LastWork{
			DaysAgo: int(utils.GetDateUnitNumBetweenDates(task.StartedAt, time.Now(), utils.Day)),
			WorkURL: fmt.Sprintf(TaskUrlTemplate, sdUrl, task.WorkNumber),
		})
	}
	return lastWorksTemplate
}
