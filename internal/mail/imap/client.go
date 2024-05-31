package imap

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"sync"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/prodboard/infra/tracer"
)

const reconnectRetryCount = 3

type Client struct {
	imapClient     *client.Client
	imapConnection string
	imapUserName   string
	imapPassword   string
	imapMailBox    string
	criteria       *imap.SearchCriteria
	once           sync.Once
}

type ClientConfig struct {
	ImapConnection string
	ImapUserName   string
	ImapPassword   string
	ImapMailBox    string
}

func NewImapClient(cfg *ClientConfig) (*Client, error) {
	c := &Client{
		imapConnection: cfg.ImapConnection,
		imapUserName:   cfg.ImapUserName,
		imapPassword:   os.Getenv(cfg.ImapPassword),
		imapMailBox:    cfg.ImapMailBox,
	}

	err := c.connect()
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (s *Client) connect() error {
	cfg := &tls.Config{
		CipherSuites: []uint16{49195, 49199, 49196, 49200, 52393, 52392, 49161, 49171, 49162, 49172, 156, 157, 47, 53, 49170, 10},
		MinVersion:   tls.VersionTLS12,
	}

	c, err := client.DialTLS(s.imapConnection, cfg)
	if err != nil {
		return errors.Wrap(err, "create IMAP client")
	}

	if err = c.Login(s.imapUserName, s.imapPassword); err != nil {
		return errors.Wrap(err, "IMAP client login")
	}

	s.imapClient = c

	return nil
}

func (s *Client) Close(ctx context.Context) {
	log := logger.GetLogger(ctx)

	errLogOut := s.imapClient.Logout()
	if errLogOut != nil {
		log.Error("error on imap client logout", "error", errLogOut)
	}

	errClose := s.imapClient.Close()
	if errClose != nil {
		log.Error("error on imap client closing", "error", errClose)
	}

	<-s.imapClient.LoggedOut()
}

func (s *Client) Reconnect(ctx context.Context) error {
	s.Close(ctx)

	errInit := s.connect()
	if errInit != nil {
		return errors.Wrap(errInit, "error while re-creating the imap connection")
	}

	return nil
}

func (s *Client) Check(ctx context.Context) (err error) {
	for retryCount := 0; retryCount < reconnectRetryCount; retryCount++ {
		err = s.imapClient.Check()
		if err == nil {
			return nil
		}

		_ = s.Reconnect(ctx)
	}

	return errors.Wrap(err, "error while checking the imap connection")
}

func (s *Client) Select(ctx context.Context, name string, readOnly bool) (mailBox *imap.MailboxStatus, err error) {
	for retryCount := 0; retryCount < reconnectRetryCount; retryCount++ {
		mailBox, err = s.imapClient.Select(name, readOnly)
		if err == nil {
			return mailBox, nil
		}

		_ = s.Reconnect(ctx)
	}

	return nil, errors.Wrap(err, "error while selecting the imap mailbox")
}

func (s *Client) Search(ctx context.Context, criteria *imap.SearchCriteria) (ids []uint32, err error) {
	for retryCount := 0; retryCount < reconnectRetryCount; retryCount++ {
		ids, err = s.imapClient.Search(criteria)
		if err == nil {
			return ids, nil
		}

		_ = s.Reconnect(ctx)
	}

	return nil, errors.Wrap(err, "error while searching the messages in imap mailbox")
}

func (s *Client) Fetch(ctx context.Context, seqset *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) (err error) {
	for retryCount := 0; retryCount < reconnectRetryCount; retryCount++ {
		err = s.imapClient.Fetch(seqset, items, ch)
		if err == nil {
			return nil
		}

		_ = s.Reconnect(ctx)
	}

	return errors.Wrap(err, "error while fetching the imap messages")
}

func (s *Client) SelectUnread(ctx context.Context) (messages chan *imap.Message, section *imap.BodySectionName, err error) {
	ctx, span := trace.StartSpan(ctx, "IncomingClient.SelectUnread")
	defer func() {
		span.AddAttributes(trace.Int64Attribute("unseen", int64(cap(messages))))
		tracer.End(span, err)
	}()

	log := logger.GetLogger(ctx)
	log.Info("start SelectUnread")

	s.once.Do(func() {
		criteria := imap.NewSearchCriteria()
		criteria.WithoutFlags = []string{imap.SeenFlag}
		s.criteria = criteria
	})

	mailBox, err := s.Select(ctx, s.imapMailBox, false)
	if err != nil {
		return nil, nil, err
	}

	err = s.Check(ctx)
	if err != nil {
		return nil, nil, err
	}

	if mailBox.Messages == 0 {
		log.Info("mailbox is empty")

		return nil, nil, nil
	}

	log.Info(fmt.Sprintf("found %d messages in mailbox", mailBox.Messages))

	ids, err := s.Search(ctx, s.criteria)
	if err != nil {
		return nil, nil, err
	}

	if len(ids) == 0 {
		log.Info("no unseen messages to process")

		return nil, nil, nil
	}

	log.Info(fmt.Sprintf("Found %d unseen messages to process", len(ids)))

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(ids...)

	section = &imap.BodySectionName{}
	items := []imap.FetchItem{
		imap.FetchEnvelope,
		imap.FetchFlags,
		imap.FetchInternalDate,
		imap.FetchRFC822,
		section.FetchItem(),
	}

	messages = make(chan *imap.Message, len(ids))

	err = s.Fetch(ctx, seqSet, items, messages)
	if err != nil {
		return nil, nil, err
	}

	log.Info("SelectUnread completed successfully")

	return messages, section, nil
}
