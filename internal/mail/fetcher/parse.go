package imapparser

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-message/mail"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/briefcase/notification-service/pkg/mail/answer"
	"gitlab.services.mts.ru/prodboard/infra/tracer"
)

func (s *FetchService) processMessage(ctx context.Context, msg *imap.Message, section *imap.BodySectionName) (err error) {
	ctx, span := trace.StartSpan(ctx, "IncomingService.processMessage")
	defer func() { tracer.End(span, err) }()

	if msg == nil {
		err = errors.Wrap(errors.New("server didn't return message"), "no messages")
		return err
	}

	r := msg.GetBody(section)
	if r == nil {
		err = errors.Wrap(errors.New("server didn't return message"), "no messages")
		return err
	}

	msgReader, err := mail.CreateReader(r)
	if err != nil {
		err = errors.Wrap(err, "can't create reader")
		return err
	}

	s.log.Info("Start processing email")

	processedEmail, err := s.parseEmail(ctx, msgReader)
	if err != nil {
		err = errors.New("parseMgsHeaders")
		return err
	}

	if processedEmail == nil || processedEmail.Action == nil {
		return
	}

	switch processedEmail.TemplateName {
	case answer.TdEngineCoreWebApproverDecisionCode:
		if decisionMeta, ok := processedEmail.Action.(*answer.TDWebApproverDecision); ok {
			err = s.tdRequestService.UpdateProcessNodeProps(ctx, decisionMeta.ToUpdateProcessNodePropsRequest())
			if err != nil {
				return err
			}
		}
	case answer.TdEngineCoreWebDirectorApproverDecisionCode:
		if decisionMeta, ok := processedEmail.Action.(*answer.TDWebDirectorApproverDecision); ok {

			nodeIds, err := s.tdRequestService.GetLawyersDigitalProjectNodesIds(ctx, decisionMeta.ProjectCode)
			if err != nil {
				return err
			}

			s.log.Info("TdEngineCoreWebDirectorApproverDecisionCode ids", nodeIds)

			for i := range nodeIds {
				err = s.tdRequestService.UpdateProcessNodeProps(
					ctx,
					decisionMeta.ToUpdateProcessNodePropsRequest(nodeIds[i]),
				)
				if err != nil {
					s.log.Warn(err, "nodeId: " + nodeIds[i])
				}
			}
		}
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

func (s *FetchService) parseEmail(ctx context.Context, r *mail.Reader) (pe *parsedEmail, err error) {
	const funcName = "IncomingService.parseEmail"
	var subject string
	_, span := trace.StartSpan(ctx, funcName)
	defer func() {
		span.AddAttributes(trace.StringAttribute("subject", fmt.Sprintf("%+v", subject)))
		tracer.End(span, err)
	}()

	headers, err := parseEmailHeaders(r.Header)
	if err != nil {
		return nil, errors.Wrap(err, funcName+": headers")
	}

	from := AddressListToStrList(headers.From)
	if len(from) == 0 {
		return nil, errors.New("invalid from header")
	}

	to := AddressListToStrList(headers.To)
	if len(to) == 0 {
		return nil, errors.New("invalid to header")
	}

	subject = headers.Subject

	fields := strings.Split(subject, answer.FieldsDelimiter)
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
	templateNameField := strings.Split(fields[0], answer.FieldsKeyValueDelimiter)
	if len(templateNameField) != 2 {
		return nil, "", errors.New("invalid template code to parse")
	}

	tmplName = templateNameField[1]

	switch tmplName {
	case answer.TdEngineCoreWebApproverDecisionCode:
		action, err := answer.NewTDWebApproverDecision(from, to, fields)
		if err != nil {
			return nil, tmplName, err
		}
		return action, tmplName, nil
	case answer.TdEngineCoreWebDirectorApproverDecisionCode:
		action, err := answer.NewTDWebDirectorApproverDecision(from, fields)
		if err != nil {
			return nil, tmplName, err
		}
		return action, tmplName, nil

	default:
		return nil, tmplName, errors.New(fmt.Sprintf("template with code \"%s\" doesn't exist", templateNameField[1]))
	}
}

func AddressListToStrList(addrs []*mail.Address) (res []string) {
	res = make([]string, 0)
	for i := range addrs {
		if addrs[i] != nil && len(addrs[i].Address) > 0 {
			res = append(res, addrs[i].Address)
		}
	}

	return res
}
