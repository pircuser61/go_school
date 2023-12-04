package mail

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/mail"
	"os"
	"regexp"
	"strings"
	"text/template"

	"go.opencensus.io/trace"

	"github.com/labstack/gommon/log"

	"gitlab.services.mts.ru/abp/mail/pkg/broker"
	"gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/abp/mail/pkg/mailclient"
)

const imageMimeTypePrefix = "image"
const messageTplStart = `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
    <meta content="width=device-width, initial-scale=1" name="viewport" />
    <meta name="x-apple-disable-message-reformatting" />
    <meta http-equiv="X-UA-Compatible" content="IE=edge" />
    <meta content="telephone=no" name="format-detection" />
    <title></title>
    <!--[if mso]>
    <xml>
        <o:OfficeDocumentSettings>
            <o:AllowPNG />
            <o:PixelsPerInch>96</o:PixelsPerInch>
        </o:OfficeDocumentSettings>
    </xml>
    <![endif]-->
    <!--[if lte mso 11]>
    <style type="text/css">
        .mj-outlook-group-fix {
            width: 100% !important;
        }
        .preheader {
            display: none !important;
            visibility: hidden;
            opacity: 0;
            color: transparent;
            height: 0;
            width: 0;
        }
    </style>
    <![endif]-->
</head><body>
`
const msgTplEnd = "</body></html>"
const headTemp = "internal/mail/template/00header-template.html"

type Service struct {
	cli *mailclient.Client

	from *mail.Address

	Images     map[string][]byte
	SdAddress  string
	FetchEmail string
}

// nolint:gocritic // it's more comfortable to work with config as a value
func NewService(c Config) (*Service, error) {
	cfg := &broker.Config{
		Broker:       broker.Kind(c.Broker),
		Host:         c.Host,
		Port:         c.Port,
		Database:     c.Database,
		Queue:        c.Queue,
		ReadTimeout:  c.ReadTimeout,
		WriteTimeout: c.WriteTimeout,
	}

	images, err := getImages(c.ImagesPath)
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
	}
	return &s, nil
}

func (s *Service) GetApplicationLink(applicationID string) string {
	return fmt.Sprintf(TaskUrlTemplate, s.SdAddress, applicationID)
}

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
	}).ParseFiles(headTemp, tmpl.Template)
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

func getImages(path string) (map[string][]byte, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		log.Error("error read directory", err)
		return nil, err
	}

	photos := make(map[string][]byte)
	for _, v := range files {
		data, readErr := os.ReadFile(path + v.Name())
		if readErr != nil {
			log.Error("error read file ", v.Name(), err)
			return nil, readErr
		}
		photos[v.Name()] = data
	}

	return photos, nil
}
