package script

type AccessType string

const (
	NoneAccess      AccessType = "None"
	ReadOnlyAccess  AccessType = "Read"
	ReadWriteAccess AccessType = "ReadWrite"
)

type FormAccessibility struct {
	Id         string     `json:"id"`
	Title      string     `json:"title"`
	AccessType AccessType `json:"accessType"`
}
