package fileregistry

type FileInfo struct {
	FileID    string
	Name      string
	CreatedAt string
	Size      int64
}

type AttachInfo struct {
	FileID       string
	Name         string
	CreatedAt    string
	Size         int64
	ExternalLink string
}

type fileID struct {
	Data string `json:"data"`
}
