package api

import (
	c "context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/iancoleman/orderedmap"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

type runVersionsDTO struct {
	WorkNumber        string
	Description       string
	PipelineID        string
	AttachmentFields  []string
	Keys              map[string]string
	IsTestApplication bool
	CustomTitle       string
	Authorization     string
	RequestID         string
	ClientID          string

	requestInfo *metrics.RequestInfo

	ApplicationBody orderedmap.OrderedMap
}

func (ae *Env) WorkRunTaskHandler(ctx c.Context, jobs <-chan kafka.TimedRunTaskMessage) {
	log := ae.Log.WithField("funcName", "WorkRunTaskHandler")

	for job := range jobs {
		ae.RunTaskHandler(ctx, job.Msg) //nolint:errcheck // Все ошибки уже обрабатываются внутри

		if err := ae.Kafka.DelRunTaskMsg(ctx, strconv.Itoa(int(job.TimeNow.Unix()))); err != nil {
			log.WithField("workNumber", job.Msg.WorkNumber).WithError(err).Error("cannot delete function message from redis")
		}
	}
}

//nolint:all //its ok here
func (ae *Env) RunTaskHandler(ctx c.Context, message kafka.RunTaskMessage) error {
	start := time.Now()

	ctx, span := trace.StartSpan(ctx, "RunTaskHandler")

	requestInfo := metrics.NewPostRequestInfo(runByKafka)

	log := ae.Log.WithField("funcName", "RunTaskHandler").
		WithField("workNumber", message.WorkNumber).
		WithField("method", "kafka")

	defer func() {
		span.End()

		if r := recover(); r != nil {
			log.WithField("funcName", "recover").Error(r)
			requestInfo.Status = PipelineExecutionError.Status()
		}

		requestInfo.Duration = time.Since(start)
		requestInfo.PipelineID = message.PipelineID
		requestInfo.WorkNumber = message.WorkNumber

		ae.Metrics.RequestsIncrease(requestInfo)
	}()

	ctx = logger.WithLogger(ctx, log)
	ctx = c.WithValue(ctx, script.RequestID{}, message.RequestID)

	messageTmp, err := json.Marshal(message)
	if err != nil {
		log.WithError(err).Error("error marshaling message from kafka")
	}

	messageString := string(messageTmp)

	log.WithField("body", messageString).Info("start handle message from kafka")

	if message.Username != "" {
		var u people.SSOUser
		u, err = ae.People.GetUser(ctx, strings.ToLower(message.Username), true)
		if err != nil {
			log.WithField("username", message.Username).Error(err)

			return err
		}

		var ui *sso.UserInfo
		ui, err = u.ToUserinfo()
		if err != nil {
			log.WithField("username", message.Username).Error(err)

			return err
		}

		ctx = user.SetUserInfoToCtx(ctx, ui)
	}

	if message.XAsOther != "" {
		var u people.SSOUser
		u, err = ae.People.GetUser(ctx, strings.ToLower(message.XAsOther), true)
		if err != nil {
			log.WithField("XAsOther", message.XAsOther).Error(err)

			goto skipXAsOther
		}

		var ui *sso.UserInfo
		ui, err = u.ToUserinfo()
		if err != nil {
			log.WithField("XAsOther", message.XAsOther).Error(err)

			goto skipXAsOther
		}

		ctx = user.SetAsOtherUserInfoToCtx(ctx, ui)
	}

skipXAsOther:

	run := &runVersionsDTO{
		WorkNumber:        message.WorkNumber,
		Description:       message.Description,
		PipelineID:        message.PipelineID,
		AttachmentFields:  message.AttachmentFields,
		Keys:              message.Keys,
		IsTestApplication: message.IsTestApplication,
		CustomTitle:       message.CustomTitle,
		ApplicationBody:   message.ApplicationBody,
		ClientID:          message.ClientID,
		RequestID:         message.RequestID,

		requestInfo: requestInfo,
	}

	err = ae.runVersion(ctx, log, run)
	if err != nil {
		log.Error(err)
		requestInfo.Status = PipelineExecutionError.Status()

		return nil
	}

	log.Info("message from kafka successfully handled")

	return nil
}
