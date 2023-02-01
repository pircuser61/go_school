package fetcher

import (
	c "context"
	"fmt"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/human-tasks/pkg/utils/tracer"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail/imap"

	"go.opencensus.io/trace"
)

const (
	stepName   = "step_name"
	decision   = "decision"
	workNumber = "work_number"
	actionName = "action_name"

	fieldsDelimiter         string = "|"
	fieldsKeyValueDelimiter string = "="
)

type service struct {
	incomingClient imap.IncomingClient
}

func NewService(cfg Config) (Service, error) {
	imapCli, err := imap.NewImapClient(&imap.ClientConfig{
		ImapConnection: cfg.ImapConnection,
		ImapUserName:   cfg.ImapUserName,
		ImapPassword:   cfg.ImapPassword,
		ImapMailBox:    cfg.ImapMailBox,
	})
	if err != nil {
		return nil, err
	}

	return &service{
		incomingClient: imapCli,
	}, nil
}

func (s *service) FetchEmails(ctx c.Context) (actions []ParsedEmail, err error) {
	ctx, span := trace.StartSpan(ctx, "mail.fetcher.FetchEmails")
	defer func() { tracer.End(span, err) }()

	log := logger.GetLogger(ctx)

	messages, section, err := s.incomingClient.SelectUnread(ctx)
	if err != nil {
		return nil, err
	}

	if messages == nil || section == nil {
		return nil, nil
	}

	actions = make([]ParsedEmail, 0)

	for msg := range messages {
		action, errProcess := s.processMessage(ctx, msg, section)
		if errProcess != nil {
			log.Error(fmt.Sprintf("processMessage err: %s", errProcess.Error()))
			continue
		}

		if action == nil {
			log.Warning("processMessage action is nil")
			continue
		}

		actions = append(actions, *action)
	}
	return actions, nil
}

func (s *service) CloseIMAP(ctx c.Context) {
	s.incomingClient.Close(ctx)
}
