package mail

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/iancoleman/orderedmap"

	"gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	defaultApprovementActionName = "согласования"
	defaultComment               = "К заявке добавлено вложение"
	attachExists                 = "attachExist"
	attachLinks                  = "attachLinks"
	attachList                   = "attachList"
	TaskURLTemplate              = "%s/applications/details/%s"
	taskRateURLTemplate          = "%s/score?work_number=%s&rate="
)

type Descriptions struct {
	AttachLinks  []file_registry.AttachInfo
	AttachExists bool
	AttachFields []string
}

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

type Attachments struct {
	AttachmentsList []email.Attachment
	AttachExists    bool
	AttachFields    []string
	AttachLinks     []file_registry.AttachInfo
}

type SignerNotifTemplate struct {
	WorkNumber           string
	Name                 string
	SdURL                string
	JocastaURL           string
	Deadline             string
	AutoReject           bool
	IsPositive           bool
	IsLastExecutableStep bool
	Description          []orderedmap.OrderedMap
	Action               string
	Comment              string
}

type Notif struct {
	Title       string
	Body        string
	Description []orderedmap.OrderedMap
	Link        string
	Initiator   *sso.UserInfo
}

type ExecutorNotifTemplate struct {
	WorkNumber  string
	Name        string
	SdURL       string
	Executor    *sso.UserInfo
	Initiator   *sso.UserInfo
	Description []orderedmap.OrderedMap
	BlockID     string
	Mailto      string
	Login       string
	IsGroup     bool
	LastWorks   []*entity.EriusTask
	Deadline    string
	Comment     string
}

type ProcessFinishedTemplate struct {
	WorkNumber  string
	Name        string
	Initiator   *sso.UserInfo
	Mailto      string
	Login       string
	SdURL       string
	Description []orderedmap.OrderedMap
}

type ReviewTemplate struct {
	ID          string
	Name        string
	Decision    string
	Comment     string
	SdURL       string
	Author      *sso.UserInfo
	AttachLinks []file_registry.AttachInfo
	AttachExist bool
}

type LastWork struct {
	WorkNumber string `json:"work_number"`
	Name       string `json:"work_title"`
	DaysAgo    int    `json:"days_ago"`
	Link       string `json:"work_url"`
}

type LastWorks []*LastWork

//nolint:dupl // not duplicate
func NewApprovementSLATpl(id, name, sdURL, status string) Template {
	actionName := getApprovementActionNameByStatus(status, defaultApprovementActionName)

	return Template{
		Subject:  fmt.Sprintf("По заявке № %s %s истекло время %s", id, name, actionName),
		Template: "internal/mail/template/13approvalHasExpired-template.html",
		Image:    "13_isteklo_sogl.png",
		Variables: struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Action string `json:"action"`
			Link   string `json:"link"`
		}{
			ID:     id,
			Name:   name,
			Action: actionName,
			Link:   fmt.Sprintf(TaskURLTemplate, sdURL, id),
		},
	}
}

//nolint:dupl // not duplicate
func NewApprovementHalfSLATpl(id, name, sdURL, status, deadline string, lastWorks []*entity.EriusTask) Template {
	actionName := getApprovementActionNameByStatus(status, defaultApprovementActionName)

	lastWorksTemplate := getLastWorksForTemplate(lastWorks, sdURL)

	return Template{
		Subject:  fmt.Sprintf("По заявке № %s %s истекает время %s", id, name, actionName),
		Template: "internal/mail/template/14approvalExpires-template.html",
		Image:    "14_istekaet_sogl.png",
		Variables: struct {
			ID         string    `json:"id"`
			Name       string    `json:"name"`
			Link       string    `json:"link"`
			Deadline   string    `json:"deadline"`
			LastWorks  LastWorks `json:"last_works"`
			ActionName string    `json:"action_name"`
		}{
			ID:         id,
			Name:       name,
			Link:       fmt.Sprintf(TaskURLTemplate, sdURL, id),
			Deadline:   deadline,
			ActionName: actionName,
			LastWorks:  lastWorksTemplate,
		},
	}
}

func NewExecutionSLATpl(id, name, sdURL string) Template {
	return Template{
		Subject:  fmt.Sprintf("По заявке № %s %s истекло время исполнения", id, name),
		Template: "internal/mail/template/19executionExpired-template.html",
		Image:    "19_isteklo_isp.png",
		Variables: struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			ID:   id,
			Name: name,
			Link: fmt.Sprintf(TaskURLTemplate, sdURL, id),
		},
	}
}

func NewFormSLATpl(id, name, sdURL string) Template {
	return Template{
		Subject:  fmt.Sprintf("По заявке № %s %s истекло время предоставления дополнительной информации", id, name),
		Template: "internal/mail/template/32dopInfoIsteklo-template.html",
		Image:    "32_vremja_isteklo.png",
		Variables: struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			ID:   id,
			Name: name,
			Link: fmt.Sprintf(TaskURLTemplate, sdURL, id),
		},
	}
}

func NewExecutiontHalfSLATpl(id, name, sdURL, deadline string, lastWorks []*entity.EriusTask) Template {
	lastWorksTemplate := getLastWorksForTemplate(lastWorks, sdURL)

	return Template{
		Subject:  fmt.Sprintf("По заявке № %s %s истекает время выполнения", id, name),
		Template: "internal/mail/template/20executionExpires-template.html",
		Image:    "20_istekaet_isp.png",
		Variables: struct {
			ID        string    `json:"id"`
			Name      string    `json:"name"`
			Link      string    `json:"link"`
			Deadline  string    `json:"deadline"`
			LastWorks LastWorks `json:"last_works"`
		}{
			ID:        id,
			Name:      name,
			Link:      fmt.Sprintf(TaskURLTemplate, sdURL, id),
			Deadline:  deadline,
			LastWorks: lastWorksTemplate,
		},
	}
}

func NewFormDayHalfSLATpl(id, name, sdURL, deadline string) Template {
	return Template{
		Subject:  fmt.Sprintf("По заявке № %s %s истекает время предоставления информации", id, name),
		Template: "internal/mail/template/33dopInfoIstekaet-template.html",
		Image:    "33_vremja_istekaet.png",
		Variables: struct {
			Name     string `json:"name"`
			ID       string `json:"id"`
			Link     string `json:"link"`
			Deadline string `json:"deadline"`
		}{
			Name:     name,
			ID:       id,
			Link:     fmt.Sprintf(TaskURLTemplate, sdURL, id),
			Deadline: deadline,
		},
	}
}

func NewReworkSLATpl(id, name, sdURL string, reworkSLA int, checkSLA bool) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s автоматически перенесена в архив", id, name),
		Template: "internal/mail/template/34rejectToarchive-template.html",
		Image:    "34_istjok_srok_ojidaniya_dorabotok.png",
		Variables: struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Duration string `json:"duration"`
			Link     string `json:"link"`
			CheckSLA bool   `json:"checkSLA"`
		}{
			ID:       id,
			Name:     name,
			Duration: strconv.Itoa(reworkSLA / 8),
			Link:     fmt.Sprintf(TaskURLTemplate, sdURL, id),
			CheckSLA: checkSLA,
		},
	}
}

func NewRequestExecutionInfoTpl(id, name, sdURL, comment string) Template {
	comm := defaultComment

	if comment != "" {
		comm = comment
	}

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s запрос дополнительной информации", id, name),
		Template: "internal/mail/template/15moreInfoRequired-template.html",
		Image:    "15_dop_info_trebuetsya.png",
		Variables: struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Link    string `json:"link"`
			Comment string `json:"comment"`
		}{
			ID:      id,
			Name:    name,
			Link:    fmt.Sprintf(TaskURLTemplate, sdURL, id),
			Comment: comm,
		},
	}
}

func NewRequestFormExecutionInfoTpl(id, name, sdURL, deadline string, isReentry bool) Template {
	var retryStr string

	//nolint:goconst // нет нужды в константе
	if isReentry {
		retryStr = " повторно"
	}

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s - Необходимо%s предоставить информацию", id, name, retryStr),
		Template: "internal/mail/template/29form-template.html",
		Image:    "29_neobhodimo_predostavit'_info.png",
		Variables: struct {
			ID       string
			Name     string
			Link     string
			Deadline string
			RetryStr string
		}{
			ID:       id,
			Name:     name,
			Link:     fmt.Sprintf(TaskURLTemplate, sdURL, id),
			Deadline: deadline,
			RetryStr: retryStr,
		},
	}
}

type NewFormExecutionNeedTakeInWorkDto struct {
	WorkNumber string
	WorkTitle  string
	SdURL      string
	Mailto     string
	BlockName  string
	Login      string
	Deadline   string
}

func NewFormExecutionNeedTakeInWorkTpl(dto *NewFormExecutionNeedTakeInWorkDto, isReentry bool) Template {
	actionSubject := fmt.Sprintf(subjectTpl, dto.BlockName, "", dto.WorkNumber, formExecutorStartWorkAction, dto.Login)
	actionBtn := getButton(dto.Mailto, actionSubject, "")

	var retryStr string
	if isReentry {
		retryStr = " повторно"
	}

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s - Необходимо%s предоставить информацию", dto.WorkNumber, dto.WorkTitle, retryStr),
		Template: "internal/mail/template/39takeInWork-template.html",
		Image:    "39_neobhodimo_info.png",
		Variables: struct {
			ID        string
			Name      string
			Link      string
			Deadline  string
			ActionBtn Button
			RetryStr  string
		}{
			ID:        dto.WorkNumber,
			Name:      dto.WorkTitle,
			Link:      fmt.Sprintf(TaskURLTemplate, dto.SdURL, dto.WorkNumber),
			Deadline:  dto.Deadline,
			ActionBtn: *actionBtn,
			RetryStr:  retryStr,
		},
	}
}

func NewRequestApproverInfoTpl(id, name, sdURL, comment string) Template {
	comm := defaultComment

	if comment != "" {
		comm = comment
	}

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s запрос дополнительной информации", id, name),
		Template: "internal/mail/template/15moreInfoRequired-template.html",
		Image:    "15_dop_info_trebuetsya.png",
		Variables: struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Link    string `json:"link"`
			Comment string `json:"comment"`
		}{
			ID:      id,
			Name:    name,
			Link:    fmt.Sprintf(TaskURLTemplate, sdURL, id),
			Comment: comm,
		},
	}
}

func NewAnswerApproverInfoTpl(id, name, sdURL, comment string) Template {
	comm := defaultComment

	if comment != "" {
		comm = comment
	}

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s — Получена дополнительная информация", id, name),
		Template: "internal/mail/template/16additionalInfoReceived-template.html",
		Image:    "16_dop_info_polucheno.png",
		Variables: struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Link    string `json:"link"`
			Comment string `json:"comment"`
		}{
			ID:      id,
			Name:    name,
			Link:    fmt.Sprintf(TaskURLTemplate, sdURL, id),
			Comment: comm,
		},
	}
}

func NewAnswerExecutionInfoTpl(id, name, sdURL, comment string) Template {
	comm := defaultComment

	if comment != "" {
		comm = comment
	}

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s — Получена дополнительная информация", id, name),
		Template: "internal/mail/template/16additionalInfoReceived-template.html",
		Image:    "16_dop_info_polucheno.png",
		Variables: struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Link    string `json:"link"`
			Comment string `json:"comment"`
		}{
			ID:      id,
			Name:    name,
			Link:    fmt.Sprintf(TaskURLTemplate, sdURL, id),
			Comment: comm,
		},
	}
}

func isUser(v interface{}) bool {
	vv, ok := v.(orderedmap.OrderedMap)
	if !ok {
		return false
	}

	if _, oks := vv.Get("fullname"); !oks {
		return false
	}

	if _, oks := vv.Get("fullname"); !oks {
		return false
	}

	return true
}

func retMap(v orderedmap.OrderedMap) map[string]interface{} {
	t := v.Values()

	return t
}

func hasValue(v orderedmap.OrderedMap) bool {
	t := v.Values()

	return len(t) != 0
}

func toMbyte(v int64) string {
	return fmt.Sprintf("%.2f", float64(v)/1000000)
}

func isLink(v interface{}) bool {
	str, ok := v.(string)

	if len(str) < 5 {
		return false
	}

	if ok {
		return str[0:4] == "http"
	}

	return ok
}

//nolint:gocognit // ok here
func CheckGroup(desc []orderedmap.OrderedMap) []orderedmap.OrderedMap {
	for _, item := range desc {
		for key, v := range item.Values() {
			if key == attachExists || key == attachLinks || key == attachList {
				continue
			}

			if boolBlock, ok := v.(bool); ok {
				answer := "Нет"
				if boolBlock {
					answer = "Да"
				}

				item.Set(key, answer)
			}

			field, ok := v.(orderedmap.OrderedMap)
			if !ok {
				continue
			}

			if isUser(field) {
				continue
			}

			groupMap, oks := item.Get(key)
			if !oks {
				continue
			}

			if group, types := groupMap.(orderedmap.OrderedMap); types {
				for keys, dVal := range group.Values() {
					switch itemGroup := dVal.(type) {
					case []interface{}:
						arrayBlock := make([]string, 0, len(itemGroup))

						for _, vars := range itemGroup {
							if str, strOk := vars.(string); strOk {
								arrayBlock = append(arrayBlock, str)
							}
						}

						if cap(arrayBlock) != 0 {
							item.Set(keys, strings.Join(arrayBlock, `, `))

							continue
						}

						if len(itemGroup) != 0 {
							item.Set(keys, dVal)
						}
					default:
						item.Set(keys, dVal)
					}
				}
			}

			item.Delete(key)
		}
	}

	return desc
}

func checkKey(key string) bool {
	switch key {
	case attachExists, attachList, attachLinks:
		return false
	default:
		return true
	}
}

func isFile(v interface{}) bool {
	file, ok := v.(orderedmap.OrderedMap)
	if !ok {
		files, oks := v.([]interface{})
		if !oks {
			return false
		}

		for _, vs := range files {
			vvs, okss := vs.(orderedmap.OrderedMap)
			if !okss {
				return false
			}

			if _, fileOks := vvs.Get("file_id"); fileOks {
				return true
			}
		}
	}

	if _, fileOks := file.Get("file_id"); fileOks {
		return true
	}

	return false
}

func NewAppInitiatorStatusNotificationTpl(dto *SignerNotifTemplate) Template {
	subject := fmt.Sprintf("Заявка № %s %s %s", dto.WorkNumber, dto.Name, dto.Action)
	textPart := fmt.Sprintf(`Уважаемый коллега, <span
                  style="
                    font-family: MTS Text, sans-serif, serif, EmojiFont;
                    font-size: 17px;
                    line-height: 24px;
                    font-weight: 500;
                  "
                  ><strong>заявка № %s %s <b>%s</b>.</span>`, dto.WorkNumber, dto.Name, dto.Action)

	switch dto.Action {
	case "согласована", "выполнена исполнителем", "отклонена", "отклонена исполнителем":
		comment := defaultComment

		if dto.Comment != "" {
			comment = dto.Comment
		}

		textPart = fmt.Sprintf(`Уважаемый коллега, <span
			style="
			font-family: MTS Text, sans-serif, serif, EmojiFont;
			font-size: 17px;
			line-height: 24px;
			font-weight: 500;"
			><strong>заявка № %s %s <b>%s</b></strong> с комментарием: %q.</span>`,
			dto.WorkNumber, dto.Name, dto.Action, comment)
	case "ознакомлено":
		subject = fmt.Sprintf("Ознакомление по заявке № %s %s", dto.WorkNumber, dto.Name)
		textPart = fmt.Sprintf(`Уважаемый коллега, <span
                  style="
                    font-family: MTS Text, sans-serif, serif, EmojiFont;
                    font-size: 17px;
                    line-height: 24px;
                    font-weight: 500;
                  "
                  ><strong>заявка № %s %s получена виза <b>Ознакомлен</b>.</strong></span>`, dto.WorkNumber, dto.Name)
	case "проинформировано":
		subject = fmt.Sprintf("Информирование по заявке № %s %s", dto.WorkNumber, dto.Name)
		textPart = fmt.Sprintf(`Уважаемый коллега, <span
                  style="
                    font-family: MTS Text, sans-serif, serif, EmojiFont;
                    font-size: 17px;
                    line-height: 24px;
                    font-weight: 500;
                  "
                  ><strong>заявка № %s %s получена виза <b>Проинформирован</b>.</strong></span>`, dto.WorkNumber, dto.Name)
	}

	dto.Description = CheckGroup(dto.Description)

	return Template{
		Subject:  subject,
		Template: "internal/mail/template/40newAppInitiator-template.html",
		Image:    "40_answer_po_zayavke.png",
		Variables: struct {
			Body        string
			Description []orderedmap.OrderedMap
			Link        string
			RateURL     string
		}{
			Body:        textPart,
			Description: dto.Description,
			Link:        fmt.Sprintf(TaskURLTemplate, dto.SdURL, dto.WorkNumber),
			RateURL:     fmt.Sprintf(taskRateURLTemplate, dto.JocastaURL, dto.WorkNumber),
		},
	}
}

type NewAppPersonStatusTpl struct {
	WorkNumber  string
	Name        string
	Status      string
	Action      string
	DeadLine    string
	Description []orderedmap.OrderedMap
	SdURL       string
	Mailto      string
	Login       string
	Initiator   *sso.UserInfo

	BlockID                   string
	ExecutionDecisionExecuted string
	ExecutionDecisionRejected string

	AttachLinks  []file_registry.AttachInfo `json:"attachLinks"`
	AttachExists bool                       `json:"attachExists"`
	AttachFields []string                   `json:"attachFields"`

	// actions for approver
	ApproverActions    []Action
	AdditionalApprover []string

	IsEditable bool

	LastWorks []*entity.EriusTask
}

func NewSignerNotificationTpl(dto *SignerNotifTemplate) Template {
	dto.Description = CheckGroup(dto.Description)

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s ожидает подписания", dto.WorkNumber, dto.Name),
		Template: "internal/mail/template/26applicationIsAwaitingSignature-template.html",
		Image:    "26_zayavka_ojidaet_podpis.png",
		Variables: struct {
			ID          string
			Name        string
			Link        string
			Description []orderedmap.OrderedMap
			Deadline    string
			AutoReject  bool
		}{
			ID:          dto.WorkNumber,
			Name:        dto.Name,
			Link:        fmt.Sprintf(TaskURLTemplate, dto.SdURL, dto.WorkNumber),
			Deadline:    dto.Deadline,
			Description: dto.Description,
			AutoReject:  dto.AutoReject,
		},
	}
}

const (
	statusExecution = "processing"
)

func NewAppPersonStatusNotificationTpl(in *NewAppPersonStatusTpl) (Template, []Button) {
	actionName := getApprovementActionNameByStatus(in.Status, in.Action)

	buttons := make([]Button, 0)
	template := ""

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
		template = "internal/mail/template/12applicationIsAwaitingExecution-template.html"

	case script.SettingStatusApprovement, script.SettingStatusApproveConfirm, script.SettingStatusApproveView,
		script.SettingStatusApproveInform, script.SettingStatusApproveSign, script.SettingStatusApproveSignUkep:
		buttons = getApproverButtons(in.WorkNumber, in.Mailto, in.BlockID, in.Login, in.ApproverActions, in.IsEditable, false)
		template = "internal/mail/template/11receivedForApproval-template.html"
	}

	lastWorksTemplate := getLastWorksForTemplate(in.LastWorks, in.SdURL)
	in.Description = CheckGroup(in.Description)

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s ожидает %s", in.WorkNumber, in.Name, actionName),
		Template: template,
		Image:    "11_postupila_na_sogl.png",
		Variables: struct {
			ID          string
			Name        string
			Link        string
			Action      string
			Description []orderedmap.OrderedMap
			Deadline    string
			ActionBtn   []Button
			Initiator   *sso.UserInfo
			JobTitle    string
			LastWorks   LastWorks
		}{
			ID:          in.WorkNumber,
			Name:        in.Name,
			Link:        fmt.Sprintf(TaskURLTemplate, in.SdURL, in.WorkNumber),
			Action:      actionName,
			Description: in.Description,
			Deadline:    in.DeadLine,
			ActionBtn:   buttons,
			Initiator:   in.Initiator,
			LastWorks:   lastWorksTemplate,
		},
	}, buttons
}

func NewSendToInitiatorEditTpl(id, name, sdURL, comment string) Template {
	comm := defaultComment

	if comment != "" {
		comm = comment
	}

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s требует доработки", id, name),
		Template: "internal/mail/template/17needsImprovement-template.html",
		Image:    "17_nujna_dorabotka.png",
		Variables: struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Link    string `json:"link"`
			Comment string `json:"comment"`
		}{
			ID:      id,
			Name:    name,
			Link:    fmt.Sprintf(TaskURLTemplate, sdURL, id),
			Comment: comm,
		},
	}
}

func NewExecutionNeedTakeInWorkTpl(dto *ExecutorNotifTemplate) Template {
	actionSubject := fmt.Sprintf(subjectTpl, dto.BlockID, "", dto.WorkNumber, executionStartWorkAction, dto.Login)
	actionBtn := getButton(dto.Mailto, actionSubject, "")

	subject := ""

	if dto.IsGroup {
		subject = fmt.Sprintf("Заявка № %s %s назначена на Группу исполнителей", dto.WorkNumber, dto.Name)
	} else {
		subject = fmt.Sprintf("Заявка № %s %s назначена на исполнение", dto.WorkNumber, dto.Name)
	}

	lastWorksTemplate := getLastWorksForTemplate(dto.LastWorks, dto.SdURL)

	dto.Description = CheckGroup(dto.Description)

	comm := defaultComment

	if dto.Comment != "" {
		comm = dto.Comment
	}

	return Template{
		Subject:  subject,
		Template: "internal/mail/template/27reassignment-template.html",
		Image:    "27_zayavka_ojidaet_isp.png",
		Variables: struct {
			ID          string
			Name        string
			Link        string
			Description []orderedmap.OrderedMap
			Group       bool
			Deadline    string
			ActionBtn   Button
			LastWorks   LastWorks
			Comment     string
		}{
			ID:          dto.WorkNumber,
			Name:        dto.Name,
			Link:        fmt.Sprintf(TaskURLTemplate, dto.SdURL, dto.WorkNumber),
			Description: dto.Description,
			Group:       dto.IsGroup,
			Deadline:    dto.Deadline,
			ActionBtn:   *actionBtn,
			LastWorks:   lastWorksTemplate,
			Comment:     comm,
		},
	}
}

func NewExecutionTakenInWorkTpl(dto *ExecutorNotifTemplate) Template {
	lastWorksTemplate := getLastWorksForTemplate(dto.LastWorks, dto.SdURL)
	dto.Description = CheckGroup(dto.Description)

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s взята в работу пользователем %s", dto.WorkNumber, dto.Name, dto.Executor.FullName),
		Template: "internal/mail/template/05applicationAccepted-template.html",
		Image:    "05_zayavka_vzyata_v_rabotu.png",
		Variables: struct {
			ID          string
			Name        string
			Link        string
			Executor    *sso.UserInfo
			Initiator   *sso.UserInfo
			Description []orderedmap.OrderedMap
			LastWorks   LastWorks
		}{
			ID:          dto.WorkNumber,
			Name:        dto.Name,
			Link:        fmt.Sprintf(TaskURLTemplate, dto.SdURL, dto.WorkNumber),
			Executor:    dto.Executor,
			Initiator:   dto.Initiator,
			Description: dto.Description,
			LastWorks:   lastWorksTemplate,
		},
	}
}

func NewAddApproversTpl(in *NewAppPersonStatusTpl, recipientEmail string) (Template, []Button) {
	lastWorksTemplate := getLastWorksForTemplate(in.LastWorks, in.SdURL)
	actionName := getApprovementActionNameByStatus(in.Status, defaultApprovementActionName)

	in.Description = CheckGroup(in.Description)

	emails := strings.Split(recipientEmail, "@")
	actions := []Action{{InternalActionName: "approved"}, {InternalActionName: "rejected"}}
	buttons := getApproverButtons(in.WorkNumber, in.Mailto, in.BlockID, emails[0], actions, false, true)

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s ожидает %s", in.WorkNumber, in.Name, actionName),
		Template: "internal/mail/template/42receivedForApproval-template.html",
		Image:    "42_zayavka_ojidaet_sogl.png",
		Variables: struct {
			ID        string    `json:"id"`
			Name      string    `json:"name"`
			Link      string    `json:"link"`
			Deadline  string    `json:"deadline"`
			Action    string    `json:"action"`
			LastWorks LastWorks `json:"last_works"`

			Description []orderedmap.OrderedMap
			ActionBtn   []Button
			Initiator   *sso.UserInfo
		}{
			ID:        in.WorkNumber,
			Name:      in.Name,
			Link:      fmt.Sprintf(TaskURLTemplate, in.SdURL, in.WorkNumber),
			Action:    actionName,
			Deadline:  in.DeadLine,
			LastWorks: lastWorksTemplate,

			Description: in.Description,
			ActionBtn:   buttons,
			Initiator:   in.Initiator,
		},
	}, buttons
}

func NewDecisionMadeByAdditionalApprover(tmpl *ReviewTemplate) Template {
	return Template{
		Subject:  fmt.Sprintf("Получена рецензия по Заявке № %s %s", tmpl.ID, tmpl.Name),
		Template: "internal/mail/template/18reviewReceived-template.html",
		Image:    "18_poluchena_recenziya.png",
		Variables: struct {
			ID          string                     `json:"id"`
			Name        string                     `json:"name"`
			Link        string                     `json:"link"`
			Decision    string                     `json:"decision"`
			Comment     string                     `json:"comment"`
			Author      *sso.UserInfo              `json:"author"`
			AttachLinks []file_registry.AttachInfo `json:"attachLinks"`
			AttachExist bool                       `json:"attachExist"`
		}{
			ID:          tmpl.ID,
			Name:        tmpl.Name,
			Decision:    tmpl.Decision,
			Comment:     tmpl.Comment,
			Link:        fmt.Sprintf(TaskURLTemplate, tmpl.SdURL, tmpl.ID),
			Author:      tmpl.Author,
			AttachLinks: tmpl.AttachLinks,
			AttachExist: tmpl.AttachExist,
		},
	}
}

func NewDayBeforeRequestAddInfoSLABreached(id, name, sdURL string) Template {
	return Template{
		Subject:  fmt.Sprintf("По заявке № %s %s требуется дополнительная информация", id, name),
		Template: "internal/mail/template/41infoWithinOneBusinessDay-template.html",
		Image:    "41_neobhodimo_dop_info.png",
		Variables: struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			ID:   id,
			Name: name,
			Link: fmt.Sprintf(TaskURLTemplate, sdURL, id),
		},
	}
}

func NewRequestAddInfoSLABreached(id, name, sdURL string, _ int) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s автоматически перенесена в архив", id, name),
		Template: "internal/mail/template/36notGetDopInfo-template.html",
		Image:    "36_avto_perenesena_v_archiv.png",
		Variables: struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			ID:   id,
			Name: name,
			Link: fmt.Sprintf(TaskURLTemplate, sdURL, id),
		},
	}
}

func NewInvalidFunctionResp(id, name, sdURL string) Template {
	return Template{
		Subject:  fmt.Sprintf("По заявке № %s %s не удалось получить обратную связь от внешней системы", id, name),
		Template: "internal/mail/template/35errorRespOtherSystem-template.html",
		Image:    "35_ne_poluchili_obr_svyaz'.png",
		Variables: struct {
			ID   string
			Name string
			Link string
		}{
			ID:   id,
			Name: name,
			Link: fmt.Sprintf(TaskURLTemplate, sdURL, id),
		},
	}
}

func NewFormExecutionTakenInWorkTpl(dto *ExecutorNotifTemplate) Template {
	dto.Description = CheckGroup(dto.Description)

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s взята в работу пользователем %s", dto.WorkNumber, dto.Name, dto.Executor.FullName),
		Template: "internal/mail/template/05applicationAccepted-template.html",
		Image:    "05_zayavka_vzyata_v_rabotu.png",
		Variables: struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Link        string `json:"link"`
			Executor    *sso.UserInfo
			Initiator   *sso.UserInfo
			Description []orderedmap.OrderedMap `json:"description"`
			LastWorks   LastWorks               `json:"last_works"`
		}{
			ID:          dto.WorkNumber,
			Name:        dto.Name,
			Link:        fmt.Sprintf(TaskURLTemplate, dto.SdURL, dto.WorkNumber),
			Executor:    dto.Executor,
			Initiator:   dto.Initiator,
			Description: nil,
			LastWorks:   nil,
		},
	}
}

func NewFormPersonExecutionNotificationTemplate(workNumber, workTitle, sdURL, deadline string) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s - Необходимо предоставить информацию", workNumber, workTitle),
		Template: "internal/mail/template/29form-template.html",
		Image:    "29_neobhodimo_predostavit'_info.png",
		Variables: struct {
			ID       string
			Name     string
			Link     string
			Deadline string
		}{
			ID:       workNumber,
			Name:     workTitle,
			Link:     fmt.Sprintf(TaskURLTemplate, sdURL, workNumber),
			Deadline: deadline,
		},
	}
}

func NewTaskRejectedWithCommentTemplate(workNumber, workTitle, sdURL, comment string) Template {
	comm := defaultComment

	if comment != "" {
		comm = comment
	}

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s - отозвана", workNumber, workTitle),
		Template: "internal/mail/template/24applicationWithdrawn-template.html",
		Image:    "24_zayavka_otozvana_inic.png",
		Variables: struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Link        string `json:"link"`
			WithComment string `json:"withComment"`
		}{
			ID:          workNumber,
			Name:        workTitle,
			Link:        fmt.Sprintf(TaskURLTemplate, sdURL, workNumber),
			WithComment: fmt.Sprintf(" с комментарием: %q", comm),
		},
	}
}

func NewTaskRejectedTemplate(workNumber, workTitle, sdURL string) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s - отозвана", workNumber, workTitle),
		Template: "internal/mail/template/24applicationWithdrawn-template.html",
		Image:    "24_zayavka_otozvana_inic.png",
		Variables: struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Link        string `json:"link"`
			WithComment string `json:"withComment"`
		}{
			ID:          workNumber,
			Name:        workTitle,
			Link:        fmt.Sprintf(TaskURLTemplate, sdURL, workNumber),
			WithComment: "",
		},
	}
}

func NewFunctionNotify(funcName, funcVersion string, versions []script.VersionsByFunction) Template {
	activeVersions := make([]script.VersionsByFunction, 0)
	draftVersions := make([]script.VersionsByFunction, 0)

	for i := range versions {
		if versions[i].Status == 2 {
			activeVersions = append(activeVersions, versions[i])
		}

		if versions[i].Status == 1 {
			draftVersions = append(draftVersions, versions[i])
		}
	}

	return Template{
		Subject:  "У функции из вашего сценария появилась новая версия",
		Template: "internal/mail/template/43notifyNewFunction-template.html",
		Image:    "43_notify_new_function.png",
		Variables: struct {
			FunctionName    string
			FunctionVersion string
			HaveActive      bool
			HaveDraft       bool
			ActiveVersions  []script.VersionsByFunction
			DraftVersions   []script.VersionsByFunction
		}{
			FunctionName:    funcName,
			FunctionVersion: funcVersion,
			HaveActive:      len(activeVersions) > 0,
			HaveDraft:       len(draftVersions) > 0,
			ActiveVersions:  activeVersions,
			DraftVersions:   draftVersions,
		},
	}
}

func NewSignSLAExpiredTemplate(workNumber, workTitle, sdURL string) Template {
	return Template{
		Subject:  fmt.Sprintf("По заявке № %s %s- истекло время подписания", workNumber, workTitle),
		Template: "internal/mail/template/37SignIsteklo-template.html",
		Image:    "37_isteklo_vremja_podpisanija.png",
		Variables: struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			ID:   workNumber,
			Name: workTitle,
			Link: fmt.Sprintf(TaskURLTemplate, sdURL, workNumber),
		},
	}
}

func NewAppCompletedTemplate(dto *ProcessFinishedTemplate) Template {
	dto.Description = CheckGroup(dto.Description)

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s завершена", dto.WorkNumber, dto.Name),
		Template: "internal/mail/template/44appCompleted-template.html",
		Image:    "05_zayavka_vzyata_v_rabotu.png",
		Variables: struct {
			ID          string
			Name        string
			Link        string
			Description []orderedmap.OrderedMap
		}{
			ID:          dto.WorkNumber,
			Name:        dto.Name,
			Link:        fmt.Sprintf(TaskURLTemplate, dto.SdURL, dto.WorkNumber),
			Description: dto.Description,
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

func NewSignErrorTemplate(workNumber, name, sdURL string) Template {
	return Template{
		Subject:  fmt.Sprintf("По заявке № %s - возникла ошибка подписания", workNumber),
		Template: "internal/mail/template/31signingError-template.html",
		Image:    "31_oshibka_podpisaniya.png",
		Variables: struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			ID:   workNumber,
			Name: name,
			Link: fmt.Sprintf(TaskURLTemplate, sdURL, workNumber),
		},
	}
}

type Action struct {
	Title              string
	InternalActionName string
}

func getButton(to, subject, image string) *Button {
	subject = strings.ReplaceAll(subject, " ", "")

	body := "***КОММЕНТАРИЙ%20НИЖЕ***%0D%0A%0D%0A***ОБЩИЙ%20РАЗМЕР%20ВЛОЖЕНИЙ%20НЕ%20БОЛЕЕ%2020МБ***"
	href := fmt.Sprintf("mailto:%s?subject=%s&body=%s", to, subject, body)

	return &Button{
		Href: href,
		Img:  image,
	}
}

const (
	subjectTpl = "step_name=%s|decision=%s|work_number=%s|action_name=%s|login=%s"

	actionApproverSendEditApp           = "approver_send_edit_app"
	actionExecutorSendEditApp           = "executor_send_edit_app"
	taskUpdateActionExecution           = "execution"
	taskUpdateActionApprovement         = "approvement"
	taskUpdateActionAdditionApprovement = "additional_approvement"
	executionStartWorkAction            = "executor_start_work"
	formExecutorStartWorkAction         = "form_executor_start_work"
	actionApproverSignUkep              = "sign_ukep"
)

func getApproverButtons(workNumber, mailto, blockID, login string, actions []Action, isEditable, isAdditionalApprover bool) []Button {
	buttons := make([]Button, 0, len(actions))

	for i := range actions {
		if actions[i].InternalActionName == actionApproverSignUkep {
			return nil
		}

		if actions[i].InternalActionName == actionApproverSendEditApp {
			continue
		}

		action := taskUpdateActionApprovement

		if isAdditionalApprover {
			action = taskUpdateActionAdditionApprovement
		}

		var img string

		switch actions[i].InternalActionName {
		case "approved", "approve":
			img = "soglas.png"
		case "rejected", "reject":
			img = "otklon.png"
		case "informed":
			img = "proinform.png"
		case "confirm":
			img = "utverdit.png"
		case "sign":
			img = "podpisat.png"
		case "viewed":
			img = "oznakomlen.png"
		}

		subject := fmt.Sprintf(
			subjectTpl,
			blockID,
			actions[i].InternalActionName,
			workNumber,
			action,
			login,
		)

		buttons = append(buttons, *getButton(mailto, subject, img))
	}

	if isEditable {
		sendEditAppSubject := fmt.Sprintf(subjectTpl, blockID, "", workNumber, actionApproverSendEditApp, login)
		sendEditAppBtn := getButton(mailto, sendEditAppSubject, "vernut.png")
		buttons = append(buttons, *sendEditAppBtn)
	}

	return buttons
}

func getExecutionButtons(workNumber, mailto, blockID, executed, rejected, login string, isEditable bool) []Button {
	executedSubject := fmt.Sprintf(subjectTpl, blockID, executed, workNumber, taskUpdateActionExecution, login)
	executedBtn := getButton(mailto, executedSubject, "reshit.png")

	rejectedSubject := fmt.Sprintf(subjectTpl, blockID, rejected, workNumber, taskUpdateActionExecution, login)
	rejectedBtn := getButton(mailto, rejectedSubject, "otklon.png")

	buttons := []Button{
		*executedBtn,
		*rejectedBtn,
	}

	if isEditable {
		sendEditAppSubject := fmt.Sprintf(subjectTpl, blockID, "", workNumber, actionExecutorSendEditApp, login)
		sendEditAppBtn := getButton(mailto, sendEditAppSubject, "vernut.png")
		buttons = append(buttons, *sendEditAppBtn)
	}

	return buttons
}

func getLastWorksForTemplate(lastWorks []*entity.EriusTask, sdURL string) LastWorks {
	lastWorksTemplate := make(LastWorks, 0, len(lastWorks))

	for _, task := range lastWorks {
		lastWorksTemplate = append(lastWorksTemplate, &LastWork{
			WorkNumber: task.WorkNumber,
			Name:       task.Name,
			DaysAgo:    int(math.Round(utils.GetDateUnitNumBetweenDates(task.StartedAt, time.Now(), utils.Day))),
			Link:       fmt.Sprintf(TaskURLTemplate, sdURL, task.WorkNumber),
		})
	}

	return lastWorksTemplate
}

type Button struct {
	Href string `json:"href"`
	Img  string `json:"img"`
}
