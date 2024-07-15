package fetcher

import (
	c "context"
	"io"
	"regexp"
	"strings"

	"github.com/emersion/go-imap"

	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

type AttachmentData struct {
	Raw []byte
	Ext string
}

type ActionPayload struct {
	WorkNumber     string                    `json:"workNumber"`
	StepName       string                    `json:"stepName"`
	ActionName     string                    `json:"actionName"`
	Decision       string                    `json:"decision"`
	Comment        string                    `json:"comment"`
	Login          string                    `json:"login"`
	AttachmentsIds []entity.Attachment       `json:"attachments"`
	Attachments    map[string]AttachmentData `json:"-"`
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

	msgBodyMap := make(map[*imap.BodySectionName]imap.Literal)
	for k, v := range msg.Body {
		msgBodyMap[k] = v
	}

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

	processedEmail, err := s.parseEmail(ctx, msgReader, msgBodyMap)
	if err != nil {
		err = errors.Wrap(err, "parse email")

		return nil, err
	}

	if processedEmail == nil || processedEmail.Action == nil {
		return nil, errors.New("processedEmail is nil")
	}

	return processedEmail, nil
}

func (s *service) parseEmail(ctx c.Context, r *mail.Reader, sn map[*imap.BodySectionName]imap.Literal) (pe *ParsedEmail, err error) {
	const (
		funcName = "mail.fetcher.parseEmail"
		rejected = "Отклонено"
	)

	log := logger.GetLogger(ctx).WithField("funcName", funcName)

	_, span := trace.StartSpan(ctx, funcName)
	defer span.End()

	headers, err := parseEmailHeaders(r.Header)
	if err != nil {
		return nil, err
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
		comment := getComment(ctx, r)

		if comment != nil {
			action.Comment = comment.Body

			action.Attachments, err = s.getAttachments(ctx, sn)
			if err != nil {
				log.WithError(err).Error("can't parse message body: " + action.WorkNumber)
			}
		}

		if action.Comment == "" {
			switch action.Decision {
			case "approve":
				action.Comment = "Согласовано"
			case "confirm":
				action.Comment = "Утверждено"
			case "informed":
				action.Comment = "Проинформировано"
			case "reject":
				action.Comment = rejected
			case "sign":
				action.Comment = "Подписано"
			case "viewed":
				action.Comment = "Ознакомлено"
			case "executed":
				action.Comment = "Решено"
			case "rejected":
				action.Comment = rejected
			}
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
	to, err := header.AddressList("To")
	if err != nil {
		return nil, errors.Wrap(err, "header To")
	}

	from, err := header.AddressList("From")
	if err != nil {
		return nil, errors.Wrap(err, "header From")
	}

	subject, err := header.Subject()
	if err != nil {
		return nil, errors.Wrap(err, "header Subject")
	}

	return &parsedHeaders{
		From:    from,
		To:      to,
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
		case login:
			action.Login = keyValue[1]
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
	Body string
}

func getComment(ctx c.Context, r *mail.Reader) *parsedBody {
	log := logger.GetLogger(ctx)

	var (
		body string
		pb   parsedBody
	)

LOOP:
	for {
		part, err := r.NextPart()
		if err != nil && err == io.EOF {
			log.Info("readPart EOF")

			break
		} else if err != nil {
			log.Error(errors.Wrap(err, "can`t next part"))

			break
		}

		switch part.Header.(type) {
		case *mail.InlineHeader:
			if !strings.Contains(body, "40МБ***") {
				b, errRead := io.ReadAll(part.Body)
				if errRead != nil {
					log.
						WithField("text", string(b)).
						Error(errors.Wrap(errRead, "can`t read body"))

					break LOOP
				}

				body += regexp.MustCompile(`(\[.+\])`).ReplaceAllString(string(b), "")
				body = regexp.MustCompile(`(\*{3}.+\*{3})`).ReplaceAllString(body, "")
			}

			break LOOP
		}
	}

	pb.Body = strings.ReplaceAll(body, "\n", " ")
	pb.Body = strings.ReplaceAll(pb.Body, "\t", "")
	pb.Body = strings.TrimSpace(pb.Body)

	return &pb
}

func (s *service) getAttachments(ctx c.Context, mb map[*imap.BodySectionName]imap.Literal) (attach map[string]AttachmentData, err error) {
	attach = make(map[string]AttachmentData)

	log := logger.GetLogger(ctx)

	for _, r := range mb {
		messageEntity, err := message.Read(r)
		if err != nil {
			log.Error(errors.Wrap(err, "can`t read attachments"))

			continue
		}

		if messageEntity == nil {
			log.Error(errors.Wrap(err, "can`t read attachments messageEntity is nil"))

			continue
		}

		multiPartReader := messageEntity.MultipartReader()
		if multiPartReader == nil {
			log.Error(errors.Wrap(err, "can`t read attachments multiPartReader is nil"))

			continue
		}

		for part, err := multiPartReader.NextPart(); err != io.EOF; part, err = multiPartReader.NextPart() {
			_, params, cErr := part.Header.ContentType()
			if cErr != nil {
				log.Error(errors.Wrap(cErr, "can`t read attachment"))

				return nil, cErr
			}

			filename := params["name"]
			log.Info("file params: ", params)

			nameParts := strings.Split(filename, ".")
			log.Info("attachmentName: ", filename)

			fileBytes, rErr := io.ReadAll(part.Body)
			if rErr != nil {
				log.Error(errors.Wrap(rErr, "can`t read part mail body"))

				return nil, rErr
			}

			if len(nameParts) < 2 {
				continue
			}

			ext := nameParts[len(nameParts)-1]
			attach[filename] = AttachmentData{Raw: fileBytes, Ext: ext}
		}
	}

	return attach, nil
}
