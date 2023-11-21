package mail

import (
	"fmt"
	"math"
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
	Template  string
	Image     string
	Variables interface{}
}

type Notification struct {
	Template Template
	To       []string
}

type ExecutorNotifTemplate struct {
	WorkNumber   string
	Name         string
	SdUrl        string
	ExecutorName string
	Initiator    string
	Description  string
	BlockID      string
	Mailto       string
	Login        string
	IsGroup      bool
	LastWorks    []*entity.EriusTask
}

type LastWork struct {
	DaysAgo int    `json:"days_ago"`
	WorkURL string `json:"work_url"`
}

type LastWorks []*LastWork

//nolint:dupl // not duplicate
func NewApprovementSLATpl(id, name, sdUrl, status, deadline string) Template {
	actionName := getApprovementActionNameByStatus(status, defaultApprovementActionName)

	return Template{
		Subject:  fmt.Sprintf("По заявке %s %s истекло время %s", id, name, actionName),
		Template: "internal/mail/template/13-14approvalHasExpired-template.html",
		Image:    "isteklo_soglasovanie.png",
		//Template: "Истекло время {{.ActionName}} заявки {{.Name}}<br>Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>",
		Variables: struct {
			Name       string `json:"name"`
			Link       string `json:"link"`
			Action     string `json:"action"`
			ActionName string `json:"actionName"`
			Deadline   string `json:"deadline"`
		}{
			Name:       name,
			Link:       fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			Action:     status,
			ActionName: actionName,
			Deadline:   deadline,
		},
	}
}

// 19-20
func NewExecutionSLATpl(id, name, sdUrl, status string) Template {
	return Template{
		Subject:  fmt.Sprintf("По заявке %s %s истекло время исполнения", id, name),
		Template: "internal/mail/template/19-20executionExpired-template.html",
		Image:    "isteklo_ispolnenie.png",
		//		Template: `Истекло время исполнения заявки {{.Name}}<br>
		//Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>`,
		Variables: struct {
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			Name: name,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

//nolint:dupl // not duplicate
func NewApprovementHalfSLATpl(id, name, sdUrl, status, deadline string) Template {
	actionName := getApprovementActionNameByStatus(status, defaultApprovementActionName)

	//lastWorksTemplate := getLastWorksForTemplate(lastWorks, sdUrl)

	return Template{
		Subject:  fmt.Sprintf("По заявке %s %s истекает время %s", id, name, actionName),
		Template: "internal/mail/template/13-14approvalHasExpired-template.html",
		Image:    "isteklo_soglasovanie.png",
		//Template: "Истекает время {{.ActionName}} заявки {{.Name}}<br>" +
		//	"{{range .LastWorks}}Внимание! Предыдущая заявка была подана {{.DaysAgo}} дней назад. {{.WorkURL}}<br>{{end}}" +
		//	"Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>",
		Variables: struct {
			Name       string `json:"name"`
			Link       string `json:"link"`
			ActionName string `json:"actionName"`
			Action     string `json:"action"`
			Deadline   string `json:"deadline"`
		}{
			Name:       name,
			Link:       fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			ActionName: actionName,
			Action:     status,
			Deadline:   deadline,
		},
	}
}

func NewFormSLATpl(id, name, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("По заявке №%s %s истекло время предоставления дополнительной информации", id, name),
		Template: `Истекло время предоставление дополнительной информации по заявке {{.Name}}<br>
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

// 19-20
func NewExecutiontHalfSLATpl(id, name, sdUrl, status string) Template {
	//lastWorksTemplate := getLastWorksForTemplate(lastWorks, sdUrl)

	return Template{
		Subject:  fmt.Sprintf("По заявке %s %s истекает время исполнения", id, name),
		Template: "internal/mail/template/19-20executionExpired-template.html",
		Image:    "istekaet_ispolnenie.png",
		//Template: "Истекает время исполнения заявки {{.Name}}<br>" +
		//	"{{range .LastWorks}}Внимание! Предыдущая заявка была подана {{.DaysAgo}} дней назад. {{.WorkURL}}<br>{{end}}" +
		//	"Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>",
		Variables: struct {
			Id     string `json:"id"`
			Name   string `json:"name"`
			Link   string `json:"link"`
			Action string `json:"action"`
			//			LastWorks LastWorks `json:"last_works"`
		}{
			Id:     id,
			Name:   name,
			Link:   fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			Action: status,
			//			LastWorks: lastWorksTemplate,
		},
	}
}

func NewFormDayHalfSLATpl(id, name, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("По заявке №%s %s истекает время предоставления информации", id, name),
		Template: "Уважаемый коллега, время предоставления информации по {{.Name}} заявке № {{.Id}} истекает <br>" +
			"Для просмотра перейдите по <a href={{.Link}}>заявке</a>",
		Variables: struct {
			Name string `json:"name"`
			Id   string `json:"id"`
			Link string `json:"link"`
		}{
			Name: name,
			Id:   id,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

// 23
func NewReworkSLATpl(id, name, sdUrl string, reworkSla int) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка %s %s автоматически перенесена в архив", id, name),
		Template: "internal/mail/template/23rejectedDueToLackOfResponse-template.html",
		Image:    "avtootklonena.png",
		//		Template: `Уважаемый коллега, истек срок ожидания доработок по заявке {{.Id}} {{.Name}}.</br>
		//Заявка автоматически перенесена в архив по истечении {{.Duration}} дней.</br>
		//Для просмотра заявки перейдите по <a href={{.Link}}>ссылке</a><br>`,
		Variables: struct {
			Id       string `json:"id"`
			Name     string `json:"name"`
			Duration string `json:"duration"`
			Link     string `json:"link"`
		}{
			Id:       id,
			Name:     name,
			Duration: strconv.Itoa(reworkSla / 8),
			Link:     fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

func NewRequestExecutionInfoTpl(id, name, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка %s %s запрос дополнительной информации", id, name),
		Template: "internal/mail/template/15moreInfoRequired-template.html",
		Image:    "dop_info.png",
		//Template: `Уважаемый коллега, по заявке {{.Id}} {{.Name}} требуется дополнительная информация<br>
		//		Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>`,
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

func NewRequestFormExecutionInfoTpl(id, name, sdUrl string, isReentry bool) Template {
	var retryStr string
	if isReentry {
		retryStr = " повторно"
	}
	action := "исполнения"
	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s - Необходимо%s предоставить информацию", id, name, retryStr),
		Template: "internal/mail/template/22deadlineForAdditionalInfo-template.html",
		Image:    "dop_info.png",
		//Template: fmt.Sprintf(`Уважаемый коллега, по заявке № {{.Id}} {{.Name}} необходимо%s предоставить информацию.<br>
		//		Для просмотра и заполнения полей заявки перейдите по <a href={{.Link}}>ссылке</a>`, retryStr),
		Variables: struct {
			Id     string
			Name   string
			Link   string
			Action string
		}{
			Id:     id,
			Name:   name,
			Action: action,
			Link:   fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

type NewFormExecutionNeedTakeInWorkDto struct {
	WorkNumber string
	WorkTitle  string
	SdUrl      string
	Mailto     string
	BlockName  string
	Login      string
	Deadline   string
}

func NewFormExecutionNeedTakeInWorkTpl(dto *NewFormExecutionNeedTakeInWorkDto, isReentry bool) Template {
	actionSubject := fmt.Sprintf(subjectTpl, dto.BlockName, "", dto.WorkNumber, formExecutorStartWorkAction, dto.Login)
	actionBtn := getButton(dto.Mailto, actionSubject, "Взять в работу")

	var retryStr string
	if isReentry {
		retryStr = " повторно"
	}

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s - Необходимо%s предоставить информацию", dto.WorkNumber, dto.WorkTitle, retryStr),
		Template: "internal/mail/template/22deadlineForAdditionalInfo-template.html",
		Image:    "dop_info.png",
		//Template: fmt.Sprintf(`Уважаемый коллега, по заявке № {{.Id}} {{.Name}} необходимо%s предоставить информацию.<br>
		//			Для просмотра полей заявки перейдите по <a href={{.Link}}>ссылке</a><br>
		//			Срок предоставления информации заявки: {{.Deadline}}
		//			</br><b>Действия с заявкой</b></br>{{.ActionBtn}}</br>`, retryStr),
		Variables: struct {
			Id        string
			Name      string
			Link      string
			Deadline  string
			ActionBtn string
		}{
			Id:        dto.WorkNumber,
			Name:      dto.WorkTitle,
			Link:      fmt.Sprintf(TaskUrlTemplate, dto.SdUrl, dto.WorkNumber),
			Deadline:  dto.Deadline,
			ActionBtn: actionBtn,
		},
	}
}

func NewRequestApproverInfoTpl(id, name, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка %s %s запрос дополнительной информации", id, name),
		Template: "internal/mail/template/15moreInfoRequired-template.html",
		Image:    "dop_info.png",
		//Template: `Уважаемый коллега, по заявке № {{.Id}} {{.Name}} требуется дополнительная информация<br>
		//		Для просмотра перейдите по <a href={{.Link}}>ссылке</a>`,
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

// 18
func NewAnswerApproverInfoTpl(id, name, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка %s %s запрос дополнительной информации", id, name),
		Template: "internal/mail/template/16additionalInfoReceived-template.html",
		Image:    "dop_info_poluchena.png",
		//Template: `Уважаемый коллега, по заявке № {{.Id}} {{.Name}} была получена дополнительная информация<br>
		//		Для просмотра перейдите по <a href={{.Link}}>ссылке</a>`,
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
		Subject:  fmt.Sprintf("Заявка %s %s получена дополнительная информация", id, name),
		Template: "internal/mail/template/16additionalInfoReceived-template.html",
		Image:    "dop_info_poluchena.png",
		//Template: `Уважаемый коллега, по заявке {{.Id}} {{.Name}} была получена дополнительная информация<br>
		//		Для ознакомления Вы можете перейти в <a href={{.Link}}>заявку</a>`,
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
	subject := fmt.Sprintf("Заявка %s %s %s", id, name, action)
	textPart := `Уважаемый коллега, заявка {{.Id}} {{.Name}} <b>{{.Action}}</b><br>`

	if action == "ознакомлено" {
		subject = fmt.Sprintf("Ознакомление по заявке %s %s", id, name)
		textPart = `Уважаемый коллега, заявка {{.Id}} {{.Name}} получена виза <b>Ознакомлен</b><br>`
	}

	if action == "проинформировано" {
		subject = fmt.Sprintf("Информирование по заявке %s %s", id, name)
		textPart = `Уважаемый коллега, заявка {{.Id}} {{.Name}} получена виза <b>Проинформирован</b><br>`
	}

	textPart += `Для просмотра перейдите по 	 <a href={{.Link}}>ссылке</a><br>`

	if description != "" {
		textPart += `Текст заявки:<br><br>
			<pre style="white-space: pre-wrap; word-break: keep-all; font-family: inherit;">{{.Description}}</pre>`
	}

	return Template{
		Subject:  subject,
		Template: textPart,
		Variables: struct {
			Id          string `json:"id"`
			Name        string `json:"name"`
			Link        string `json:"link"`
			Description string `json:"description"`
			Action      string `json:"action"`
		}{
			Id:          id,
			Name:        name,
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

func NewSignerNotificationTpl(id, name, description, sdUrl, slaDate string, autoReject bool) Template {
	autoRejectText := ""

	if autoReject {
		autoRejectText = "После истечения срока заявка будет автоматически отклонена.<br>"
	}

	return Template{
		Subject: fmt.Sprintf("Заявка №%s %s ожидает подписания", id, name),
		Template: `Уважаемый коллега, заявка {{.Id}} {{.Name}} <b>ожидает подписания</b>.<br>
				Для просмотра перейдите по <a href={{.Link}}>ссылке</a><br>
				Срок подписания до {{.SLADate}} <br>
				{{.AutoRejectText}}
				Текст заявки:<br>
<pre style="white-space: pre-wrap; word-break: keep-all; font-family: inherit;">{{.Description}}</pre>`,

		Variables: struct {
			Id             string
			Name           string
			Link           string
			Description    string
			SLADate        string
			AutoRejectText string
		}{
			Id:             id,
			Name:           name,
			Link:           fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			Description:    description,
			SLADate:        slaDate,
			AutoRejectText: autoRejectText,
		},
	}
}

const (
	statusExecution = "processing"
)

func NewAppPersonStatusNotificationTpl(in *NewAppPersonStatusTpl) Template {
	actionName := getApprovementActionNameByStatus(in.Status, in.Action)
	buttons := ""

	switch in.Status {
	case statusExecution:
		buttons = getExecutionButtons(
			in.WorkNumber,
			in.Mailto,
			in.BlockID,
			in.ExecutionDecisionExecuted,
			in.ExecutionDecisionRejected,
			in.Login,
			in.IsEditable,
		)
	case script.SettingStatusApprovement, script.SettingStatusApproveConfirm, script.SettingStatusApproveView,
		script.SettingStatusApproveInform, script.SettingStatusApproveSign, script.SettingStatusApproveSignUkep:
		buttons = getApproverButtons(in.WorkNumber, in.Mailto, in.BlockID, in.Login, in.ApproverActions, in.IsEditable)
	}

	lastWorksTemplate := getLastWorksForTemplate(in.LastWorks, in.SdUrl)

	textPart := `Уважаемый коллега, заявка {{.Id}} {{.Name}} <b>ожидает {{.Action}}</b><br/>
				{{range .LastWorks}}Внимание! Предыдущая заявка была подана {{.DaysAgo}} дней назад. {{.WorkURL}}<br>{{end}}
				Для просмотра перейдите по <a href={{.Link}}>ссылке</a><br/>
				Срок {{.Action}} до {{.Deadline}}<br/>
				{{.Buttons}}`

	if in.Description != "" {
		textPart = fmt.Sprintf("%s\n%s", textPart, `Текст заявки:<br/>
<pre style="white-space: pre-wrap; word-break: keep-all; font-family: inherit;">{{.Description}}</pre>`)
	}

	return Template{
		Subject:  fmt.Sprintf("Заявка %s %s ожидает %s", in.WorkNumber, in.Name, actionName),
		Template: textPart,
		Variables: struct {
			Id          string
			Name        string
			Link        string
			Action      string
			Description string
			Deadline    string
			Buttons     string
			LastWorks   LastWorks
		}{
			Id:          in.WorkNumber,
			Name:        in.Name,
			Link:        fmt.Sprintf(TaskUrlTemplate, in.SdUrl, in.WorkNumber),
			Action:      actionName,
			Description: in.Description,
			Deadline:    in.DeadLine,
			Buttons:     buttons,
			LastWorks:   lastWorksTemplate,
		},
	}
}

func NewSendToInitiatorEditTpl(id, name, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка %s %s требует доработки", id, name),
		Template: "internal/mail/template/17needsImprovement-template.html",
		Image:    "nuzhna_dorabotka.png",
		//Template: `Уважаемый коллега, заявка {{.Id}} {{.Name}} <b>требует доработки.</b><br>
		//		Для просмотра перейти по <a href={{.Link}}>ссылке</a>`,
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
	actionBtn := getButton(dto.Mailto, actionSubject, "Взять в работу")

	text := ""
	subject := ""

	if dto.IsGroup {
		text = `{{range .LastWorks}}Внимание! Предыдущая заявка была подана {{.DaysAgo}} дней назад. {{.WorkURL}}<br>
				{{end}}Уважаемый коллега, заявка {{.Id}} {{.Name}} <b>назначена на Группу исполнителей</b><br>
				Для просмотра перейти по <a href={{.Link}}>ссылке</a></br>
				<b>Действия с заявкой</b></br>{{.ActionBtn}}</br>`
		subject = fmt.Sprintf("Заявка №%s %s назначена на Группу исполнителей", dto.WorkNumber, dto.Name)
	} else {
		text = `{{range .LastWorks}}Внимание! Предыдущая заявка была подана {{.DaysAgo}} дней назад. {{.WorkURL}}<br>
				{{end}}Уважаемый коллега, заявка {{.Id}} {{.Name}} <b>ожидает исполнения</b><br>
				Для просмотра перейти по <a href={{.Link}}>ссылке</a></br>
				<b>Действия с заявкой</b></br>{{.ActionBtn}}</br>`
		subject = fmt.Sprintf("Заявка №%s %s назначена на исполнение", dto.WorkNumber, dto.Name)
	}

	if dto.Description != "" {
		text += ` ------------ Описание ------------  </br>
<pre style="white-space: pre-wrap; word-break: keep-all; font-family: inherit;">{{.Description}}</pre>`
	}

	lastWorksTemplate := getLastWorksForTemplate(dto.LastWorks, dto.SdUrl)

	return Template{
		Subject:  subject,
		Template: "internal/mail/template/12applicationIsAwaitingExecution-template.html",
		Image:    "ozhidaet_ispolneniya.png",
		//		Template: text + `<style>
		//    p {
		//        font-family: Arial;
		//        font-size: 11px;
		//        margin-bottom: -20px;
		//    }
		//</style>`,
		Variables: struct {
			Id          string
			Name        string
			Link        string
			Description string
			ActionBtn   string
			LastWorks   LastWorks
		}{
			Id:          dto.WorkNumber,
			Name:        dto.Name,
			Link:        fmt.Sprintf(TaskUrlTemplate, dto.SdUrl, dto.WorkNumber),
			Description: dto.Description,
			ActionBtn:   actionBtn,
			LastWorks:   lastWorksTemplate,
		},
	}
}

func NewExecutionTakenInWorkTpl(dto *ExecutorNotifTemplate) Template {
	textPart := `{{range .LastWorks}}Внимание! Предыдущая заявка была подана {{.DaysAgo}} дней назад. {{.WorkURL}}<br>
{{end}}Уважаемый коллега, заявка {{.Id}} {{.Name}} <b>взята в работу</b> пользователем <b>{{.Executor}}</b><br>
 <b>Инициатор: </b>{{.Initiator}}</br>
 <b>Ссылка на заявку: </b><a href={{.Link}}>{{.Link}}</a></br>`

	if dto.Description != "" {
		textPart += `------------ Описание ------------  </br>
<pre style="white-space: pre-wrap; word-break: keep-all; font-family: inherit;">{{.Description}}</pre>`
	}

	lastWorksTemplate := getLastWorksForTemplate(dto.LastWorks, dto.SdUrl)

	return Template{
		Subject:  fmt.Sprintf("Заявка №%s %s взята в работу пользователем %s", dto.WorkNumber, dto.Name, dto.ExecutorName),
		Template: "internal/mail/template/05applicationAccepted-template.html",
		Image:    "zayavka_vzyata_v_rabotu.png",
		//	Template: textPart + `<style>
		//p {
		//    font-family: Arial;
		//    font-size: 11px;
		//    margin-bottom: -20px;
		//}
		//</style>`,
		Variables: struct {
			Id          string
			Name        string
			Executor    string
			Link        string
			Initiator   string
			LastWorks   LastWorks
			Description string
		}{
			Id:          dto.WorkNumber,
			Name:        dto.Name,
			Executor:    dto.ExecutorName,
			Link:        fmt.Sprintf(TaskUrlTemplate, dto.SdUrl, dto.WorkNumber),
			Initiator:   dto.Initiator,
			LastWorks:   lastWorksTemplate,
			Description: dto.Description,
		},
	}
}

// 11
func NewAddApproversTpl(id, name, sdUrl, status, deadline string, lastWorks []*entity.EriusTask) Template {
	actionName := getApprovementActionNameByStatus(status, defaultApprovementActionName)

	lastWorksTemplate := getLastWorksForTemplate(lastWorks, sdUrl)

	return Template{
		Subject:  fmt.Sprintf("Заявка %s %s ожидает %s", id, name, actionName),
		Template: "internal/mail/template/11receivedForApproval-template.html",
		Image:    "ozhidaet_ispolneniya.png",
		//Template: `Уважаемый коллега, заявка {{.Id}} {{.Name}} <b>ожидает {{.ActionName}}.</b><br>
		//		{{range .LastWorks}}Внимание! Предыдущая заявка была подана {{.DaysAgo}} дней назад. {{.WorkURL}}<br>{{end}}
		//		Для просмотра перейти по <a href={{.Link}}>ссылке</a>`,
		Variables: struct {
			Id         string    `json:"id"`
			Name       string    `json:"name"`
			Link       string    `json:"link"`
			ActionName string    `json:"actionName"`
			LastWorks  LastWorks `json:"last_works"`
		}{
			Id:         id,
			Name:       name,
			Link:       fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			ActionName: actionName,
			LastWorks:  lastWorksTemplate,
		},
	}
}

func NewDecisionMadeByAdditionalApprover(id, name, fullname, decision, comment, sdUrl string) Template {
	if comment != "" {
		comment = ": " + comment
	}
	return Template{
		Subject:  fmt.Sprintf("Получена рецензия по Заявке №%s %s", id, name),
		Template: "internal/mail/template/18reviewReceived-template.html",
		Image:    "poluchena_retsenzia.png",
		//Template: `Уважаемый коллега, получена рецензия по заявке №{{.Id}} {{.Name}}<br>
		//		{{.Fullname}} {{.Decision}}{{.Comment}}<br>
		//		Для просмотра перейдите по <a href={{.Link}}>ссылке</a><br>
		//
		//		<style>
		//			p {
		//				font-family: Arial;
		//				font-size: 11px;
		//				margin-bottom: -20px;
		//			}
		//		</style>`,
		Variables: struct {
			Id       string `json:"id"`
			Name     string `json:"name"`
			Fullname string `json:"fullname"`
			Decision string `json:"decision"`
			Comment  string `json:"comment"`
			Link     string `json:"link"`
		}{
			Id:       id,
			Name:     name,
			Fullname: fullname,
			Decision: decision,
			Comment:  comment,
			Link:     fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

func NewDayBeforeRequestAddInfoSLABreached(id, name, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("По заявке №%s %s требуется дополнительная информация", id, name),
		Template: "internal/mail/template/15moreInfoRequired-template.html",
		Image:    "dop_info.png",
		//Template: `Уважаемый коллега, по вашей заявке №{{.Id}} {{.Name}}
		//		необходимо предоставить дополнительную информацию в течение
		//		одного рабочего дня с момента данного уведомления,
		//		иначе заявка будет автоматически <b>перенесена в архив</b>.</br>
		//		Заявка доступна по <a href={{.Link}}>ссылке</a></br>`,
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

// NewRequestAddInfoSLABreached 21
func NewRequestAddInfoSLABreached(id, name, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка №%s %s автоматически перенесена в архив", id, name),
		Template: "internal/mail/template/21rejectedDueToPendingRevision-template.html",
		//Template: `Уважаемый коллега, заявка №{{.Id}} {{.Name}}
		//		автоматически перенесена в архив из-за отсутствия ответа
		//		на запрос дополнительной информации в течение 3 дней <br>
		//		Заявка доступна по <a href={{.Link}}>ссылке</a></br>`,
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

func NewInvalidFunctionResp(id, name, sdUrl string) Template {
	return Template{
		Subject: fmt.Sprintf("По заявке №%s %s не удалось получить обратную связь от внешней системы", id, name),
		Template: `Уважаемый коллега, по заявке №{{.Id}} {{.Name}} 
				не удалось получить обратную связь от внешней системы. 
				Попробуйте создать заявку повторно. 
				Если ошибка возникает снова, необходимо обратиться в техническую поддержку <br>
				Заявка доступна по <a href={{.Link}}>ссылке</a></br>`,
		Variables: struct {
			Id   string
			Name string
			Link string
		}{
			Id:   id,
			Name: name,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
		},
	}
}

func NewFormExecutionTakenInWorkTpl(workNumber, workTitle, namePerson, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s взята в работу", workNumber, workTitle),
		Template: "internal/mail/template/05applicationAccepted-template.html",
		Image:    "zayavka_vzyata_v_rabotu.png",
		//Template: "Уважаемый коллега, заявка № {{.Id}} {{.Name}} взята в работу {{.NamePerson}}<br>Для просмотра перейдите по {{.Link}}",
		Variables: struct {
			Id         string `json:"id"`
			Name       string `json:"name"`
			NamePerson string `json:"name_person"`
			Link       string `json:"link"`
		}{
			Id:         workNumber,
			Name:       workTitle,
			NamePerson: namePerson,
			Link:       fmt.Sprintf(TaskUrlTemplate, sdUrl, workNumber),
		},
	}
}

func NewFormPersonExecutionNotificationTemplate(workNumber, workTitle, sdUrl, deadline string) Template {
	return Template{
		Subject: fmt.Sprintf("Заявка № %s %s - Необходимо предоставить информацию", workNumber, workTitle),
		Template: `Уважаемый коллега, по заявке № {{.Id}} {{.Name}} 
					вам необходимо предоставить информацию.<br>
					Для просмотра и заполнения полей заявки перейдите по <a href={{.Link}}>ссылке</a><br>
					Срок предоставления информации заявки: {{.Deadline}}`,
		Variables: struct {
			Id       string
			Name     string
			Link     string
			Deadline string
		}{
			Id:       workNumber,
			Name:     workTitle,
			Link:     fmt.Sprintf(TaskUrlTemplate, sdUrl, workNumber),
			Deadline: deadline,
		},
	}
}

// ВОт тут работаем
func NewRejectPipelineGroupTemplate(workNumber, workTitle, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s - отозвана", workNumber, workTitle),
		Template: "internal/mail/template/24applicationWithdrawn-template.html",
		Image:    "otozvana.png",
		Variables: struct {
			Id   string `json:"id"`
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			Id:   workNumber,
			Name: workTitle,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, workNumber),
		},
	}
}

func NewSignSLAExpiredTemplate(workNumber, workTitle, sdUrl string) Template {
	action := "подписания"
	return Template{
		Subject:  fmt.Sprintf("По заявке № %s %s- истекло время подписания", workNumber, workTitle),
		Template: "Истекло время подписания заявки {{.Name}}<br>Для просмотра перейдите по <a href={{.Link}}>ссылке</a>",
		Variables: struct {
			Id     string `json:"id"`
			Name   string `json:"name"`
			Link   string `json:"link"`
			Action string `json:"action"`
		}{
			Id:     workNumber,
			Name:   workTitle,
			Action: action,
			Link:   fmt.Sprintf(TaskUrlTemplate, sdUrl, workNumber),
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
	case script.SettingStatusApproveSignUkep:
		return "подписания УКЭП"
	default:
		return defaultActionName
	}
}
func NewSignErrorTemplate(workNumber, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("По заявке № %s - возникла ошибка подписания", workNumber),
		Template: "Уважаемый коллега, по заявке № <a href={{.Link}}>{{.Id}}</a> произошла ошибка подписания. Документ не был подписан. Необходимо повторно подписать заявку.",
		Variables: struct {
			Id   string `json:"id"`
			Link string `json:"link"`
		}{
			Id:   workNumber,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, workNumber),
		},
	}
}

type Action struct {
	Title              string
	InternalActionName string
}

func getButton(to, subject, title string) string {
	subject = strings.ReplaceAll(subject, " ", "")
	title = strings.ReplaceAll(title, " ", "_")
	return "<a href='mailto:" + to +
		"?subject=" + subject +
		"&body=***КОММЕНТАРИЙ%20НИЖЕ***%0D%0A%0D%0A***ОБЩИЙ%20РАЗМЕР%20ВЛОЖЕНИЙ%20НЕ%20БОЛЕЕ%2040МБ***' target='_blank'>" + title +
		"</a><br>"
}

const (
	subjectTpl = "step_name=%s|decision=%s|work_number=%s|action_name=%s|login=%s"

	actionApproverSendEditApp   = "approver_send_edit_app"
	actionExecutorSendEditApp   = "executor_send_edit_app"
	taskUpdateActionExecution   = "execution"
	taskUpdateActionApprovement = "approvement"
	executionStartWorkAction    = "executor_start_work"
	formExecutorStartWorkAction = "form_executor_start_work"
	actionApproverSignUkep      = "sign_ukep"
)

func getApproverButtons(workNumber, mailto, blockId, login string, actions []Action, isEditable bool) string {
	buttons := make([]string, 0, len(actions))
	for i := range actions {
		if actions[i].InternalActionName == actionApproverSignUkep {
			return ""
		}
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
			DaysAgo: int(math.Round(utils.GetDateUnitNumBetweenDates(task.StartedAt, time.Now(), utils.Day))),
			WorkURL: fmt.Sprintf(TaskUrlTemplate, sdUrl, task.WorkNumber),
		})
	}
	return lastWorksTemplate
}
