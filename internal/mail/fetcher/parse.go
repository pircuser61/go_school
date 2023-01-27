package fetcher

import (
	"context"
	"strings"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-message/mail"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"
)

type ApproverDecisionPayload struct {
	WorkNumber string
	Decision   string
}

type ExecutorDecisionPayload struct {
	WorkNumber string
	Decision   string
}

func (s *service) processMessage(ctx context.Context, msg *imap.Message, section *imap.BodySectionName) (err error) {
	const fn = "mail.fetcher.processMessage"
	ctx, span := trace.StartSpan(ctx, fn)
	defer span.End()

	if msg == nil {
		err = errors.Wrap(errors.New("server didn't return message"), "no messages")
		return err
	}

	log := logger.GetLogger(ctx)

	msgBody := msg.GetBody(section)
	if msgBody == nil {
		err = errors.Wrap(errors.New("server didn't return message"), "no messages")
		return err
	}

	msgReader, err := mail.CreateReader(msgBody)
	if err != nil {
		err = errors.Wrap(err, "can't create reader")
		return err
	}

	log.Info(fn, "Start processing email")

	processedEmail, err := s.parseEmail(ctx, msgReader)
	if err != nil {
		err = errors.Wrap(err, "parseMgsHeaders")
		return err
	}

	if processedEmail == nil || processedEmail.Action == nil {
		return
	}

	return nil
}

type parsedEmail struct {
	Date         time.Time
	From         string
	To           string
	TemplateName string
	Action       interface{}
}

func (s *service) parseEmail(ctx context.Context, r *mail.Reader) (pe *parsedEmail, err error) {
	const funcName = "mail.fetcher.parseEmail"
	_, span := trace.StartSpan(ctx, funcName)
	defer span.End()

	var subject string

	headers, err := parseEmailHeaders(r.Header)
	if err != nil {
		return nil, errors.Wrap(err, funcName+": headers")
	}

	from := addressListToStrList(headers.From)
	if len(from) == 0 {
		return nil, errors.New("invalid from header")
	}

	to := addressListToStrList(headers.To)
	if len(to) == 0 {
		return nil, errors.New("invalid to header")
	}

	subject = headers.Subject

	fields := strings.Split(subject, fieldsDelimiter)
	if len(fields) < 1 {
		return nil, errors.New("invalid subject to parse")
	}

	action, tmplName, err := parseAction(fields, from, to)
	if err != nil {
		return nil, err
	}

	return &parsedEmail{
		Date:         headers.Date,
		From:         from[0],
		To:           to[0],
		TemplateName: tmplName,
		Action:       action,
	}, nil
}

type ParsedHeaders struct {
	Date    time.Time
	From    []*mail.Address
	To      []*mail.Address
	Subject string
}

func parseEmailHeaders(header mail.Header) (parsedHeaders *ParsedHeaders, err error) {
	date, err := header.Date()
	if err != nil {
		return nil, errors.Wrap(err, ": date")
	}

	fromAddrs, err := header.AddressList("From")
	if err != nil {
		return nil, errors.Wrap(err, ": header From")
	}

	toAddrs, err := header.AddressList("To")
	if err != nil {
		return nil, errors.Wrap(err, ": header To")
	}

	subject, err := header.Subject()
	if err != nil {
		return nil, errors.Wrap(err, ": header Subject")
	}

	return &ParsedHeaders{
		Date:    date,
		From:    fromAddrs,
		To:      toAddrs,
		Subject: subject,
	}, nil
}

func parseAction(fields, from, to []string) (action interface{}, tmplName string, err error) {
	templateNameField := strings.Split(fields[0], fieldsKeyValueDelimiter)
	if len(templateNameField) != 2 {
		return nil, "", errors.New("invalid template code to parse")
	}

	tmplName = templateNameField[1]

	return nil, "", err
}

func addressListToStrList(addrs []*mail.Address) (res []string) {
	res = make([]string, 0)
	for i := range addrs {
		if addrs[i] != nil && len(addrs[i].Address) > 0 {
			res = append(res, addrs[i].Address)
		}
	}

	return res
}
