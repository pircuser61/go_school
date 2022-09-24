package api

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
)

func  (ae *APIEnv)  makeAndSandNotif() error {
	data, err := ae.DB.GetNotifData(context.Background())
	if err != nil {
		return err
	}
	people := make(map[string]string)
	records := [][]string{
		{"Номер заявки", "Инициатор", "Получатель", "Поля"},
	}
	for _, item := range data  {
		initName, recName := "", ""
		fullname, ok := people[item.Initiator]
		if ok {
			initName = fullname
		} else {
			user, err := ae.People.GetUser(context.Background(), item.Initiator)
			if err != nil {
				return err
			}
			typed, err := user.ToSSOUserTyped()
			if err != nil {
				return err
			}
			initName = typed.Attributes.FullName
		}
		fullname, ok = people[item.Recipient]
		if ok {
			recName = fullname
		} else {
			user, err := ae.People.GetUser(context.Background(), item.Recipient)
			if err != nil {
				return err
			}
			typed, err := user.ToSSOUserTyped()
			if err != nil {
				return err
			}
			recName = typed.Attributes.FullName
		}
		records = append(records, []string{
			item.WorkNum,
			fmt.Sprintf("%s (%s)", item.Initiator, initName),
			fmt.Sprintf("%s (%s)", item.Recipient, recName),
			item.Description,
		})
	}
	f, err := os.Create("temp.csv")
	defer f.Close()

	if err != nil {
		return err
	}

	w := csv.NewWriter(f)
	err = w.WriteAll(records) // calls Flush internally

	if err != nil {
		return err
	}

	ae.Mail.SendNotification(context.Background(), []string{"snkosya1@mts.ru"}, atts, )
}
