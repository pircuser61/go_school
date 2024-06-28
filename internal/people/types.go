package people

import (
	"encoding/json"
	"fmt"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

type SSOUserAttributes struct {
	TelephoneNumber   string   `json:"telephoneNumber"`
	OrgUnit           string   `json:"OrgUnit"`
	WhenCreated       string   `json:"whenCreated"`
	Manager           string   `json:"manager"`
	Mobile            string   `json:"mobile"`
	EmployeeID        string   `json:"employeeID"`
	L                 string   `json:"l"`
	Title             string   `json:"title"`
	LDAPID            string   `json:"LDAPID"`
	LDAPENTRYDN       string   `json:"LDAP_ENTRY_DN"`
	Phone             string   `json:"phone"`
	ThumbnailPhoto    string   `json:"thumbnailPhoto"`
	MemberOf          []string `json:"memberOf,omitempty"`
	FullName          string   `json:"fullname"`
	UserPrincipalName string   `json:"userPrincipalName"`
	ProxyEmails       []string `json:"proxyAddresses"`
}

func zeroOrDefault(ss []string) string {
	if len(ss) == 0 {
		return ""
	}

	return ss[0]
}

type SSOUserAttributesRAW struct {
	OrgUnit           []string `json:"OrgUnit"`
	WhenCreated       []string `json:"whenCreated"`
	Manager           []string `json:"manager"`
	Mobile            []string `json:"mobile"`
	EmployeeID        []string `json:"employeeID"`
	L                 []string `json:"l"`
	Title             []string `json:"title"`
	LDAPID            []string `json:"LDAPID"`
	LDAPENTRYDN       []string `json:"LDAP_ENTRY_DN"`
	Phone             []string `json:"phone"`
	ThumbnailPhoto    []string `json:"thumbnailPhoto"`
	MemberOf          []string `json:"memberOf,omitempty"`
	FullName          []string `json:"fullname"`
	UserPrincipalName []string `json:"userPrincipalName"`
	TelephoneNumber   []string `json:"telephoneNumber"`
	ProxyEmails       []string `json:"proxyAddresses"`
}

func (a *SSOUserAttributes) UnmarshalJSON(data []byte) error {
	var raw SSOUserAttributesRAW

	err := json.Unmarshal(data, &raw)
	if err != nil {
		return err
	}

	newA := SSOUserAttributes{
		OrgUnit:           zeroOrDefault(raw.OrgUnit),
		WhenCreated:       zeroOrDefault(raw.WhenCreated),
		Manager:           zeroOrDefault(raw.Manager),
		Mobile:            zeroOrDefault(raw.Mobile),
		EmployeeID:        zeroOrDefault(raw.EmployeeID),
		L:                 zeroOrDefault(raw.L),
		Title:             zeroOrDefault(raw.Title),
		LDAPID:            zeroOrDefault(raw.LDAPID),
		LDAPENTRYDN:       zeroOrDefault(raw.LDAPENTRYDN),
		Phone:             zeroOrDefault(raw.Phone),
		ThumbnailPhoto:    zeroOrDefault(raw.ThumbnailPhoto),
		MemberOf:          raw.MemberOf,
		FullName:          zeroOrDefault(raw.FullName),
		UserPrincipalName: zeroOrDefault(raw.UserPrincipalName),
		TelephoneNumber:   zeroOrDefault(raw.TelephoneNumber),
		ProxyEmails:       raw.ProxyEmails,
	}

	*a = newA

	return nil
}

type SSOUserTyped struct {
	ID                      string            `json:"id"`
	CreatedTimestamp        int               `json:"createdTimestamp"`
	Username                string            `json:"username"`
	Enable                  bool              `json:"enable"`
	Totp                    bool              `json:"totp"`
	EmailVerified           bool              `json:"emailVerified"`
	FirstName               string            `json:"firstName"`
	LastName                string            `json:"lastName"`
	Email                   string            `json:"email"`
	FederationLink          string            `json:"federationLink"`
	Attributes              SSOUserAttributes `json:"attributes"`
	DisabledCredentialTypes []string          `json:"disabledCredentialTypes"`
	RequiredActions         []string          `json:"requiredActions"`
	NotBefore               int               `json:"notBefore"`
	Tabnum                  string            `json:"tabnum"`
	UnitPaths               string            `json:"unitPaths"`
}

type SSOUser map[string]interface{}

func (u SSOUser) ToSSOUserTyped() (*SSOUserTyped, error) {
	var ui SSOUserTyped

	bb, err := json.Marshal(u)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(bb, &ui)
	if err != nil {
		return nil, err
	}

	return &ui, nil
}

func (u SSOUser) ToUserinfo() (*sso.UserInfo, error) {
	typed, err := u.ToSSOUserTyped()
	if err != nil {
		return nil, err
	}

	return &sso.UserInfo{
		Email:          typed.Email,
		EmployeeID:     typed.Attributes.EmployeeID,
		FamilyName:     typed.LastName,
		FullName:       typed.Attributes.FullName,
		GivenName:      typed.FirstName,
		PhoneNumber:    typed.Attributes.TelephoneNumber,
		Title:          typed.Attributes.Title,
		Username:       typed.Username,
		ThumbnailPhoto: typed.Attributes.ThumbnailPhoto,
		MemberOf:       typed.Attributes.MemberOf,
		OrgUnit:        typed.Attributes.OrgUnit,
		ProxyEmails:    typed.Attributes.ProxyEmails,
	}, nil
}

type SearchUsersResp struct {
	Resources []SSOUser `json:"resources"`
}

func (user *SSOUserTyped) GetFullName() string {
	if user.LastName == "" && user.FirstName == "" {
		return user.Username
	}

	return user.LastName + " " + user.FirstName
}

func GetSsoPersonSchemaProperties() map[string]script.JSONSchemaPropertiesValue {
	return map[string]script.JSONSchemaPropertiesValue{
		"fullname":    {Type: "string", Title: "Полное имя"},
		"username":    {Type: "string", Title: "Логин"},
		"email":       {Type: "string", Title: "email"},
		"mobile":      {Type: "string", Title: "Номер мобильного телефона"},
		"fullOrgUnit": {Type: "string", Title: "Подразделение"},
		"position":    {Type: "string", Title: "Должность"},
		"phone":       {Type: "string", Title: "Телефон"},
		"tabnum":      {Type: "string", Title: "Табельный номер"},
	}
}

func (u SSOUser) ToPerson() (*Person, error) {
	typed, err := u.ToSSOUserTyped()
	if err != nil {
		return nil, err
	}

	return &Person{
		Fullname:    fmt.Sprintf("%s %s", typed.FirstName, typed.LastName),
		Username:    typed.Username,
		Email:       typed.Email,
		Mobile:      typed.Attributes.TelephoneNumber,
		FullOrgUnit: typed.UnitPaths,
		Position:    typed.Attributes.Title,
		Phone:       typed.Attributes.TelephoneNumber,
		Tabnum:      typed.Tabnum,
	}, nil
}

type Person struct {
	Fullname    string `json:"fullname"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	Mobile      string `json:"mobile"`
	FullOrgUnit string `json:"fullOrgUnit"`
	Position    string `json:"position"`
	Phone       string `json:"phone"`
	Tabnum      string `json:"tabnum"`
}
