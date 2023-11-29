package file_registry

type FileInfo struct {
	FileId    string
	Name      string
	CreatedAt string
	Size      int64
}

type AttachInfo struct {
	FileId       string
	Name         string
	CreatedAt    string
	Size         int64
	ExternalLink string
}

type fileID struct {
	Data string `json:"data"`
}
