package api

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"
	"gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"go.opencensus.io/trace"
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

	dataWithDouble, err := ae.DB.GetNotifData(ctxSh)
	if err != nil {
		return 0, err
	}
	data := entity.CheckDoubleNeededNotify(dataWithDouble)

	f := excelize.NewFile()
	streamingWriter, err := f.NewStreamWriter("Sheet1")
	records := [][]interface{}{{"Номер заявки", "Инициатор", "Получатель", "Поля"}}
	peopleMap := make(map[string]string)

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
		records = append(records, []interface{}{
			item.WorkNum,
			fmt.Sprintf("%s (%s)", item.Initiator, initName),
			fmt.Sprintf("%s (%s)", item.Recipient, recName),
			item.Description,
		})
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

	return len(records) - 1, ae.Mail.SendNotification(ctxSh, []string{"lslaptev@mts.ru", "aamonak5@mts.ru", "snkosya1@mts.ru"}, []email.Attachment{
		{
			Name:    "applications.xlsx",
			Content: buf.Bytes(),
			Type:    email.PlainAttachment,
		},
	}, mail.NewMakeAndSendNotifTemplate())
}
