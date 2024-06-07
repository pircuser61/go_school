package fileregistry

import (
	"bytes"
	c "context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

const authorizationHeader = "Authorization"

func (s *service) SaveFile(ctx c.Context, token, clientID, name string, file []byte, workNumber string) (string, error) {
	ctxLocal, span := trace.StartSpan(ctx, "file_registry.save_file")
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

	log := logger.GetLogger(ctxLocal).
		WithField("traceID", span.SpanContext().TraceID.String()).WithField("transport", "HTTP")
	reqURL := s.restURL + saveFile
	ctxLocal = script.MakeContextWithRetyrCnt(ctxLocal)

	req, err := retryablehttp.NewRequestWithContext(ctxLocal, http.MethodPost, reqURL, buf)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Work-Number", workNumber)
	req.Header.Set("Clientid", clientID)
	req.Header.Set(authorizationHeader, token)

	resp, err := s.restCli.Do(req)

	attempt := script.GetRetryCnt(ctxLocal) - 1

	if err != nil {
		log.Warning("Pipeliner failed to connect to fileregistry. Exceeded max retry count: ", attempt)

		return "", err
	}

	defer resp.Body.Close()

	if attempt > 0 {
		log.Warning("Pipeliner successfully reconnected to fileregistry: ", attempt)
	}

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
