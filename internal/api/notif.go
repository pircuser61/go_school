package api

import (
	"context"
	"fmt"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"reflect"
	"strconv"
	"time"

	"go.opencensus.io/trace"

	"github.com/xuri/excelize/v2"

	"gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
)

func (ae *APIEnv) MakeAndSendNotifSheduler(ctx context.Context) {
	log := logger.GetLogger(ctx)

	n, err := ae.makeAndSendNotif(ctx)
	if err != nil {
		log.Error(err)
	} else {
		log.Info("make and send notif success count applications = ", n)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Hour):
			n, err = ae.makeAndSendNotif(ctx)
			if err != nil {
				log.Error(err)
			}

			log.Info("make and send notif success count applications = ", n)
		}
	}
}

func (ae *APIEnv) makeAndSendNotif(ctx context.Context) (int, error) {
	ctxSh, span := trace.StartSpan(ctx, "sheduler make and send notif")
	defer span.End()

	data, err := ae.DB.GetNotifData(ctxSh)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}

	f := excelize.NewFile()
	streamingWriter, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return 0, err
	}
	titles := []interface{}{"Номер заявки", "Инициатор", "Получатель", "Статус"}
	fc := reflect.TypeOf(entity.NotifData5{}).NumField()
	for i := 0; i < fc; i++ {
		titles = append(titles, reflect.TypeOf(entity.NotifData5{}).Field(i).Tag.Get("xlsx"))
	}
	records := [][]interface{}{titles}
	peopleMap := make(map[string]string)

	statusMapping := map[pipeline.TaskHumanStatus]string{
		pipeline.StatusDone:              "Готово",
		pipeline.StatusExecutionRejected: "Отклонено",
		pipeline.StatusNew:               "Новый",
		pipeline.StatusWait:              "Ожидание",
		pipeline.StatusExecution:         "Обработка",
	}

	for _, item := range data {
		initName, recName := "", ""
		fullname, ok := peopleMap[item.Initiator]
		if ok {
			initName = fullname
		} else {
			var user people.SSOUser
			user, err = ae.People.GetUser(ctxSh, item.Initiator)
			if err != nil {
				return 0, err
			}
			typed, err := user.ToSSOUserTyped()
			if err != nil {
				return 0, err
			}
			initName = typed.Attributes.FullName
		}
		fullname, ok = peopleMap[item.Recipient]
		if ok {
			recName = fullname
		} else {
			if item.Recipient == "" {
				continue
			}
			var user people.SSOUser
			user, err = ae.People.GetUser(ctxSh, item.Recipient)
			if err != nil {
				return 0, err
			}
			typed, err := user.ToSSOUserTyped()
			if err != nil {
				return 0, err
			}
			recName = typed.Attributes.FullName
		}
		newRow := []interface{}{
			item.WorkNum,
			fmt.Sprintf("%s (%s)", item.Initiator, initName),
			fmt.Sprintf("%s (%s)", item.Recipient, recName),
			statusMapping[pipeline.TaskHumanStatus(item.Status)],
		}
		for i := 0; i < reflect.ValueOf(item.Description).NumField(); i++ {
			if reflect.ValueOf(item.Description).Field(i).Kind() == reflect.Interface {
				newRow = append(newRow, fmt.Sprintf("%v", reflect.ValueOf(item.Description).Field(i).Interface()))
			} else {
				newRow = append(newRow, reflect.ValueOf(item.Description).Field(i).String())
			}

		}
		records = append(records, newRow)
	}

	for i := range records {
		err = streamingWriter.SetRow("A"+strconv.Itoa(i+1), records[i])
		if err != nil {
			return 0, err
		}
	}

	if err = streamingWriter.Flush(); err != nil {
		return 0, err
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return 0, err
	}

	return len(records) - 1, ae.Mail.SendNotification(ctxSh, []string{
		"yvyegorova@mts.ru",
		"ampetr13@mts.ru",
		"lslaptev@mts.ru",
		"Maksim.Kiselev@mts.ru"}, []email.Attachment{
		{
			Name:    "applications.xlsx",
			Content: buf.Bytes(),
			Type:    email.PlainAttachment,
		},
	}, mail.NewMakeAndSendNotifTemplate())
}
