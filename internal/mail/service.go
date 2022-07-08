package mail

import (
	"bytes"
	"context"
	"net/mail"
	"text/template"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/mail/pkg/broker"
	"gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/abp/mail/pkg/mailclient"
)

type Service struct {
	cli *mailclient.Client

	from *mail.Address

	SdAddress string
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
		SdAddress: c.SdAddress,
	}
	return &s, nil
}

func (s *Service) SendNotification(ctx context.Context, to []string, tmpl Template) error {
	_, span := trace.StartSpan(ctx, "SendNotification")
	defer span.End()

	msg := &email.Mail{
		From:    s.from,
		To:      make([]*mail.Address, 0, len(to)),
		Subject: tmpl.Subject,
	}

	for _, person := range to {
		msg.To = append(msg.To, &mail.Address{Address: person})
	}

	temp, err := template.New("").Parse(tmpl.Text)
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
