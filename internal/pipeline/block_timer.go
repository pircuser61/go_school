package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/scheduler"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"golang.org/x/exp/slices"
)

type TimerData struct {
	Duration time.Duration `json:"duration"`
	Started  bool          `json:"started"`
	Expired  bool          `json:"expired"`
}

type TimerBlock struct {
	Name      string
	ShortName string
	Title     string
	Input     map[string]string
	Output    map[string]string
	Sockets   []script.Socket
	State     *TimerData

	RunContext *BlockRunContext

	expectedEvents      map[string]struct{}
	happenedEvents      []entity.NodeEvent
	happenedKafkaEvents []entity.NodeKafkaEvent
}

func (gb *TimerBlock) CurrentExecutorData() CurrentExecutorData {
	return CurrentExecutorData{}
}

func (gb *TimerBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *TimerBlock) GetNewKafkaEvents() []entity.NodeKafkaEvent {
	return gb.happenedKafkaEvents
}

type TimerParams struct {
	Delay    string `json:"delay"`
	DatePath string `json:"date_path"`
	Coef     int    `json:"coef"`
	WorkDay  int    `json:"workDay"`
	Duration string `json:"duration"`
}

func (gb *TimerBlock) Members() []Member {
	return nil
}

func (gb *TimerBlock) Deadlines(_ context.Context) ([]Deadline, error) {
	return []Deadline{}, nil
}

func (gb *TimerBlock) GetStatus() Status {
	if gb.State.Expired {
		return StatusFinished
	}

	return StatusIdle
}

func (gb *TimerBlock) GetTaskHumanStatus() (status TaskHumanStatus, comment, action string) {
	if gb.State.Expired {
		return StatusDone, "", ""
	}

	return StatusExecution, "", ""
}

func (gb *TimerBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}

	return nexts, true
}

func (gb *TimerBlock) GetState() interface{} {
	return gb.State
}

func (gb *TimerBlock) Update(ctx context.Context) (interface{}, error) {
	if gb.RunContext.UpdateData != nil && gb.RunContext.UpdateData.Action == string(entity.TaskUpdateActionReload) {
		return nil, nil
	}

	if gb.State.Started {
		if err := gb.checkUserIsServiceAccount(ctx); err != nil {
			return nil, err
		}
	}

	if gb.State.Expired {
		return nil, errors.New("timer has already expired")
	}

	if gb.State.Started {
		gb.State.Expired = true
	} else {
		if errStart := gb.startTimer(ctx); errStart != nil {
			return nil, errStart
		}

		gb.State.Started = true
	}

	stateBytes, err := json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)

	if gb.State.Expired {
		if _, ok := gb.expectedEvents[eventEnd]; ok {
			status, _, _ := gb.GetTaskHumanStatus()

			event, eventErr := gb.RunContext.MakeNodeEndEvent(ctx, MakeNodeEndEventArgs{
				NodeName:      gb.Name,
				NodeShortName: gb.ShortName,
				HumanStatus:   status,
				NodeStatus:    gb.GetStatus(),
			})
			if eventErr != nil {
				return nil, eventErr
			}

			gb.happenedEvents = append(gb.happenedEvents, event)
		}
	}

	return nil, nil
}

func (gb *TimerBlock) startTimer(ctx context.Context) error {
	_, err := gb.RunContext.Services.Scheduler.CreateTask(ctx, &scheduler.CreateTask{
		WorkNumber:  gb.RunContext.WorkNumber,
		WorkID:      gb.RunContext.TaskID.String(),
		ActionName:  string(entity.TaskUpdateActionFinishTimer),
		StepName:    gb.Name,
		WaitSeconds: int(gb.State.Duration.Seconds()),
	})

	return err
}

func (gb *TimerBlock) checkUserIsServiceAccount(ctx context.Context) error {
	currentUser, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		return err
	}

	if currentUser.Username != ServiceAccountDev &&
		currentUser.Username != ServiceAccountStage &&
		currentUser.Username != ServiceAccount {
		err = fmt.Errorf("user %s is not service account", currentUser.Username)

		return err
	}

	return nil
}

func (gb *TimerBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockTimerID,
		BlockType: script.TypeGo,
		Title:     BlockTimerTitle,
		Inputs:    nil,
		Outputs:   nil,
		Params: &script.FunctionParams{
			Type: BlockTimerID,
			Params: TimerParams{
				Delay:    "0d",
				DatePath: "",
				Coef:     0,
				WorkDay:  0,
				Duration: "0h",
			},
		},
		Sockets: []script.Socket{script.DefaultSocket},
	}
}

func (gb *TimerBlock) BlockAttachments() (ids []string) {
	return ids
}

func (gb *TimerBlock) UpdateManual() bool {
	return false
}

// nolint:dupl // another block
func createTimerBlock(
	ctx context.Context,
	name string,
	ef *entity.EriusFunc,
	runCtx *BlockRunContext,
	expectedEvents map[string]struct{},
) (*TimerBlock, bool, error) {
	b := &TimerBlock{
		Name:       name,
		ShortName:  ef.ShortTitle,
		Title:      ef.Title,
		Input:      map[string]string{},
		Output:     map[string]string{},
		Sockets:    entity.ConvertSocket(ef.Sockets),
		RunContext: runCtx,

		expectedEvents: expectedEvents,
		happenedEvents: make([]entity.NodeEvent, 0),
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	if ef.Output != nil {
		//nolint:gocritic //в этом проекте не принято использовать поинтеры в коллекциях
		for propertyName, v := range ef.Output.Properties {
			if v.Global == "" {
				continue
			}

			b.Output[propertyName] = v.Global
		}
	}

	rawState, blockExists := runCtx.VarStore.State[name]

	reEntry := blockExists && runCtx.UpdateData == nil
	if blockExists && !reEntry {
		if err := b.loadState(rawState); err != nil {
			return nil, false, err
		}
	} else {
		err := b.createExpectedEvents(ctx, runCtx, name, ef)
		if err != nil {
			return nil, false, err
		}
	}

	return b, reEntry, nil
}

func (gb *TimerBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

//nolint:dupl //another block
func (gb *TimerBlock) createExpectedEvents(
	ctx context.Context,
	runCtx *BlockRunContext,
	name string,
	ef *entity.EriusFunc,
) error {
	if err := gb.createState(ef); err != nil {
		return err
	}

	gb.RunContext.VarStore.AddStep(gb.Name)

	if _, ok := gb.expectedEvents[eventStart]; ok {
		status, _, _ := gb.GetTaskHumanStatus()

		event, err := runCtx.MakeNodeStartEvent(ctx, MakeNodeStartEventArgs{
			NodeName:      name,
			NodeShortName: ef.ShortTitle,
			HumanStatus:   status,
			NodeStatus:    gb.GetStatus(),
		})
		if err != nil {
			return err
		}

		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	return nil
}

//nolint:dupl,gocyclo //its not duplicate
func (gb *TimerBlock) createState(ef *entity.EriusFunc) error {
	var params TimerParams

	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get timer parameters")
	}

	var duration time.Duration

	if params.Duration != "" && !strings.Contains(params.Duration, "d") {
		params.Duration = "0d" + params.Duration
	}

	if params.Duration != "" {
		var day int

		dur := strings.Split(params.Duration, "d")

		day, err = strconv.Atoi(dur[0])
		if err != nil {
			return errors.Wrap(err, "can not convert timer day duration")
		}

		dayInHours := fmt.Sprintf("%dh", day*24)

		duration, err = time.ParseDuration(dur[1])
		if err != nil {
			return errors.Wrap(err, "can not parse timer duration")
		}

		var duration2 time.Duration

		duration2, err = time.ParseDuration(dayInHours)
		if err != nil {
			return errors.Wrap(err, "can not parse timer days duration")
		}

		duration += duration2

		if duration <= 0 {
			return errors.New("delay time is not set for the timer")
		}

		gb.State = &TimerData{Duration: duration}

		return nil
	}

	variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
	if grabStorageErr != nil {
		return grabStorageErr
	}

	dateInt := getVariable(variableStorage, params.DatePath)
	if dateInt == nil {
		gb.State = &TimerData{Duration: 1 * time.Millisecond}

		return nil
	}

	dateWithHour := fmt.Sprintf("%v", dateInt)

	date := strings.Split(dateWithHour, " ")

	dateObj, err := time.Parse("02.01.2006", date[0])
	if err != nil {
		gb.State = &TimerData{Duration: 1 * time.Millisecond}

		return nil
	}

	targetTime := time.Date(dateObj.Year(), dateObj.Month(), dateObj.Day(), 5, 0, 0, 0, dateObj.Location())

	var (
		slaInfoPtr    *sla.Info
		getSLAInfoErr error
		calendarDays  *hrgate.CalendarDays
		weekends      []time.Weekday
	)

	useCalendar := false
	if params.WorkDay != 0 {
		useCalendar = true

		slaInfoPtr, getSLAInfoErr = gb.RunContext.Services.SLAService.GetSLAInfoPtr(
			context.Background(),
			sla.InfoDTO{
				TaskCompletionIntervals: []entity.TaskCompletionInterval{
					{
						StartedAt:  gb.RunContext.CurrBlockStartTime,
						FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
					},
				},
				WorkType: sla.WorkHourType("8/5"),
			},
		)
		if getSLAInfoErr != nil {
			return errors.Wrap(err, "can not prepare slaInfo")
		}

		calendarDays = slaInfoPtr.GetCalendarDays()

		weekends = slaInfoPtr.GetWeekends()
	}

	if useCalendar {
		targetTime = skipNotWorkingDays(targetTime, calendarDays, weekends, params.WorkDay)
	}

	daysToAdd := 0

	if params.Delay != "" {
		daysToAdd, err = strconv.Atoi(strings.TrimSuffix(params.Delay, "d"))
		if err != nil {
			return errors.New("wrong format of delay days")
		}
	}

	for ; daysToAdd != 0; daysToAdd-- {
		targetTime = targetTime.Add(time.Duration(params.Coef) * 24 * time.Hour)

		if useCalendar {
			targetTime = skipNotWorkingDays(targetTime, calendarDays, weekends, params.Coef)
		}
	}

	currentDate := time.Now()

	duration = targetTime.Sub(currentDate)
	if duration <= 0 {
		duration = 1 * time.Millisecond
	}

	year := 365 * 24 * time.Hour
	if duration > year {
		duration = 365 * 24 * time.Hour
	}

	gb.State = &TimerData{Duration: duration}

	return nil
}

func (gb *TimerBlock) UpdateStateUsingOutput(context.Context, []byte) (state map[string]interface{}, err error) {
	return nil, nil
}

func (gb *TimerBlock) UpdateOutputUsingState(context.Context) (output map[string]interface{}, err error) {
	return nil, nil
}

func notWorkingHours(t time.Time, calendarDays *hrgate.CalendarDays, weekends []time.Weekday) bool {
	workDayType, found := calendarDays.GetDayType(t)

	if !found && slices.Contains(weekends, t.Weekday()) {
		return true
	}

	if found && (workDayType == hrgate.CalendarDayTypeWeekend || workDayType == hrgate.CalendarDayTypeHoliday) {
		return true
	}

	return false
}

func skipNotWorkingDays(
	targetTime time.Time,
	calendarDays *hrgate.CalendarDays,
	weekends []time.Weekday,
	delay int,
) time.Time {
	for {
		if !notWorkingHours(targetTime, calendarDays, weekends) {
			break
		}

		targetTime = targetTime.AddDate(0, 0, delay)
	}

	return targetTime
}
