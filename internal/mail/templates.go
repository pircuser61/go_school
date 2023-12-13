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
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/file-registry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	defaultApprovementActionName = "согласования"

	TaskUrlTemplate = "%s/applications/details/%s"
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
	WorkNumber  string
	Name        string
	SdUrl       string
	Deadline    string
	AutoReject  bool
	Description []orderedmap.OrderedMap
	Action      string
}

type ExecutorNotifTemplate struct {
	WorkNumber  string
	Name        string
	SdUrl       string
	Executor    *sso.UserInfo
	Initiator   *sso.UserInfo
	Description []orderedmap.OrderedMap
	BlockID     string
	Mailto      string
	Login       string
	IsGroup     bool
	LastWorks   []*entity.EriusTask
	Deadline    string
}

type LastWork struct {
	WorkNumber string `json:"work_number"`
	Name       string `json:"work_title"`
	DaysAgo    int    `json:"days_ago"`
	Link       string `json:"work_url"`
}

type LastWorks []*LastWork

//nolint:dupl // not duplicate
func NewApprovementSLATpl(id, name, sdUrl, status string) Template {
	actionName := getApprovementActionNameByStatus(status, defaultApprovementActionName)

	return Template{
		Subject:  fmt.Sprintf("Поf  заявке %s %s истекло время %s", id, name, actionName),
		Template: "internal/mail/template/13approvalHasExpired-template.html",
		Image:    "isteklo_ispolnenie.png",
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

//nolint:dupl // not duplicate
func NewApprovementHalfSLATpl(id, name, sdUrl, status, deadline string, lastWorks []*entity.EriusTask) Template {
	actionName := getApprovementActionNameByStatus(status, defaultApprovementActionName)

	lastWorksTemplate := getLastWorksForTemplate(lastWorks, sdUrl)

	return Template{
		Subject:  fmt.Sprintf("По заявке %s %s истекает время %s", id, name, actionName),
		Template: "internal/mail/template/14approvalExpires-template.html",
		Image:    "istekaet_soglasovanie.png",
		Variables: struct {
			Id        string    `json:"id"`
			Name      string    `json:"name"`
			Link      string    `json:"link"`
			Deadline  string    `json:"deadline"`
			LastWorks LastWorks `json:"last_works"`
		}{
			Id:        id,
			Name:      name,
			Link:      fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			Deadline:  deadline,
			LastWorks: lastWorksTemplate,
		},
	}
}

func NewExecutionSLATpl(id, name, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("По заявке %s %s истекло время исполнения", id, name),
		Template: "internal/mail/template/19executionExpired-template.html",
		Image:    "isteklo_ispolnenie.png",
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

func NewFormSLATpl(id, name, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("По заявке №%s %s истекло время предоставления дополнительной информации", id, name),
		Template: "internal/mail/template/32dopInfoIsteklo-template.html",
		Image:    "dop_info_isteklo.png",
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

func NewExecutiontHalfSLATpl(id, name, sdUrl, deadline string, lastWorks []*entity.EriusTask) Template {
	lastWorksTemplate := getLastWorksForTemplate(lastWorks, sdUrl)
	return Template{
		Subject:  fmt.Sprintf("По заявке %s %s истекает время исполнения", id, name),
		Template: "internal/mail/template/20executionExpires-template.html",
		Image:    "istekaet_ispolnenie.png",
		Variables: struct {
			Id        string    `json:"id"`
			Name      string    `json:"name"`
			Link      string    `json:"link"`
			Deadline  string    `json:"deadline"`
			LastWorks LastWorks `json:"last_works"`
		}{
			Id:        id,
			Name:      name,
			Link:      fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			Deadline:  deadline,
			LastWorks: lastWorksTemplate,
		},
	}
}

func NewFormDayHalfSLATpl(id, name, sdUrl, deadline string) Template {
	return Template{
		Subject:  fmt.Sprintf("По заявке №%s %s истекает время предоставления информации", id, name),
		Template: "internal/mail/template/33dopInfoIstekaet-template.html",
		Image:    "dop_info_istekaet.png",
		Variables: struct {
			Name     string `json:"name"`
			Id       string `json:"id"`
			Link     string `json:"link"`
			Deadline string `json:"deadline"`
		}{
			Name:     name,
			Id:       id,
			Link:     fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			Deadline: deadline,
		},
	}
}

func NewReworkSLATpl(id, name, sdUrl string, reworkSla int, checkSla bool) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка %s %s автоматически перенесена в архив", id, name),
		Template: "internal/mail/template/34rejectToarchive-template.html",
		Image:    "istekla_dorabotka.png",
		Variables: struct {
			Id       string `json:"id"`
			Name     string `json:"name"`
			Duration string `json:"duration"`
			Link     string `json:"link"`
			CheckSLA bool   `json:"checkSLA"`
		}{
			Id:       id,
			Name:     name,
			Duration: strconv.Itoa(reworkSla / 8),
			Link:     fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			CheckSLA: checkSla,
		},
	}
}

func NewRequestExecutionInfoTpl(id, name, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка %s %s запрос дополнительной информации", id, name),
		Template: "internal/mail/template/15moreInfoRequired-template.html",
		Image:    "dop_info.png",
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

func NewRequestFormExecutionInfoTpl(id, name, sdUrl, deadline string, isReentry bool) Template {
	var retryStr string

	if isReentry {
		retryStr = " повторно"
	}

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s - Необходимо%s предоставить информацию", id, name, retryStr),
		Template: "internal/mail/template/29form-template.html",
		Image:    "dop_info.png",
		Variables: struct {
			Id       string
			Name     string
			Link     string
			Deadline string
		}{
			Id:       id,
			Name:     name,
			Link:     fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			Deadline: deadline,
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
	actionBtn := getButton(dto.Mailto, actionSubject, "Взять в работу", "")

	var retryStr string
	if isReentry {
		retryStr = " повторно"
	}

	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s - Необходимо%s предоставить информацию", dto.WorkNumber, dto.WorkTitle, retryStr),
		Template: "internal/mail/template/39takeInWork-template.html",
		Image:    "dop_info.png",
		Variables: struct {
			Id        string
			Name      string
			Link      string
			Deadline  string
			ActionBtn Button
		}{
			Id:        dto.WorkNumber,
			Name:      dto.WorkTitle,
			Link:      fmt.Sprintf(TaskUrlTemplate, dto.SdUrl, dto.WorkNumber),
			Deadline:  dto.Deadline,
			ActionBtn: *actionBtn,
		},
	}
}

func NewRequestApproverInfoTpl(id, name, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка %s %s запрос дополнительной информации", id, name),
		Template: "internal/mail/template/15moreInfoRequired-template.html",
		Image:    "dop_info.png",
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
		Subject:  fmt.Sprintf("Заявка %s %s запрос дополнительной информации", id, name),
		Template: "internal/mail/template/16additionalInfoReceived-template.html",
		Image:    "dop_info_poluchena.png",
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

func checkKey(key string) bool {
	switch key {
	case "attachExist", "attachList", "attachLinks":
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
	subject := fmt.Sprintf("Заявка %s %s %s", dto.WorkNumber, dto.Name, dto.Action)
	textPart := fmt.Sprintf(`Уважаемый коллега, <span style="font-weight: 500;">заявка %s %s <b>%s</b></span>`, dto.WorkNumber, dto.Name, dto.Action)

	if dto.Action == "ознакомлено" {
		subject = fmt.Sprintf("Ознакомление по заявке %s %s", dto.WorkNumber, dto.Name)
		textPart = fmt.Sprintf(`Уважаемый коллега,<span style="font-weight: 500;"> заявка %s %s получена виза <b>Ознакомлен</b></span>`, dto.WorkNumber, dto.Name)
	}

	if dto.Action == "проинформировано" {
		subject = fmt.Sprintf("Информирование по заявке %s %s", dto.WorkNumber, dto.Name)
		textPart = fmt.Sprintf(`Уважаемый коллега, <span style="font-weight: 500;">заявка %s %s получена виза <b>Проинформирован</b></span>`, dto.WorkNumber, dto.Name)
	}

	return Template{
		Subject:  subject,
		Template: "internal/mail/template/40newAppInitiator-template.html",
		Image:    "new_zayavka.png",
		Variables: struct {
			Body        string                  `json:"body"`
			Description []orderedmap.OrderedMap `json:"description"`
			Link        string                  `json:"link"`
		}{
			Body:        textPart,
			Description: dto.Description,
			Link:        fmt.Sprintf(TaskUrlTemplate, dto.SdUrl, dto.WorkNumber),
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
	SdUrl       string
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
	ApproverActions []Action

	IsEditable bool

	LastWorks []*entity.EriusTask
}

func NewSignerNotificationTpl(dto *SignerNotifTemplate) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка №%s %s ожидает подписания", dto.WorkNumber, dto.Name),
		Template: "internal/mail/template/26applicationIsAwaitingSignature-template.html",
		Image:    "ozhidaet_podpisaniya.png",
		Variables: struct {
			Id          string
			Name        string
			Link        string
			Description []orderedmap.OrderedMap
			Deadline    string
			AutoReject  bool
		}{
			Id:          dto.WorkNumber,
			Name:        dto.Name,
			Link:        fmt.Sprintf(TaskUrlTemplate, dto.SdUrl, dto.WorkNumber),
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
		buttons = getApproverButtons(in.WorkNumber, in.Mailto, in.BlockID, in.Login, in.ApproverActions, in.IsEditable)
		template = "internal/mail/template/11receivedForApproval-template.html"
	}
	lastWorksTemplate := getLastWorksForTemplate(in.LastWorks, in.SdUrl)

	return Template{
		Subject:  fmt.Sprintf("Заявка %s %s ожидает %s", in.WorkNumber, in.Name, actionName),
		Template: template,
		Image:    "ozhidaet_ispolneniya.png",
		Variables: struct {
			Id          string
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
			Id:          in.WorkNumber,
			Name:        in.Name,
			Link:        fmt.Sprintf(TaskUrlTemplate, in.SdUrl, in.WorkNumber),
			Action:      actionName,
			Description: in.Description,
			Deadline:    in.DeadLine,
			ActionBtn:   buttons,
			Initiator:   in.Initiator,
			LastWorks:   lastWorksTemplate,
		},
	}, buttons
}

func NewSendToInitiatorEditTpl(id, name, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка %s %s требует доработки", id, name),
		Template: "internal/mail/template/17needsImprovement-template.html",
		Image:    "nuzhna_dorabotka.png",
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
	actionBtn := getButton(dto.Mailto, actionSubject, "Взять в работу", "")

	subject := ""

	if dto.IsGroup {
		subject = fmt.Sprintf("Заявка №%s %s назначена на Группу исполнителей", dto.WorkNumber, dto.Name)
	} else {
		subject = fmt.Sprintf("Заявка №%s %s назначена на исполнение", dto.WorkNumber, dto.Name)
	}

	lastWorksTemplate := getLastWorksForTemplate(dto.LastWorks, dto.SdUrl)

	return Template{
		Subject:  subject,
		Template: "internal/mail/template/27reassignment-template.html",
		Image:    "ozhidaet_ispolneniya.png",
		Variables: struct {
			Id          string
			Name        string
			Link        string
			Description []orderedmap.OrderedMap
			Group       bool
			Deadline    string
			ActionBtn   Button
			LastWorks   LastWorks
		}{
			Id:          dto.WorkNumber,
			Name:        dto.Name,
			Link:        fmt.Sprintf(TaskUrlTemplate, dto.SdUrl, dto.WorkNumber),
			Description: dto.Description,
			Group:       dto.IsGroup,
			Deadline:    dto.Deadline,
			ActionBtn:   *actionBtn,
			LastWorks:   lastWorksTemplate,
		},
	}
}

func NewExecutionTakenInWorkTpl(dto *ExecutorNotifTemplate) Template {
	lastWorksTemplate := getLastWorksForTemplate(dto.LastWorks, dto.SdUrl)
	return Template{
		Subject:  fmt.Sprintf("Заявка №%s %s взята в работу пользователем %s", dto.WorkNumber, dto.Name, dto.Executor.FullName),
		Template: "internal/mail/template/05applicationAccepted-template.html",
		Image:    "zayavka_vzyata_v_rabotu.png",
		Variables: struct {
			Id          string
			Name        string
			Link        string
			Executor    *sso.UserInfo
			Initiator   *sso.UserInfo
			Description []orderedmap.OrderedMap
			LastWorks   LastWorks
		}{
			Id:          dto.WorkNumber,
			Name:        dto.Name,
			Link:        fmt.Sprintf(TaskUrlTemplate, dto.SdUrl, dto.WorkNumber),
			Executor:    dto.Executor,
			Initiator:   dto.Initiator,
			Description: dto.Description,
			LastWorks:   lastWorksTemplate,
		},
	}
}

func NewAddApproversTpl(id, name, sdUrl, deadline string, lastWorks []*entity.EriusTask) Template {
	lastWorksTemplate := getLastWorksForTemplate(lastWorks, sdUrl)
	return Template{
		Subject:  fmt.Sprintf("Заявка %s %s ожидает согласования", id, name),
		Template: "internal/mail/template/42receivedForApproval-template.html",
		Image:    "ozhidaet_ispolneniya.png",
		Variables: struct {
			Id        string    `json:"id"`
			Name      string    `json:"name"`
			Link      string    `json:"link"`
			Deadline  string    `json:"deadline"`
			LastWorks LastWorks `json:"last_works"`
		}{
			Id:        id,
			Name:      name,
			Link:      fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			Deadline:  deadline,
			LastWorks: lastWorksTemplate,
		},
	}
}

func NewDecisionMadeByAdditionalApprover(id, name, decision, comment, sdUrl string, author *sso.UserInfo) Template {
	return Template{
		Subject:  fmt.Sprintf("Получена рецензия по Заявке №%s %s", id, name),
		Template: "internal/mail/template/18reviewReceived-template.html",
		Image:    "poluchena_retsenzia.png",
		Variables: struct {
			Id       string `json:"id"`
			Name     string `json:"name"`
			Link     string `json:"link"`
			Decision string `json:"decision"`
			Comment  string `json:"comment"`
			Author   *sso.UserInfo
		}{
			Id:       id,
			Name:     name,
			Decision: decision,
			Comment:  comment,
			Link:     fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			Author:   author,
		},
	}
}

func NewDayBeforeRequestAddInfoSLABreached(id, name, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("По заявке №%s %s требуется дополнительная информация", id, name),
		Template: "internal/mail/template/41infoWithinOneBusinessDay-template.html",
		Image:    "dop_info.png",
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

func NewRequestAddInfoSLABreached(id, name, sdUrl string, reworkSla int) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка №%s %s автоматически перенесена в архив", id, name),
		Template: "internal/mail/template/36notGetDopInfo-template.html",
		Image:    "dop_info_isteklo.png",
		Variables: struct {
			Id       string `json:"id"`
			Name     string `json:"name"`
			Link     string `json:"link"`
			Duration string `json:"duration"`
		}{
			Id:       id,
			Name:     name,
			Link:     fmt.Sprintf(TaskUrlTemplate, sdUrl, id),
			Duration: strconv.Itoa(reworkSla / 8),
		},
	}
}

func NewInvalidFunctionResp(id, name, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("По заявке №%s %s не удалось получить обратную связь от внешней системы", id, name),
		Template: "internal/mail/template/35errorRespOtherSystem-template.html",
		Image:    "oshibka_other.png",
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

func NewFormExecutionTakenInWorkTpl(dto *ExecutorNotifTemplate) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s взята в работу пользователем %s", dto.WorkNumber, dto.Name, dto.Executor.FullName),
		Template: "internal/mail/template/05applicationAccepted-template.html",
		Image:    "zayavka_vzyata_v_rabotu.png",
		Variables: struct {
			Id          string `json:"id"`
			Name        string `json:"name"`
			Link        string `json:"link"`
			Executor    *sso.UserInfo
			Initiator   *sso.UserInfo
			Description []orderedmap.OrderedMap `json:"description"`
			LastWorks   LastWorks               `json:"last_works"`
		}{
			Id:          dto.WorkNumber,
			Name:        dto.Name,
			Link:        fmt.Sprintf(TaskUrlTemplate, dto.SdUrl, dto.WorkNumber),
			Executor:    dto.Executor,
			Initiator:   dto.Initiator,
			Description: nil,
			LastWorks:   nil,
		},
	}
}

func NewFormPersonExecutionNotificationTemplate(workNumber, workTitle, sdUrl, deadline string) Template {
	return Template{
		Subject:  fmt.Sprintf("Заявка № %s %s - Необходимо предоставить информацию", workNumber, workTitle),
		Template: "internal/mail/template/29form-template.html",
		Image:    "dop_info.png",
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
	return Template{
		Subject:  fmt.Sprintf("По заявке № %s %s- истекло время подписания", workNumber, workTitle),
		Template: "internal/mail/template/37SignIsteklo-template.html",
		Image:    "isteklo_podpisanie.png",
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
func NewSignErrorTemplate(workNumber, name, sdUrl string) Template {
	return Template{
		Subject:  fmt.Sprintf("По заявке № %s - возникла ошибка подписания", workNumber),
		Template: "internal/mail/template/31signingError-template.html",
		Image:    "oshibka_podisania.png",
		Variables: struct {
			Id   string `json:"id"`
			Name string `json:"name"`
			Link string `json:"link"`
		}{
			Id:   workNumber,
			Name: name,
			Link: fmt.Sprintf(TaskUrlTemplate, sdUrl, workNumber),
		},
	}
}

type Action struct {
	Title              string
	InternalActionName string
}

func getButton(to, subject, title, image string) *Button {
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

	actionApproverSendEditApp   = "approver_send_edit_app"
	actionExecutorSendEditApp   = "executor_send_edit_app"
	taskUpdateActionExecution   = "execution"
	taskUpdateActionApprovement = "approvement"
	executionStartWorkAction    = "executor_start_work"
	formExecutorStartWorkAction = "form_executor_start_work"
	actionApproverSignUkep      = "sign_ukep"
)

func getApproverButtons(workNumber, mailto, blockId, login string, actions []Action, isEditable bool) []Button {
	buttons := make([]Button, 0, len(actions))
	fmt.Println("Len", len(actions))
	for i := range actions {
		if actions[i].InternalActionName == actionApproverSignUkep {
			return nil
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
		var img string

		fmt.Println(actions[i].InternalActionName)
		switch actions[i].InternalActionName {
		case "approve":
			img = "otklonButton.png"
		case "reject":
			img = "otklonButton.png"
		case "informed":
			img = "otklonButton.png"
		case "confirm":
			img = "otklonButton.png"
		case "sign":
			img = "otklonButton.png"
		case "viewed":
			img = "otklonButton.png"
		}

		buttons = append(buttons, *getButton(mailto, subject, actions[i].Title, img))
	}

	if len(buttons) == 0 {
		approveAppSubject := fmt.Sprintf(subjectTpl, blockId, "approve", workNumber, taskUpdateActionApprovement, login)
		approveAppBtn := getButton(mailto, approveAppSubject, "Согласовать", "soglButton.png")
		buttons = append(buttons, *approveAppBtn)

		rejectAppSubject := fmt.Sprintf(subjectTpl, blockId, "reject", workNumber, taskUpdateActionApprovement, login)
		rejectAppBtn := getButton(mailto, rejectAppSubject, "Отклонить", "otklonButton.png")
		buttons = append(buttons, *rejectAppBtn)
	}

	if isEditable {
		sendEditAppSubject := fmt.Sprintf(subjectTpl, blockId, "", workNumber, actionApproverSendEditApp, login)
		sendEditAppBtn := getButton(mailto, sendEditAppSubject, "Вернуть на доработку", "naDorabotkuButton.png")
		buttons = append(buttons, *sendEditAppBtn)
	}

	return buttons
}

func getExecutionButtons(workNumber, mailto, blockId, executed, rejected, login string, isEditable bool) []Button {
	executedSubject := fmt.Sprintf(subjectTpl, blockId, executed, workNumber, taskUpdateActionExecution, login)
	executedBtn := getButton(mailto, executedSubject, "Решить", "reshitButton.png")

	rejectedSubject := fmt.Sprintf(subjectTpl, blockId, rejected, workNumber, taskUpdateActionExecution, login)
	rejectedBtn := getButton(mailto, rejectedSubject, "Отклонить", "otklonButton.png")

	buttons := []Button{
		*executedBtn,
		*rejectedBtn,
	}

	if isEditable {
		sendEditAppSubject := fmt.Sprintf(subjectTpl, blockId, "", workNumber, actionExecutorSendEditApp, login)
		sendEditAppBtn := getButton(mailto, sendEditAppSubject, "Вернуть на доработку", "naDorabotkuButton.png")
		buttons = append(buttons, *sendEditAppBtn)
	}

	return buttons
}

func getLastWorksForTemplate(lastWorks []*entity.EriusTask, sdUrl string) LastWorks {
	lastWorksTemplate := make(LastWorks, 0, len(lastWorks))

	for _, task := range lastWorks {
		lastWorksTemplate = append(lastWorksTemplate, &LastWork{
			WorkNumber: task.WorkNumber,
			Name:       task.Name,
			DaysAgo:    int(math.Round(utils.GetDateUnitNumBetweenDates(task.StartedAt, time.Now(), utils.Day))),
			Link:       fmt.Sprintf(TaskUrlTemplate, sdUrl, task.WorkNumber),
		})
	}
	return lastWorksTemplate
}

type Button struct {
	Href string `json:"href"`
	Img  string `json:"img"`
}
