package api

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"
	"gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"go.opencensus.io/trace"
)

func (ae *APIEnv) MakeAndSendNotifSheduler(ctx context.Context) {
	ctxSh, span := trace.StartSpan(ctx, "sheduler make and send notif")
	defer span.End()

	log := logger.GetLogger(ctxSh)

	err := ae.makeAndSendNotif()
	if err != nil {
		log.Error(err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Hour):
			err = ae.makeAndSendNotif()
			if err != nil {
				log.Error(err)
			}
		}
	}
}

func (ae *APIEnv) makeAndSendNotif() error {
	data, err := ae.DB.GetNotifData(context.Background())
	if err != nil {
		return err
	}

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
			user, err = ae.People.GetUser(context.Background(), item.Initiator)
			if err != nil {
				return err
			}
			typed, err := user.ToSSOUserTyped()
			if err != nil {
				return err
			}
			initName = typed.Attributes.FullName
		}
		fullname, ok = peopleMap[item.Recipient]
		if ok {
			recName = fullname
		} else {
			var user people.SSOUser
			user, err = ae.People.GetUser(context.Background(), item.Recipient)
			if err != nil {
				return err
			}
			typed, err := user.ToSSOUserTyped()
			if err != nil {
				return err
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
			return err
		}
	}

	if err = streamingWriter.Flush(); err != nil {
		return err
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return err
	}

	err = ae.Mail.SendNotification(context.Background(), []string{"lslaptev@mts.ru", "aamonak5@mts.ru", "snkosya1@mts.ru"}, []email.Attachment{
		{
			Name:    "applications.xlsx",
			Content: buf.Bytes(),
			Type:    email.PlainAttachment,
		},
	}, mail.NewEmptyTemplate())
	if err != nil {
		return err
	}

	return ae.DB.UpdateCacheTime(context.Background())
}
