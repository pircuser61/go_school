package api

import (
	c "context"
	"encoding/json"
	"golang.org/x/net/context"
	"strings"

	"github.com/iancoleman/orderedmap"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
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

	ApplicationBody orderedmap.OrderedMap
}

func (ae *Env) WorkRunTaskHandler(ctx context.Context, jobs <-chan kafka.RunTaskMessage) {
	for job := range jobs {
		ae.RunTaskHandler(ctx, job) //nolint:errcheck // Все ошибки уже обрабатываются внутри
	}
}

//nolint:all //its ok here
func (ae *Env) RunTaskHandler(ctx c.Context, message kafka.RunTaskMessage) error {
	ctx, span := trace.StartSpan(ctx, "RunTaskHandler")
	defer span.End()

	log := ae.Log.WithField("funcName", "RunTaskHandler").
		WithField("workNumber", message.WorkNumber).
		WithField("method", "kafka")

	ctx = logger.WithLogger(ctx, log)

	messageTmp, err := json.Marshal(message)
	if err != nil {
		log.WithError(err).Error("error marshaling message from kafka")
	}

	messageString := string(messageTmp)

	log.WithField("body", messageString).Info("start handle message from kafka")

	defer func() {
		if r := recover(); r != nil {
			log.WithField("funcName", "recover").Error(r)
		}
	}()

	if message.Username != "" {
		var u people.SSOUser
		u, err = ae.People.GetUser(ctx, strings.ToLower(message.Username))
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
		u, err = ae.People.GetUser(ctx, strings.ToLower(message.XAsOther))
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
	}

	_, err = ae.runVersion(ctx, log, run)
	if err != nil {
		log.Error(err)

		return nil
	}

	log.Info("message from kafka successfully handled")

	return nil
}
