package mail

import (
	"fmt"
	"github.com/iancoleman/orderedmap"
	"gitlab.services.mts.ru/abp/mail/pkg/email"
	"reflect"
	"strings"
)

type userFromSD struct {
	Fullname string `json:"fullname"`
	Username string `json:"username"`
}

type attachment struct {
	fieldName string
	attach    email.Attachment
}

func (u userFromSD) String() string {
	return fmt.Sprintf("%s (%s)", u.Fullname, u.Username)
}

func toUser(data orderedmap.OrderedMap) (userFromSD, bool) {
	user := &userFromSD{}
	uv := reflect.ValueOf(user).Elem()
	ut := reflect.TypeOf(userFromSD{})
	nf := ut.NumField()
	for i := 0; i < nf; i++ {
		tf := ut.Field(i)
		val, ok := data.Get(tf.Tag.Get("json"))
		if !ok {
			return userFromSD{}, false
		}
		convVal, ok := val.(string)
		if !ok {
			return userFromSD{}, false
		}
		vf := uv.Field(i)
		if vf.CanSet() {
			vf.SetString(convVal)
		}
	}
	return *user, true
}

func writeValue(res *strings.Builder, data interface{}) {
	switch val := data.(type) {
	case float64:
		if val == float64(int64(val)) {
			res.WriteString(fmt.Sprintf("%v", int64(val)))
			return
		}
		res.WriteString(fmt.Sprintf("%v", val))
	case bool:
		toWrite := "Нет"
		if val {
			toWrite = "Да"
		}
		res.WriteString(fmt.Sprintf("%v", toWrite))
	case []interface{}:
		for i := range val {
			item := val[i]
			writeValue(res, item)
			if i < len(val)-1 {
				res.WriteString(", ")
			}
		}
	case orderedmap.OrderedMap:
		if user, ok := toUser(val); ok {
			res.WriteString(user.String())
		}
	default:
		res.WriteString(fmt.Sprintf("%v", data))
	}
}

func makeDescriptionFromJSON(data orderedmap.OrderedMap) string {
	res := strings.Builder{}
	for i := range data.Keys() {
		k := data.Keys()[i]
		res.WriteString(fmt.Sprintf("<p>%s: ", k))
		v, _ := data.Get(k)
		writeValue(&res, v)
		res.WriteString("</p></br>")
		if i == len(data.Keys())-1 {
			continue
		}
		res.WriteString("\n")
	}
	return res.String()
}

func addAttachments(res *strings.Builder, attachments []attachment) {
	for _, a := range attachments {
		res.WriteString(fmt.Sprintf("<p>%s: %s</p></br>", a.fieldName, a.attach.Name))
	}
}

func MakeDescription(data orderedmap.OrderedMap, keys map[string]string) string {
	descr := makeDescriptionFromJSON(data)
	for k, v := range keys {
		descr = strings.Replace(descr, "<p>"+k+":", "<p>"+v+":", -1)
	}
	return descr
}

func MakeBodyHeader(fullname, username, link string, initialDescription string) string {
	res := strings.Builder{}
	res.WriteString(fmt.Sprintf("<p>%s<p>", initialDescription))
	res.WriteString(fmt.Sprintf("<p> <b>Инициатор: </b>%s</p> </br>", userFromSD{fullname, username}.String()))
	res.WriteString(fmt.Sprintf("<p> <b>Ссылка: </b><a href=\"%s\">%s</a></p> </br>", link, link))
	return res.String()
}

func WrapDescription(header, body string, attachments []attachment) string {
	res := strings.Builder{}
	res.WriteString(header)
	res.WriteString("<p> ------------ Описание ------------ </p> </br>")
	res.WriteString(body)
	addAttachments(&res, attachments)

	return res.String()
}

func AddStyles(description string) string {
	res := strings.Builder{}
	res.WriteString(description)
	res.WriteString(`
<style>
    p { 
       font-family: Arial;
       font-size: 11px;
       margin-bottom: -20px;
       }
</style>`)
	return res.String()
}
