package mail

import (
	"strings"
	"testing"

	"github.com/iancoleman/orderedmap"

	"github.com/stretchr/testify/assert"
)

func TestToUser(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
		want userFromSD
	}{
		{
			name: "ok",
			data: map[string]interface{}{
				"fullname": "Иванов Иван Иванович",
				"username": "ivanov",
			},
			want: userFromSD{
				Fullname: "Иванов Иван Иванович",
				Username: "ivanov",
			},
		},
		{
			name: "more fields",
			data: map[string]interface{}{
				"fullname": "Иванов Иван Иванович",
				"username": "ivanov",
				"test":     "test",
			},
			want: userFromSD{
				Fullname: "Иванов Иван Иванович",
				Username: "ivanov",
			},
		},
		{
			name: "field missing",
			data: map[string]interface{}{
				"fullname": "Иванов Иван Иванович",
			},
			want: userFromSD{
				Fullname: "",
				Username: "",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := orderedmap.New()
			for k, v := range test.data {
				data.Set(k, v)
			}
			user, _ := toUser(*data)
			assert.Equal(t, test.want, user)
		})
	}
}

func TestWriteValue(t *testing.T) {
	user := orderedmap.New()
	user.Set("fullname", "Иванов Иван Иванович")
	user.Set("username", "ivanov")

	tests := []struct {
		name string
		data interface{}
		want string
	}{
		{
			name: "float64",
			data: 1.1,
			want: "1.1",
		},
		{
			name: "int64",
			data: 1.0,
			want: "1",
		},
		{
			name: "bool true",
			data: true,
			want: "Да",
		},
		{
			name: "bool false",
			data: false,
			want: "Нет",
		},
		{
			name: "array simple",
			data: []interface{}{"test1, test2"},
			want: "test1, test2",
		},
		{
			name: "user",
			data: *user,
			want: "Иванов Иван Иванович (ivanov)",
		},
		{
			name: "object",
			data: *orderedmap.New(),
			want: "",
		},
		{
			name: "string",
			data: "test",
			want: "test",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := strings.Builder{}
			writeValue(&res, test.data)
			assert.Equal(t, test.want, res.String())
		})
	}
}

func TestGetAttachmentsFromBody(t *testing.T) {
	tests := []struct {
		name   string
		data   string
		fields []string
		want   map[string][]string
	}{
		{
			name: "all types",
			data: `{"recipient": {"email": "snkosya1@mts.ru", "phone": "15857", "mobile": "+79111157031", 
"tabnum": "415336", "fullname": "Косяк Сергей Николаевич", "position": "ведущий разработчик", 
"username": "snkosya1"}, "chislo_moe": 12, "stroka_moya": "строка", 
"vlozhenie_odno": "34bc6b5b-2391-11ed-b54b-04505600ad66", 
"vlozhenie_mnogo": ["34b9dd4a-2391-11ed-b54b-04505600ad66", "366bc146-2391-11ed-b54b-04505600ad66"]}`,
			fields: []string{".vlozhenie_odno", ".vlozhenie_mnogo"},
			want: map[string][]string{
				"vlozhenie_odno":  []string{"34bc6b5b-2391-11ed-b54b-04505600ad66"},
				"vlozhenie_mnogo": []string{"34b9dd4a-2391-11ed-b54b-04505600ad66", "366bc146-2391-11ed-b54b-04505600ad66"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := orderedmap.New()
			if err := data.UnmarshalJSON([]byte(test.data)); err != nil {
				t.Fatal(err)
			}
			aa := GetAttachmentsFromBody(*data, test.fields)
			assert.Equal(t, test.want, aa)
		})
	}
}

func TestMakeDescription(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "all types",
			data: `{"input_float64": 1.1, "int64": 2, "bool_true": true, "bool_false": false, "array": ["test", "test2"],
"user": {"fullname": "Иванов Иван Иванович", "username": "ivanov"}, 
"map" : {"testProperty1" : "testPropertyValue1", "testProperty2" : "testPropertyValue2"}, "string": "string"}`,
			want: `<p>input_float64: 1.1</p><br>
<p>int64: 2</p><br>
<p>bool_true: Да</p><br>
<p>bool_false: Нет</p><br>
<p>array: test, test2</p><br>
<p>user: Иванов Иван Иванович (ivanov)</p><br>
<p>map: testPropertyValue1, testPropertyValue2</p><br>
<p>string: string</p><br>`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := orderedmap.New()
			if err := data.UnmarshalJSON([]byte(test.data)); err != nil {
				t.Fatal(err)
			}
			descr := makeDescriptionFromJSON(*data)
			assert.Equal(t, test.want, descr)
		})
	}
}
