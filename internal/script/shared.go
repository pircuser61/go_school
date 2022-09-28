package script

type AccessType string

const (
	NoneAccess      AccessType = "None"
	ReadOnlyAccess  AccessType = "Read"
	ReadWriteAccess AccessType = "ReadWrite"
)

type FormAccessibility struct {
	Id         string     `json:"id"`
	Name       string     `json:"name"`
	AccessType AccessType `json:"accessType"`
}
