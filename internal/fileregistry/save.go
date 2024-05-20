package fileregistry

import (
	"bytes"
	c "context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/hashicorp/go-retryablehttp"

	"go.opencensus.io/trace"
)

func (s *Service) SaveFile(ctx c.Context, token, clientID, name string, file []byte, workNumber string) (string, error) {
	ctx, span := trace.StartSpan(ctx, "file_registry.save_file")
	defer span.End()

	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)

	filePart, err := writer.CreateFormFile("file", name)
	if err != nil {
		return "", err
	}

	_, err = filePart.Write(file)
	if err != nil {
		return "", err
	}

	err = writer.Close()
	if err != nil {
		return "", err
	}

	reqURL := s.restURL + saveFile

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodPost, reqURL, buf)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Work-Number", workNumber)
	req.Header.Set("Clientid", clientID)
	req.Header.Set(authorizationHeader, token)

	resp, err := s.restCli.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("got bad status: %s", resp.Status)
	}

	id := fileID{}

	err = json.NewDecoder(resp.Body).Decode(&id)
	if err != nil {
		return "", err
	}

	return id.Data, nil
}
