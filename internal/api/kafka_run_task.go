package api

import (
	c "context"
	"encoding/json"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"strings"

	"github.com/iancoleman/orderedmap"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
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
		}

		var ui *sso.UserInfo
		ui, err = u.ToUserinfo()
		if err != nil {
			log.WithField("username", message.Username).Error(err)
		}

		ctx = user.SetUserInfoToCtx(ctx, ui)
	}

	if message.XasOther != "" {
		var u people.SSOUser
		u, err = ae.People.GetUser(ctx, strings.ToLower(message.XasOther))
		if err != nil {
			log.WithField("xAsOther", message.XasOther).Error(err)
		}

		var ui *sso.UserInfo
		ui, err = u.ToUserinfo()
		if err != nil {
			log.WithField("xAsOther", message.XasOther).Error(err)
		}

		ctx = user.SetAsOtherUserInfoToCtx(ctx, ui)
	}

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
	}

	_, err = ae.runVersion(ctx, log, run)
	if err != nil {
		log.Error(err)

		return nil
	}

	log.Info("message from kafka successfully handled")

	return nil
}
