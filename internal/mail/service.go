package mail

import (
	"bytes"
	c "context"
	"fmt"
	"net/http"
	"net/mail"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/mail/pkg/broker"
	"gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/abp/mail/pkg/mailclient"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"

	"gitlab.services.mts.ru/abp/myosotis/logger"
)

const (
	imageMimeTypePrefix = "image"
	headTemp            = "internal/mail/template/00header-template.html"
	imagePath           = "./internal/mail/img/"
)

type Service struct {
	cli        *mailclient.Client
	from       *mail.Address
	Images     map[string][]byte
	SdAddress  string
	FetchEmail string
	host       string
	metrics    metrics.Metrics
}

func (s *Service) Ping(ctx c.Context) error {
	return nil
}

// nolint:gocritic // it's more comfortable to work with config as a value
func NewService(c Config, m metrics.Metrics) (*Service, error) {
	cfg := &broker.Config{
		Broker:       broker.Kind(c.Broker),
		Host:         c.Host,
		Port:         c.Port,
		Database:     c.Database,
		Queue:        c.Queue,
		ReadTimeout:  c.ReadTimeout,
		WriteTimeout: c.WriteTimeout,
	}

	images, err := getImages(imagePath)
	if err != nil {
		return nil, err
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
		Images:     images,
		SdAddress:  c.SdAddress,
		FetchEmail: c.FetchEmail,
		host:       c.Host,
		metrics:    m,
	}

	return &s, nil
}

func (s *Service) GetApplicationLink(applicationID string) string {
	return fmt.Sprintf(TaskURLTemplate, s.SdAddress, applicationID)
}

func (s *Service) SendNotification(ctx c.Context, to []string, files []email.Attachment, tmpl Template) error {
	const externalSystemName = "mail.inside"

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

	personMail := make(map[string]struct{})

	for _, person := range to {
		if !regexp.MustCompile(`.+@.+`).MatchString(person) {
			continue
		}

		if _, ok := personMail[person]; !ok {
			msg.To = append(msg.To, &mail.Address{Address: person})
			personMail[person] = struct{}{}
		}
	}

	temp, err := template.New("00header-template.html").Funcs(template.FuncMap{
		"isUser":   isUser,
		"retMap":   retMap,
		"isLink":   isLink,
		"isFile":   isFile,
		"checkKey": checkKey,
		"hasValue": hasValue,
		"toMbyte":  toMbyte,
	}).ParseFiles(headTemp, tmpl.Template)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	if execErr := temp.Execute(&b, tmpl.Variables); execErr != nil {
		return execErr
	}

	msg.Text = b.String()

	info := metrics.NewExternalRequestInfo(externalSystemName)
	info.Method = "post"
	info.URL = s.host
	info.TraceID = span.SpanContext().TraceID.String()

	start := time.Now()

	err = s.cli.Send(msg)

	statusCode := http.StatusOK
	if err != nil {
		statusCode = http.StatusInternalServerError
	}

	info.ResponseCode = statusCode
	info.Duration = time.Since(start)

	s.metrics.Request2ExternalSystem(info)

	return err
}

func getImages(path string) (map[string][]byte, error) {
	log := logger.GetLogger(c.Background())

	files, err := os.ReadDir(path)
	if err != nil {
		msg := fmt.Sprintf("error read directory path: %v error: %v", path, err)
		log.Error(msg)

		return nil, err
	}

	photos := make(map[string][]byte)

	for _, v := range files {
		data, readErr := os.ReadFile(path + v.Name())
		if readErr != nil {
			msg := fmt.Sprintf("error read file %s, %s", v.Name(), err)
			log.Error(msg)

			return nil, readErr
		}

		photos[v.Name()] = data
	}

	return photos, nil
}
