package script

type AccessType string

const (
	NoneAccess      AccessType = "None"
	ReadOnlyAccess  AccessType = "Read"
	ReadWriteAccess AccessType = "ReadWrite"
)

type FormAccessibility struct {
	NodeId      string     `json:"node_id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	AccessType  AccessType `json:"accessType"`
}
