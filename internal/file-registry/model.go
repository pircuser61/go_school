package file_registry

type FileInfo struct {
	FileId    string
	Name      string
	CreatedAt string
	Size      int64
}

type fileID struct {
	Data string `json:"data"`
}
