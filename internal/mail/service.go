package mail

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"text/template"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/mail/pkg/broker"
	"gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/abp/mail/pkg/mailclient"
)

const imageMimeTypePrefix = "image"

type Service struct {
	cli *mailclient.Client

	from *mail.Address

	SdAddress  string
	FetchEmail string
}

// nolint:gocritic // it's more comfortable to work with config as a value
func NewService(c Config) (*Service, error) {
	cfg := &broker.Config{
		Broker:   broker.Kind(c.Broker),
		Host:     c.Host,
		Port:     c.Port,
		Database: c.Database,
		Queue:    c.Queue,
	}

	client, err := mailclient.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	s := Service{
		cli: client,
		from: &mail.Address{
			Name:    c.From.Name,
			Address: c.From.Email,
		},
		SdAddress:  c.SdAddress,
		FetchEmail: c.FetchEmail,
	}
	return &s, nil
}

func (s *Service) GetApplicationLink(applicationID string) string {
	return fmt.Sprintf(TaskUrlTemplate, s.SdAddress, applicationID)
}

const htmlWrapper = `
<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.0 Transitional//EN">
<html lang="ru">
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
    <meta content="width=device-width, initial-scale=1" name="viewport">
    <meta name="x-apple-disable-message-reformatting">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
</head>
<body>%s</body></html>`

func (s *Service) SendNotification(ctx context.Context, to []string, files []email.Attachment, tmpl Template) error {
	_, span := trace.StartSpan(ctx, "SendNotification")
	defer span.End()

	msg := &email.Mail{
		From:    s.from,
		To:      make([]*mail.Address, 0, len(to)),
		Subject: tmpl.Subject,
	}

	for _, f := range files {
		if strings.HasPrefix(http.DetectContentType(f.Content), imageMimeTypePrefix) {
			f.Type = email.PlainAttachment
		}
		msg.Attachments = append(msg.Attachments, f)
	}

	for _, person := range to {
		if !regexp.MustCompile(`.+@.+`).MatchString(person) {
			continue
		}
		msg.To = append(msg.To, &mail.Address{Address: person})
	}

	temp, err := template.New("").Parse(fmt.Sprintf(htmlWrapper, tmpl.Text))
	if err != nil {
		return err
	}

	var b bytes.Buffer
	if execErr := temp.Execute(&b, tmpl.Variables); execErr != nil {
		return execErr
	}

	msg.Text = b.String()

	if sendErr := s.cli.Send(msg); sendErr != nil {
		return sendErr
	}
	return nil
}
