package mail

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/iancoleman/orderedmap"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

type userFromSD struct {
	Fullname string `json:"fullname"`
	Username string `json:"username"`
}

const (
	propsDelimiter   = ", "
	attachmentPrefix = "attachment:"
)

func (u userFromSD) String() string {
	return fmt.Sprintf("%s (%s)", u.Fullname, u.Username)
}

func GetAttachmentsFromBody(body orderedmap.OrderedMap) map[string][]entity.Attachment {
	aa := make(map[string][]entity.Attachment)

	iter := func(body orderedmap.OrderedMap) {
		for _, k := range body.Keys() {
			v, _ := body.Get(k)
			switch val := v.(type) {
			case orderedmap.OrderedMap:
				attachmentID, ok := val.Get("file_id")
				if !ok {
					continue
				}
				attachmentIDString, ok := attachmentID.(string)
				if !ok {
					continue
				}
				aa[k] = []entity.Attachment{{FileID: attachmentIDString}}
			case string:
				if !strings.HasPrefix(val, attachmentPrefix) {
					continue
				}
				aa[k] = []entity.Attachment{{FileID: strings.TrimPrefix(val, attachmentPrefix)}}
			case []interface{}:
				a := make([]entity.Attachment, 0)
				for _, item := range val {
					var attachmentID string
					switch itemTyped := item.(type) {
					case string:
						if !strings.HasPrefix(itemTyped, attachmentPrefix) {
							continue
						}
						attachmentID = strings.TrimPrefix(itemTyped, attachmentPrefix)
					case orderedmap.OrderedMap:
						value, ok := itemTyped.Get("file_id")
						if !ok {
							continue
						}
						attachmentID, ok = value.(string)
						if !ok {
							continue
						}
					default:
						continue
					}
					a = append(a, entity.Attachment{FileID: attachmentID})
				}
				aa[k] = a
			}
		}
	}
	iter(body)
	return aa
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
		res.WriteString(toWrite)
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
		} else {
			var propCount = 0
			for _, k := range val.Keys() {
				propCount++
				if v, ok := val.Get(k); ok {
					res.WriteString(fmt.Sprintf("%s", v))
					if propCount < len(val.Keys()) {
						res.WriteString(propsDelimiter)
					}
				}
			}
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
		res.WriteString("</p><br>")
		if i == len(data.Keys())-1 {
			continue
		}
		res.WriteString("\n")
	}
	return res.String()
}
