package fetcher

import (
	c "context"
	"fmt"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/human-tasks/pkg/utils/tracer"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail/imap"

	"go.opencensus.io/trace"
)

const fieldsDelimiter string = "|"
const fieldsKeyValueDelimiter string = "="

const (
	subjectFieldTemplateCode string = "template_code"
	subjectFieldNodeId       string = "node_id"
	subjectFieldProjectCode  string = "project_code"
	subjectFieldDecision     string = "decision"
)

type service struct {
	incomingClient imap.IncomingClient
}

func NewService(cfg *Config) (*service, error) {
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

func (s *service) FetchEmails(ctx c.Context) (err error) {
	ctx, span := trace.StartSpan(ctx, "mail.fetcher.FetchEmails")
	defer func() { tracer.End(span, err) }()

	log := logger.GetLogger(ctx)

	messages, section, err := s.incomingClient.SelectUnread(ctx)
	if err != nil {
		return err
	}

	if messages == nil || section == nil {
		return nil
	}
	for msg := range messages {
		errProcess := s.processMessage(ctx, msg, section)
		if errProcess != nil {
			log.Error(fmt.Sprintf("processMessage err: %s", errProcess.Error()))
			continue
		}
	}
	return nil
}

func (s *service) CloseIMAP(ctx c.Context) {
	s.incomingClient.Close(ctx)
}
