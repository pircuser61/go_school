package script

type PlaceholderParams struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (a *PlaceholderParams) Validate() error {
	return nil
}
