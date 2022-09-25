package entity

import (
	"encoding/json"
	"time"

	"github.com/pkg/errors"

	"github.com/google/uuid"
)

type Step struct {
	ID          uuid.UUID                  `json:"-"`
	Time        time.Time                  `json:"time"`
	Type        string                     `json:"type"`
	Name        string                     `json:"name"`
	State       map[string]json.RawMessage `json:"state" swaggertype:"object"`
	Storage     map[string]interface{}     `json:"storage"`
	Errors      []string                   `json:"errors"`
	Steps       []string                   `json:"steps"`
	BreakPoints []string                   `json:"-"`
	HasError    bool                       `json:"has_error"`
	Status      string                     `json:"status"`
}

type TaskSteps []*Step

func (ts *TaskSteps) IsEmpty() bool {
	return len(*ts) == 0
}

type EriusTasks struct {
	Tasks []EriusTask `json:"tasks"`
}

type EriusTasksPage struct {
	Tasks []EriusTask `json:"tasks"`
	Total int         `json:"total"`
}

type CountTasks struct {
	TotalActive   int `json:"active"`
	TotalApprover int `json:"approve"`
	TotalExecutor int `json:"execute"`
}

type EriusTask struct {
	ID            uuid.UUID              `json:"id"`
	VersionID     uuid.UUID              `json:"version_id"`
	StartedAt     time.Time              `json:"started_at"`
	LastChangedAt time.Time              `json:"last_changed_at"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Status        string                 `json:"status"`
	HumanStatus   string                 `json:"human_status"`
	Author        string                 `json:"author"`
	IsDebugMode   bool                   `json:"debug"`
	Parameters    map[string]interface{} `json:"parameters"`
	Steps         TaskSteps              `json:"steps"`
	WorkNumber    string                 `json:"work_number"`
	BlueprintID   string                 `json:"blueprint_id"`

	ActiveBlocks           map[string]struct{} `json:"active_blocks"`
	SkippedBlocks          map[string]struct{} `json:"skipped_blocks"`
	NotifiedBlocks         map[string][]string `json:"notified_blocks"`
	PrevUpdateStatusBlocks map[string]string   `json:"prev_update_status_blocks"`
}

func (et *EriusTask) IsRun() bool {
	return et.Status == "run"
}

func (et *EriusTask) IsCreated() bool {
	return et.Status == "created"
}

func (et *EriusTask) IsStopped() bool {
	return et.Status == "stopped"
}

func (et *EriusTask) IsFinished() bool {
	return et.Status == "finished"
}

func (et *EriusTask) IsError() bool {
	return et.Status == "error"
}

type GetTaskParams struct {
	Name        *string     `json:"name"`
	Created     *TimePeriod `json:"created"`
	Order       *string     `json:"order"`
	Limit       *int        `json:"limit"`
	Offset      *int        `json:"offset"`
	TaskIDs     *[]string   `json:"task_ids"`
	SelectAs    *string     `json:"select_as"`
	Archived    *bool       `json:"archived"`
	ForCarousel *bool       `json:"forCarousel"`
}

type TimePeriod struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type TaskFilter struct {
	GetTaskParams
	CurrentUser string
}

type TaskUpdateAction string

const (
	TaskUpdateActionApprovement          TaskUpdateAction = "approvement"
	TaskUpdateActionExecution            TaskUpdateAction = "execution"
	TaskUpdateActionChangeExecutor       TaskUpdateAction = "change_executor"
	TaskUpdateActionRequestExecutionInfo TaskUpdateAction = "request_execution_info"
	TaskUpdateActionExecutorStartWork    TaskUpdateAction = "executor_start_work"
	TaskUpdateActionSendEditApp          TaskUpdateAction = "send_edit_app"
	TaskUpdateActionCancelApp            TaskUpdateAction = "cancel_app"
	TaskUpdateActionRequestApproveInfo   TaskUpdateAction = "request_add_info"
)

var (
	checkTaskUpdateMap = map[TaskUpdateAction]struct{}{
		TaskUpdateActionApprovement:          {},
		TaskUpdateActionExecution:            {},
		TaskUpdateActionChangeExecutor:       {},
		TaskUpdateActionRequestExecutionInfo: {},
		TaskUpdateActionExecutorStartWork:    {},
		TaskUpdateActionSendEditApp:          {},
		TaskUpdateActionCancelApp:            {},
		TaskUpdateActionRequestApproveInfo:   {},
	}
)

type TaskUpdate struct {
	Action     TaskUpdateAction `json:"action" enums:"approvement,execution,change_executor,request_execution_info,executor_start_work"`
	Parameters json.RawMessage  `json:"parameters" swaggertype:"object"`
}

func (t *TaskUpdate) Validate() error {
	if _, ok := checkTaskUpdateMap[t.Action]; !ok {
		return errors.New("unknown action")
	}

	return nil
}

type NotifData5 struct {
	TabelniiNomer                                interface{} `json:"tabelnii_nomer" xlsx:"Табельный номер"`
	EstVoenniiBilet                              string      `json:"est_voennii_bilet" xlsx:"Есть военный билет?"`
	OtnoschenieKVoinskoiObyazannosti             string      `json:"otnoschenie_k_voinskoi_obyazannosti" xlsx:"Отношение к воинской обязанности"`
	VoinskoeZvanie                               string      `json:"voinskoe_zvanie" xlsx:"Воинское звание"`
	SostavProfil                                 string      `json:"sostav_profil" xlsx:"Состав (профиль)"`
	KategoriyaZapasa                             string      `json:"kategoriya_zapasa" xlsx:"Категория запаса"`
	Vus                                          string      `json:"_vus" xlsx:"№ ВУС"`
	PolnoeKodovoeOboznachenieVus                 string      `json:"polnoe_kodovoe_oboznachenie_vus" xlsx:"Полное кодовое обозначение ВУС"`
	GodnostKVoinskoiSluzhbe                      string      `json:"godnost_k_voinskoi_sluzhbe" xlsx:"Годность к воинской службе"`
	VoinskayaDolzhnost                           string      `json:"voinskaya_dolzhnost" xlsx:"Воинская должность"`
	VoenniiKommissariat                          string      `json:"voennii_kommissariat" xlsx:"Военный коммисариат"`
	SeriyaINomerDokumentaVoinskogoUcheta         string      `json:"seriya_i_nomer_dokumenta_voinskogo_ucheta" xlsx:"Серия и номер документа воинского учета"`
	SubektFederaiiGdePrikreplenPoVoinskomuUchetu string      `json:"subekt_federaii_gde_prikreplen_po_voinskomu_uchetu" xlsx:"Субъект Федерации (где прикреплен по воинскому учету)"`
	NalichieMobpredpisaniya                      string      `json:"nalichie_mobpredpisaniya" xlsx:"Наличие мобпредписания"`
	EstVisscheeObrazovanie                       string      `json:"est_visschee_obrazovanie" xlsx:"Есть высшее образование?"`
	KolichestvoVisschihObrazovanii               string      `json:"kolichestvo_visschih_obrazovanii" xlsx:"Количество высших образований"`
	FieldUUID130                                 string      `json:"field-uuid-130" xlsx:"Высшее образование 1"`
	FieldUUID131                                 string      `json:"field-uuid-131" xlsx:"Образовательная организация высшего образования (научная организация) 1"`
	FieldUUID132                                 string      `json:"field-uuid-132" xlsx:"Специальность, направление подготовки (наименование) 1"`
	FieldUUID133                                 string      `json:"field-uuid-133" xlsx:"Специальность, направление подготовки (код) 1"`
	FieldUUID134                                 string      `json:"field-uuid-134" xlsx:"Квалификация (наименование) 1"`
	FieldUUID135                                 string      `json:"field-uuid-135" xlsx:"Дата окончания образовательной организации высшего образования (научной организации) 1"`
	FieldUUID136                                 string      `json:"field-uuid-136" xlsx:"Серия диплома об образовании 1"`
	FieldUUID137                                 string      `json:"field-uuid-137" xlsx:"Номер диплома об образовании 1"`
	FieldUUID138                                 string      `json:"field-uuid-138" xlsx:"Дата выдачи диплома об образовании 1"`
	FieldUUID139                                 string      `json:"field-uuid-139" xlsx:"Высшее образование 2"`
	FieldUUID140                                 string      `json:"field-uuid-140" xlsx:"Образовательная организация высшего образования (научная организация) 2"`
	FieldUUID141                                 string      `json:"field-uuid-141" xlsx:"Специальность, направление подготовки (наименование) 2"`
	FieldUUID142                                 string      `json:"field-uuid-142" xlsx:"Специальность, направление подготовки (код) 2"`
	FieldUUID143                                 string      `json:"field-uuid-143" xlsx:"Квалификация (наименование) 2"`
	FieldUUID144                                 string      `json:"field-uuid-144" xlsx:"Дата окончания образовательной организации высшего образования (научной организации) 2"`
	FieldUUID145                                 string      `json:"field-uuid-145" xlsx:"Серия диплома об образовании 2"`
	FieldUUID146                                 string      `json:"field-uuid-146" xlsx:"Номер диплома об образовании 2"`
	FieldUUID147                                 string      `json:"field-uuid-147" xlsx:"Дата выдачи диплома об образовании 2"`
	FieldUUID148                                 string      `json:"field-uuid-148" xlsx:"Высшее образование 3"`
	FieldUUID149                                 string      `json:"field-uuid-149" xlsx:"Образовательная организация высшего образования (научная организация) 3"`
	FieldUUID150                                 string      `json:"field-uuid-150" xlsx:"Специальность, направление подготовки (наименование) 3"`
	FieldUUID151                                 string      `json:"field-uuid-151" xlsx:"Специальность, направление подготовки (код) 3"`
	FieldUUID152                                 string      `json:"field-uuid-152" xlsx:"Квалификация (наименование) 3"`
	FieldUUID153                                 string      `json:"field-uuid-153" xlsx:"Дата окончания образовательной организации высшего образования (научной организации) 3"`
	FieldUUID154                                 string      `json:"field-uuid-154" xlsx:"Серия диплома об образовании 3"`
	FieldUUID155                                 string      `json:"field-uuid-155" xlsx:"Номер диплома об образовании 3"`
	FieldUUID156                                 string      `json:"field-uuid-156" xlsx:"Дата выдачи диплома об образовании 3"`
	FieldUUID157                                 string      `json:"field-uuid-157" xlsx:"Высшее образование 4"`
	FieldUUID158                                 string      `json:"field-uuid-158" xlsx:"Образовательная организация высшего образования (научная организация) 4"`
	FieldUUID159                                 string      `json:"field-uuid-159" xlsx:"Специальность, направление подготовки (наименование) 4"`
	FieldUUID160                                 string      `json:"field-uuid-160" xlsx:"Специальность, направление подготовки (код) 4"`
	FieldUUID161                                 string      `json:"field-uuid-161" xlsx:"Квалификация (наименование) 4"`
	FieldUUID162                                 string      `json:"field-uuid-162" xlsx:"Дата окончания образовательной организации высшего образования (научной организации) 4"`
	FieldUUID163                                 string      `json:"field-uuid-163" xlsx:"Серия диплома об образовании 4"`
	FieldUUID164                                 string      `json:"field-uuid-164" xlsx:"Номер диплома об образовании 4"`
	FieldUUID165                                 string      `json:"field-uuid-165" xlsx:"Дата выдачи диплома об образовании 4"`
	FieldUUID166                                 string      `json:"field-uuid-166" xlsx:"Высшее образование 5"`
	FieldUUID167                                 string      `json:"field-uuid-167" xlsx:"Образовательная организация высшего образования (научная организация) 5"`
	FieldUUID168                                 string      `json:"field-uuid-168" xlsx:"Специальность, направление подготовки (наименование) 5"`
	FieldUUID169                                 string      `json:"field-uuid-169" xlsx:"Специальность, направление подготовки (код) 5"`
	FieldUUID170                                 string      `json:"field-uuid-170" xlsx:"Квалификация (наименование) 5"`
	FieldUUID171                                 string      `json:"field-uuid-171" xlsx:"Дата окончания образовательной организации высшего образования (научной организации) 5"`
	FieldUUID172                                 string      `json:"field-uuid-172" xlsx:"Серия диплома об образовании 5"`
	FieldUUID173                                 string      `json:"field-uuid-173" xlsx:"Номер диплома об образовании 5"`
	FieldUUID174                                 string      `json:"field-uuid-174" xlsx:"Дата выдачи диплома об образовании 5"`
}

type NotifData1 struct {
	TabelniiNomer                                interface{} `json:"tabelnii_nomer" xlsx:"Табельный номер"`
	EstVoenniiBilet                              string      `json:"est_voennii_bilet" xlsx:"Есть военный билет?"`
	OtnoschenieKVoinskoiObyazannosti             string      `json:"otnoschenie_k_voinskoi_obyazannosti" xlsx:"Отношение к воинской обязанности"`
	VoinskoeZvanie                               string      `json:"voinskoe_zvanie" xlsx:"Воинское звание"`
	SostavProfil                                 string      `json:"sostav_profil" xlsx:"Состав (профиль)"`
	KategoriyaZapasa                             string      `json:"kategoriya_zapasa" xlsx:"Категория запаса"`
	Vus                                          string      `json:"_vus" xlsx:"№ ВУС"`
	PolnoeKodovoeOboznachenieVus                 string      `json:"polnoe_kodovoe_oboznachenie_vus" xlsx:"Полное кодовое обозначение ВУС"`
	GodnostKVoinskoiSluzhbe                      string      `json:"godnost_k_voinskoi_sluzhbe" xlsx:"Годность к воинской службе"`
	VoinskayaDolzhnost                           string      `json:"voinskaya_dolzhnost" xlsx:"Воинская должность"`
	VoenniiKommissariat                          string      `json:"voennii_kommissariat" xlsx:"Военный коммисариат"`
	SeriyaINomerDokumentaVoinskogoUcheta         string      `json:"seriya_i_nomer_dokumenta_voinskogo_ucheta" xlsx:"Серия и номер документа воинского учета"`
	SubektFederaiiGdePrikreplenPoVoinskomuUchetu string      `json:"subekt_federaii_gde_prikreplen_po_voinskomu_uchetu" xlsx:"Субъект Федерации (где прикреплен по воинскому учету)"`
	NalichieMobpredpisaniya                      string      `json:"nalichie_mobpredpisaniya" xlsx:"Наличие мобпредписания"`
	EstVisscheeObrazovanie                       string      `json:"est_visschee_obrazovanie" xlsx:"Есть высшее образование?"`
	KolichestvoVisschihObrazovanii               string      `json:"kolichestvo_visschih_obrazovanii" xlsx:"Количество высших образований"`
	FieldUUID130                                 string      `json:"visschee_obrazovanie_1" xlsx:"Высшее образование 1"`
	FieldUUID131                                 string      `json:"obrazovatelnaya_organizaiya_visschego_obrazovaniya_nauchnaya_organizaiya_1" xlsx:"Образовательная организация высшего образования (научная организация) 1"`
	FieldUUID132                                 string      `json:"speialnost_napravlenie_podgotovki_naimenovanie_1" xlsx:"Специальность, направление подготовки (наименование) 1"`
	FieldUUID133                                 string      `json:"speialnost_napravlenie_podgotovki_kod_1" xlsx:"Специальность, направление подготовки (код) 1"`
	FieldUUID134                                 string      `json:"kvalifikaiya_naimenovanie_1" xlsx:"Квалификация (наименование) 1"`
	FieldUUID135                                 string      `json:"data_okonchaniya_obrazovatelnoi_organizaii_visschego_obrazovaniya_nauchnoi_organizaii_1" xlsx:"Дата окончания образовательной организации высшего образования (научной организации) 1"`
	FieldUUID136                                 string      `json:"seriya_diploma_ob_obrazovanii_1" xlsx:"Серия диплома об образовании 1"`
	FieldUUID137                                 string      `json:"nomer_diploma_ob_obrazovanii_1" xlsx:"Номер диплома об образовании 1"`
	FieldUUID138                                 string      `json:"data_vidachi_diploma_ob_obrazovanii_1" xlsx:"Дата выдачи диплома об образовании 1"`
}

type NotifData2 struct {
	TabelniiNomer                                interface{} `json:"tabelnii_nomer" xlsx:"Табельный номер"`
	EstVoenniiBilet                              string      `json:"est_voennii_bilet" xlsx:"Есть военный билет?"`
	OtnoschenieKVoinskoiObyazannosti             string      `json:"otnoschenie_k_voinskoi_obyazannosti" xlsx:"Отношение к воинской обязанности"`
	VoinskoeZvanie                               string      `json:"voinskoe_zvanie" xlsx:"Воинское звание"`
	SostavProfil                                 string      `json:"sostav_profil" xlsx:"Состав (профиль)"`
	KategoriyaZapasa                             string      `json:"kategoriya_zapasa" xlsx:"Категория запаса"`
	Vus                                          string      `json:"_vus" xlsx:"№ ВУС"`
	PolnoeKodovoeOboznachenieVus                 string      `json:"polnoe_kodovoe_oboznachenie_vus" xlsx:"Полное кодовое обозначение ВУС"`
	GodnostKVoinskoiSluzhbe                      string      `json:"godnost_k_voinskoi_sluzhbe" xlsx:"Годность к воинской службе"`
	VoinskayaDolzhnost                           string      `json:"voinskaya_dolzhnost" xlsx:"Воинская должность"`
	VoenniiKommissariat                          string      `json:"voennii_kommissariat" xlsx:"Военный коммисариат"`
	SeriyaINomerDokumentaVoinskogoUcheta         string      `json:"seriya_i_nomer_dokumenta_voinskogo_ucheta" xlsx:"Серия и номер документа воинского учета"`
	SubektFederaiiGdePrikreplenPoVoinskomuUchetu string      `json:"subekt_federaii_gde_prikreplen_po_voinskomu_uchetu" xlsx:"Субъект Федерации (где прикреплен по воинскому учету)"`
	NalichieMobpredpisaniya                      string      `json:"nalichie_mobpredpisaniya" xlsx:"Наличие мобпредписания"`
	EstVisscheeObrazovanie                       string      `json:"est_visschee_obrazovanie" xlsx:"Есть высшее образование?"`
	KolichestvoVisschihObrazovanii               string      `json:"kolichestvo_visschih_obrazovanii" xlsx:"Количество высших образований"`
	FieldUUID130                                 string      `json:"field-uuid-47" xlsx:"Высшее образование 1"`
	FieldUUID131                                 string      `json:"field-uuid-48" xlsx:"Образовательная организация высшего образования (научная организация) 1"`
	FieldUUID132                                 string      `json:"field-uuid-49" xlsx:"Специальность, направление подготовки (наименование) 1"`
	FieldUUID133                                 string      `json:"field-uuid-50" xlsx:"Специальность, направление подготовки (код) 1"`
	FieldUUID134                                 string      `json:"field-uuid-51" xlsx:"Квалификация (наименование) 1"`
	FieldUUID135                                 string      `json:"field-uuid-52" xlsx:"Дата окончания образовательной организации высшего образования (научной организации) 1"`
	FieldUUID136                                 string      `json:"field-uuid-53" xlsx:"Серия диплома об образовании 1"`
	FieldUUID137                                 string      `json:"field-uuid-54" xlsx:"Номер диплома об образовании 1"`
	FieldUUID138                                 string      `json:"field-uuid-55" xlsx:"Дата выдачи диплома об образовании 1"`
	FieldUUID139                                 string      `json:"visschee_obrazovanie_2" xlsx:"Высшее образование 2"`
	FieldUUID140                                 string      `json:"obrazovatelnaya_organizaiya_visschego_obrazovaniya_nauchnaya_organizaiya_2" xlsx:"Образовательная организация высшего образования (научная организация) 2"`
	FieldUUID141                                 string      `json:"speialnost_napravlenie_podgotovki_naimenovanie_2" xlsx:"Специальность, направление подготовки (наименование) 2"`
	FieldUUID142                                 string      `json:"speialnost_napravlenie_podgotovki_kod_2" xlsx:"Специальность, направление подготовки (код) 2"`
	FieldUUID143                                 string      `json:"kvalifikaiya_naimenovanie_2" xlsx:"Квалификация (наименование) 2"`
	FieldUUID144                                 string      `json:"data_okonchaniya_obrazovatelnoi_organizaii_visschego_obrazovaniya_nauchnoi_organizaii_2" xlsx:"Дата окончания образовательной организации высшего образования (научной организации) 2"`
	FieldUUID145                                 string      `json:"seriya_diploma_ob_obrazovanii_2" xlsx:"Серия диплома об образовании 2"`
	FieldUUID146                                 string      `json:"nomer_diploma_ob_obrazovanii_2" xlsx:"Номер диплома об образовании 2"`
	FieldUUID147                                 string      `json:"data_vidachi_diploma_ob_obrazovanii_2" xlsx:"Дата выдачи диплома об образовании 2"`
}

type NotifData3 struct {
	TabelniiNomer                                interface{} `json:"tabelnii_nomer" xlsx:"Табельный номер"`
	EstVoenniiBilet                              string      `json:"est_voennii_bilet" xlsx:"Есть военный билет?"`
	OtnoschenieKVoinskoiObyazannosti             string      `json:"otnoschenie_k_voinskoi_obyazannosti" xlsx:"Отношение к воинской обязанности"`
	VoinskoeZvanie                               string      `json:"voinskoe_zvanie" xlsx:"Воинское звание"`
	SostavProfil                                 string      `json:"sostav_profil" xlsx:"Состав (профиль)"`
	KategoriyaZapasa                             string      `json:"kategoriya_zapasa" xlsx:"Категория запаса"`
	Vus                                          string      `json:"_vus" xlsx:"№ ВУС"`
	PolnoeKodovoeOboznachenieVus                 string      `json:"polnoe_kodovoe_oboznachenie_vus" xlsx:"Полное кодовое обозначение ВУС"`
	GodnostKVoinskoiSluzhbe                      string      `json:"godnost_k_voinskoi_sluzhbe" xlsx:"Годность к воинской службе"`
	VoinskayaDolzhnost                           string      `json:"voinskaya_dolzhnost" xlsx:"Воинская должность"`
	VoenniiKommissariat                          string      `json:"voennii_kommissariat" xlsx:"Военный коммисариат"`
	SeriyaINomerDokumentaVoinskogoUcheta         string      `json:"seriya_i_nomer_dokumenta_voinskogo_ucheta" xlsx:"Серия и номер документа воинского учета"`
	SubektFederaiiGdePrikreplenPoVoinskomuUchetu string      `json:"subekt_federaii_gde_prikreplen_po_voinskomu_uchetu" xlsx:"Субъект Федерации (где прикреплен по воинскому учету)"`
	NalichieMobpredpisaniya                      string      `json:"nalichie_mobpredpisaniya" xlsx:"Наличие мобпредписания"`
	EstVisscheeObrazovanie                       string      `json:"est_visschee_obrazovanie" xlsx:"Есть высшее образование?"`
	KolichestvoVisschihObrazovanii               string      `json:"kolichestvo_visschih_obrazovanii" xlsx:"Количество высших образований"`
	FieldUUID130                                 string      `json:"field-uuid-66" xlsx:"Высшее образование 1"`
	FieldUUID131                                 string      `json:"field-uuid-67" xlsx:"Образовательная организация высшего образования (научная организация) 1"`
	FieldUUID132                                 string      `json:"field-uuid-68" xlsx:"Специальность, направление подготовки (наименование) 1"`
	FieldUUID133                                 string      `json:"field-uuid-69" xlsx:"Специальность, направление подготовки (код) 1"`
	FieldUUID134                                 string      `json:"field-uuid-70" xlsx:"Квалификация (наименование) 1"`
	FieldUUID135                                 string      `json:"field-uuid-71" xlsx:"Дата окончания образовательной организации высшего образования (научной организации) 1"`
	FieldUUID136                                 string      `json:"field-uuid-72" xlsx:"Серия диплома об образовании 1"`
	FieldUUID137                                 string      `json:"field-uuid-73" xlsx:"Номер диплома об образовании 1"`
	FieldUUID138                                 string      `json:"field-uuid-74" xlsx:"Дата выдачи диплома об образовании 1"`
	FieldUUID139                                 string      `json:"field-uuid-75" xlsx:"Высшее образование 2"`
	FieldUUID140                                 string      `json:"field-uuid-76" xlsx:"Образовательная организация высшего образования (научная организация) 2"`
	FieldUUID141                                 string      `json:"field-uuid-77" xlsx:"Специальность, направление подготовки (наименование) 2"`
	FieldUUID142                                 string      `json:"field-uuid-78" xlsx:"Специальность, направление подготовки (код) 2"`
	FieldUUID143                                 string      `json:"field-uuid-79" xlsx:"Квалификация (наименование) 2"`
	FieldUUID144                                 string      `json:"field-uuid-80" xlsx:"Дата окончания образовательной организации высшего образования (научной организации) 2"`
	FieldUUID145                                 string      `json:"field-uuid-81" xlsx:"Серия диплома об образовании 2"`
	FieldUUID146                                 string      `json:"field-uuid-82" xlsx:"Номер диплома об образовании 2"`
	FieldUUID147                                 string      `json:"field-uuid-83" xlsx:"Дата выдачи диплома об образовании 2"`
	FieldUUID148                                 string      `json:"field-uuid-84" xlsx:"Высшее образование 3"`
	FieldUUID149                                 string      `json:"field-uuid-85" xlsx:"Образовательная организация высшего образования (научная организация) 3"`
	FieldUUID150                                 string      `json:"field-uuid-86" xlsx:"Специальность, направление подготовки (наименование) 3"`
	FieldUUID151                                 string      `json:"field-uuid-87" xlsx:"Специальность, направление подготовки (код) 3"`
	FieldUUID152                                 string      `json:"field-uuid-88" xlsx:"Квалификация (наименование) 3"`
	FieldUUID153                                 string      `json:"field-uuid-89" xlsx:"Дата окончания образовательной организации высшего образования (научной организации) 3"`
	FieldUUID154                                 string      `json:"field-uuid-90" xlsx:"Серия диплома об образовании 3"`
	FieldUUID155                                 string      `json:"field-uuid-91" xlsx:"Номер диплома об образовании 3"`
	FieldUUID156                                 string      `json:"field-uuid-92" xlsx:"Дата выдачи диплома об образовании 3"`
}

type NotifData4 struct {
	TabelniiNomer                                interface{} `json:"tabelnii_nomer" xlsx:"Табельный номер"`
	EstVoenniiBilet                              string      `json:"est_voennii_bilet" xlsx:"Есть военный билет?"`
	OtnoschenieKVoinskoiObyazannosti             string      `json:"otnoschenie_k_voinskoi_obyazannosti" xlsx:"Отношение к воинской обязанности"`
	VoinskoeZvanie                               string      `json:"voinskoe_zvanie" xlsx:"Воинское звание"`
	SostavProfil                                 string      `json:"sostav_profil" xlsx:"Состав (профиль)"`
	KategoriyaZapasa                             string      `json:"kategoriya_zapasa" xlsx:"Категория запаса"`
	Vus                                          string      `json:"_vus" xlsx:"№ ВУС"`
	PolnoeKodovoeOboznachenieVus                 string      `json:"polnoe_kodovoe_oboznachenie_vus" xlsx:"Полное кодовое обозначение ВУС"`
	GodnostKVoinskoiSluzhbe                      string      `json:"godnost_k_voinskoi_sluzhbe" xlsx:"Годность к воинской службе"`
	VoinskayaDolzhnost                           string      `json:"voinskaya_dolzhnost" xlsx:"Воинская должность"`
	VoenniiKommissariat                          string      `json:"voennii_kommissariat" xlsx:"Военный коммисариат"`
	SeriyaINomerDokumentaVoinskogoUcheta         string      `json:"seriya_i_nomer_dokumenta_voinskogo_ucheta" xlsx:"Серия и номер документа воинского учета"`
	SubektFederaiiGdePrikreplenPoVoinskomuUchetu string      `json:"subekt_federaii_gde_prikreplen_po_voinskomu_uchetu" xlsx:"Субъект Федерации (где прикреплен по воинскому учету)"`
	NalichieMobpredpisaniya                      string      `json:"nalichie_mobpredpisaniya" xlsx:"Наличие мобпредписания"`
	EstVisscheeObrazovanie                       string      `json:"est_visschee_obrazovanie" xlsx:"Есть высшее образование?"`
	KolichestvoVisschihObrazovanii               string      `json:"kolichestvo_visschih_obrazovanii" xlsx:"Количество высших образований"`
	FieldUUID130                                 string      `json:"field-uuid-93" xlsx:"Высшее образование 1"`
	FieldUUID131                                 string      `json:"field-uuid-94" xlsx:"Образовательная организация высшего образования (научная организация) 1"`
	FieldUUID132                                 string      `json:"field-uuid-95" xlsx:"Специальность, направление подготовки (наименование) 1"`
	FieldUUID133                                 string      `json:"field-uuid-96" xlsx:"Специальность, направление подготовки (код) 1"`
	FieldUUID134                                 string      `json:"field-uuid-97" xlsx:"Квалификация (наименование) 1"`
	FieldUUID135                                 string      `json:"field-uuid-98" xlsx:"Дата окончания образовательной организации высшего образования (научной организации) 1"`
	FieldUUID136                                 string      `json:"field-uuid-99" xlsx:"Серия диплома об образовании 1"`
	FieldUUID137                                 string      `json:"field-uuid-100" xlsx:"Номер диплома об образовании 1"`
	FieldUUID138                                 string      `json:"field-uuid-101" xlsx:"Дата выдачи диплома об образовании 1"`
	FieldUUID139                                 string      `json:"field-uuid-102" xlsx:"Высшее образование 2"`
	FieldUUID140                                 string      `json:"field-uuid-103" xlsx:"Образовательная организация высшего образования (научная организация) 2"`
	FieldUUID141                                 string      `json:"field-uuid-104" xlsx:"Специальность, направление подготовки (наименование) 2"`
	FieldUUID142                                 string      `json:"field-uuid-105" xlsx:"Специальность, направление подготовки (код) 2"`
	FieldUUID143                                 string      `json:"field-uuid-106" xlsx:"Квалификация (наименование) 2"`
	FieldUUID144                                 string      `json:"field-uuid-107" xlsx:"Дата окончания образовательной организации высшего образования (научной организации) 2"`
	FieldUUID145                                 string      `json:"field-uuid-108" xlsx:"Серия диплома об образовании 2"`
	FieldUUID146                                 string      `json:"field-uuid-109" xlsx:"Номер диплома об образовании 2"`
	FieldUUID147                                 string      `json:"field-uuid-110" xlsx:"Дата выдачи диплома об образовании 2"`
	FieldUUID148                                 string      `json:"field-uuid-111" xlsx:"Высшее образование 3"`
	FieldUUID149                                 string      `json:"field-uuid-112" xlsx:"Образовательная организация высшего образования (научная организация) 3"`
	FieldUUID150                                 string      `json:"field-uuid-113" xlsx:"Специальность, направление подготовки (наименование) 3"`
	FieldUUID151                                 string      `json:"field-uuid-114" xlsx:"Специальность, направление подготовки (код) 3"`
	FieldUUID152                                 string      `json:"field-uuid-115" xlsx:"Квалификация (наименование) 3"`
	FieldUUID153                                 string      `json:"field-uuid-116" xlsx:"Дата окончания образовательной организации высшего образования (научной организации) 3"`
	FieldUUID154                                 string      `json:"field-uuid-117" xlsx:"Серия диплома об образовании 3"`
	FieldUUID155                                 string      `json:"field-uuid-118" xlsx:"Номер диплома об образовании 3"`
	FieldUUID156                                 string      `json:"field-uuid-119" xlsx:"Дата выдачи диплома об образовании 3"`
	FieldUUID157                                 string      `json:"field-uuid-120" xlsx:"Высшее образование 4"`
	FieldUUID158                                 string      `json:"field-uuid-121" xlsx:"Образовательная организация высшего образования (научная организация) 4"`
	FieldUUID159                                 string      `json:"field-uuid-122" xlsx:"Специальность, направление подготовки (наименование) 4"`
	FieldUUID160                                 string      `json:"field-uuid-123" xlsx:"Специальность, направление подготовки (код) 4"`
	FieldUUID161                                 string      `json:"field-uuid-125" xlsx:"Квалификация (наименование) 4"`
	FieldUUID162                                 string      `json:"field-uuid-126" xlsx:"Дата окончания образовательной организации высшего образования (научной организации) 4"`
	FieldUUID163                                 string      `json:"field-uuid-127" xlsx:"Серия диплома об образовании 4"`
	FieldUUID164                                 string      `json:"field-uuid-128" xlsx:"Номер диплома об образовании 4"`
	FieldUUID165                                 string      `json:"field-uuid-129" xlsx:"Дата выдачи диплома об образовании 4"`
}

type NeededNotif struct {
	Initiator   string
	Recipient   string
	WorkNum     string
	Description interface{}
	Status      string
}
