package pipeline

import (
	c "context"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/google/uuid"

	e "gitlab.services.mts.ru/abp/mail/pkg/email"

	"github.com/iancoleman/orderedmap"

	"github.com/pkg/errors"

	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/file-registry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	ServiceAccountDev   = "service-account-jocasta-dev"
	ServiceAccountStage = "service-account-jocasta-stage"
	ServiceAccount      = "service-account-jocasta"
)

type UpdateData struct {
	Id   uuid.UUID
	Data interface{}
}

type castUser struct {
	varValue  interface{}
	result    map[string]struct{}
	toResolve map[string]struct{}
	varName   string
}

const dotSeparator = "."

func (runCtx *BlockRunContext) GetIcons(need []string) ([]e.Attachment, error) {

	outFiles := make([]e.Attachment, 0)
	for k, v := range need {
		file, oks := runCtx.Services.Sender.Images[v]
		if !oks {
			return nil, errors.New("file not found: " + v)
		}

		if k == 0 {
			outFiles = append(outFiles, e.Attachment{Name: headImg, Content: file, Type: e.EmbeddedAttachment})
			continue
		}

		outFiles = append(outFiles, e.Attachment{Name: v, Content: file, Type: e.EmbeddedAttachment})
	}
	return outFiles, nil
}
func (runCtx *BlockRunContext) GetAttach(filesAttach []file_registry.FileInfo) (*mail.Attachments, error) {
	req, skip := sortAndFilterAttachments(filesAttach)

	attachFields, err := runCtx.getFileField()
	if err != nil {
		return nil, err
	}

	attach, err := runCtx.Services.FileRegistry.GetAttachments(c.Background(), req)
	if err != nil {
		return nil, err
	}

	attachLinks, err := runCtx.Services.FileRegistry.GetAttachmentLink(c.Background(), skip)
	if err != nil {
		return nil, err
	}

	attachExists := false
	if len(attach) != 0 {
		attachExists = true
	}

	return &mail.Attachments{AttachFields: attachFields, AttachExists: attachExists, AttachLinks: attachLinks, AttachmentsList: attach}, nil
}

func getVariable(variables map[string]interface{}, key string) interface{} {
	variableMemberNames := strings.Split(key, dotSeparator)
	if len(variableMemberNames) <= 2 {
		return variables[key]
	}

	variable, ok := variables[strings.Join(variableMemberNames, dotSeparator)]
	if ok {
		if _, ok = variable.([]interface{}); ok {
			return variable
		}
	}

	variable, ok = variables[strings.Join(variableMemberNames[:2], dotSeparator)]
	if !ok {
		return nil
	}

	newVariables, ok := variable.(map[string]interface{})
	if !ok {
		newVariables = structToMap(variable)
		if newVariables == nil {
			return nil
		}
	}

	currK := variableMemberNames[2]
	for i := 2; i < len(variableMemberNames)-1; i++ {
		newVariables, ok = newVariables[currK].(map[string]interface{})
		if !ok {
			newVariables = structToMap(variable)
			if newVariables == nil {
				return nil
			}
		}
		currK = variableMemberNames[i+1]
	}
	return newVariables[currK]
}

func getUsersFromVars(varStore map[string]interface{}, toResolve map[string]struct{}) (map[string]struct{}, error) {
	res := make(map[string]struct{})
	for varName := range toResolve {
		if len(strings.Split(varName, dotSeparator)) == 1 {
			continue
		}
		varValue := getVariable(varStore, varName)

		if varValue == nil {
			return nil, errors.New("unable to find value by varName: " + varName)
		}

		if login, castOK := varValue.(string); castOK {
			res[login] = toResolve[varName]
		}

		CastUserForLogin(castUser{
			varValue:  varValue,
			result:    res,
			toResolve: toResolve,
			varName:   varName,
		})

		if people, castOk := varValue.([]interface{}); castOk {
			for _, castedPerson := range people {
				CastUserForLogin(castUser{
					varValue:  castedPerson,
					result:    res,
					toResolve: toResolve,
					varName:   varName,
				})
			}
		}

		return res, nil
	}

	return nil, errors.New("unexpected behavior")
}

func CastUserForLogin(castData castUser) {
	if person, castOk := castData.varValue.(map[string]interface{}); castOk {
		if login, exists := person["username"]; exists {
			if loginString, castOK := login.(string); castOK {
				castData.result[loginString] = castData.toResolve[castData.varName]
			}
		}
	}
	return
}

func getSliceFromMapOfStrings(source map[string]struct{}) []string {
	var result = make([]string, 0)

	for key := range source {
		result = append(result, key)
	}

	return result
}

// nolint:deadcode,unused //used in tests
func getStringAddress(s string) *string {
	return &s
}

func getRecipientFromState(applicationBody *orderedmap.OrderedMap) string {
	if applicationBody == nil {
		return ""
	}

	var login string
	if recipientValue, ok := applicationBody.Get("recipient"); ok {
		if recipient, ok := recipientValue.(orderedmap.OrderedMap); ok {
			if usernameValue, ok := recipient.Get("username"); ok {
				if username, ok := usernameValue.(string); ok {
					login = username
				}
			}
		}
	}

	return login
}

func structToMap(variable interface{}) map[string]interface{} {
	variableType := reflect.TypeOf(variable)
	if !(variableType.Kind() == reflect.Struct ||
		(variableType.Kind() == reflect.Pointer && variableType.Elem().Kind() == reflect.Struct)) {
		return nil
	}

	bytes, err := json.Marshal(variable)
	if err != nil {
		return nil
	}

	res := make(map[string]interface{})
	if unmErr := json.Unmarshal(bytes, &res); unmErr != nil {
		return nil
	}

	return res
}

func getBlockOutput(varStore *store.VariableStore, node string) map[string]interface{} {
	res := make(map[string]interface{})

	if varStore == nil {
		return res
	}

	storage, _ := varStore.GrabStorage()
	for k, v := range storage {
		if strings.HasPrefix(k, node) {
			newK := strings.Replace(k, node+".", "", 1)
			res[newK] = v
		}
	}

	return res
}
