package mail

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/iancoleman/orderedmap"
)

type userFromSD struct {
	Fullname string `json:"fullname"`
	Username string `json:"username"`
}

const propsDelimiter = ", "

func (u userFromSD) String() string {
	return fmt.Sprintf("%s (%s)", u.Fullname, u.Username)
}

func GetAttachmentsFromBody(body orderedmap.OrderedMap, fields []string) map[string][]string {
	aa := make(map[string][]string)

	ff := make(map[string]struct{})
	for _, f := range fields {
		ff[strings.Trim(f, ".")] = struct{}{}
	}

	iter := func(body orderedmap.OrderedMap) {
		for _, k := range body.Keys() {
			if _, ok := ff[k]; !ok {
				continue
			}
			v, _ := body.Get(k)
			switch val := v.(type) {
			case string:
				aa[k] = []string{val}
			case []interface{}:
				a := make([]string, 0)
				for _, item := range val {
					if _, ok := item.(string); ok {
						a = append(a, item.(string))
					}
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

func MakeDescription(data orderedmap.OrderedMap) string {
	descr := makeDescriptionFromJSON(data)
	return descr
}

func SwapKeys(body string, keys map[string]string) string {
	for k, v := range keys {
		body = strings.Replace(body, "<p>"+k+":", "<p>"+v+":", -1)
	}
	return body
}

func MakeBodyHeader(fullname, username, link, initialDescription string) string {
	res := strings.Builder{}
	res.WriteString(fmt.Sprintf("<p>%s<p><br>", initialDescription))
	res.WriteString(fmt.Sprintf("<p> <b>Инициатор: </b>%s</p> <br>", userFromSD{fullname, username}.String()))
	res.WriteString(fmt.Sprintf("<p> <b>Ссылка: </b><a href=%q>%s</a></p> <br>", link, link))
	return res.String()
}

func WrapDescription(header, body string) string {
	res := strings.Builder{}
	res.WriteString(header)
	res.WriteString("<p> ------------ Описание ------------ </p> <br>")
	res.WriteString(body)

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
       }
</style>`)
	return res.String()
}
