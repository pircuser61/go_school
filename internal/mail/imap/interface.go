package imap

import (
	c "context"
	"github.com/emersion/go-imap"
)

type IncomingClient interface {
	Close(ctx c.Context)
	SelectUnread(ctx c.Context) (chan *imap.Message, *imap.BodySectionName, error)
}
