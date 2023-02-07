package fetcher

import (
	c "context"
	"io"
	"strings"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-message/mail"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"
)

type ActionPayload struct {
	WorkNumber string `json:"workNumber"`
	StepName   string `json:"stepName"`
	ActionName string `json:"actionName"`
	Decision   string `json:"decision"`
	Comment    string `json:"comment"`
}

type ParsedEmail struct {
	From   string
	To     string
	Action *ActionPayload
}

func (s *service) processMessage(ctx c.Context, msg *imap.Message, section *imap.BodySectionName) (*ParsedEmail, error) {
	const fn = "mail.fetcher.processMessage"
	ctx, span := trace.StartSpan(ctx, fn)
	defer span.End()

	if msg == nil {
		err := errors.Wrap(errors.New("server didn't return message"), "no messages")
		return nil, err
	}

	log := logger.GetLogger(ctx)

	msgBody := msg.GetBody(section)
	if msgBody == nil {
		err := errors.Wrap(errors.New("server didn't return message"), "no messages")
		return nil, err
	}

	msgReader, err := mail.CreateReader(msgBody)
	if err != nil {
		err = errors.Wrap(err, "can't create reader")
		return nil, err
	}

	log.Info(fn, "start processing email")

	processedEmail, err := s.parseEmail(ctx, msgReader)
	if err != nil {
		err = errors.Wrap(err, "parseEmail")
		return nil, err
	}

	if processedEmail == nil || processedEmail.Action == nil {
		return nil, nil
	}

	return processedEmail, nil
}

func (s *service) parseEmail(ctx c.Context, r *mail.Reader) (pe *ParsedEmail, err error) {
	const funcName = "mail.fetcher.parseEmail"
	_, span := trace.StartSpan(ctx, funcName)
	defer span.End()

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

	fields := strings.Split(headers.Subject, fieldsDelimiter)
	if len(fields) < 1 {
		return nil, errors.New("invalid subject to parse")
	}

	action, err := parseSubject(fields)
	if err != nil {
		return nil, err
	}

	if action != nil {
		var processedBody *parsedBody
		processedBody, err = parseMsgBody(ctx, r)
		if err != nil {
			return nil, err
		}

		if processedBody != nil {
			action.Comment = processedBody.Body
		}
	}

	return &ParsedEmail{
		From:   from[0],
		To:     to[0],
		Action: action,
	}, nil
}

type parsedHeaders struct {
	From    []*mail.Address
	To      []*mail.Address
	Subject string
}

func parseEmailHeaders(header mail.Header) (headers *parsedHeaders, err error) {
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

	return &parsedHeaders{
		From:    fromAddrs,
		To:      toAddrs,
		Subject: subject,
	}, nil
}

func parseSubject(fields []string) (action *ActionPayload, err error) {
	action = &ActionPayload{}
	for i := range fields {
		keyValue := strings.Split(fields[i], fieldsKeyValueDelimiter)
		if len(keyValue) != 2 {
			return nil, errors.New("parseSubject, invalid subject: " + strings.Join(fields, ""))
		}

		switch keyValue[0] {
		case stepName:
			action.StepName = keyValue[1]
		case decision:
			action.Decision = keyValue[1]
		case workNumber:
			action.WorkNumber = keyValue[1]
		case actionName:
			action.ActionName = keyValue[1]
		}
	}

	return action, err
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

type parsedBody struct {
	Body        string
	Attachments string
}

const (
	startLine = "___ВАШ КОММЕНТАРИЙ НИЖЕ___"
	endLine   = "_____________________________________________________"
)

func parseMsgBody(ctx c.Context, r *mail.Reader) (*parsedBody, error) {
	const funcName = "mail.fetcher.parseMsgBody"

	var (
		body, attachments string
		pb                parsedBody
	)

	log := logger.GetLogger(ctx)

LOOP:
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, errors.Wrap(err, funcName)
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			b, err := io.ReadAll(p.Body)
			if err != nil {
				log.Error(errors.Wrap(err, funcName))
			}
			text := string(b)
			log.Info(text)

			body += text
			break LOOP
		case *mail.AttachmentHeader:
			// This is an attachment
			filename, _ := h.Filename()
			attachments += filename
		}
	}

	if body == "" && attachments == "" {
		pb.Body = ""
		pb.Attachments = attachments
		return &pb, nil
	}

	if !strings.Contains(body, startLine) {
		return nil, errors.Wrap(errors.New("no parsing lines found"), funcName)
	}

	start := strings.Index(body, startLine)
	end := strings.Index(body, endLine)
	if start == -1 || end == -1 || end <= start {
		body = ""
	} else {
		body = strings.TrimSpace(body[start+len(startLine) : end])
	}
	pb.Body = body
	pb.Attachments = attachments

	return &pb, nil
}
