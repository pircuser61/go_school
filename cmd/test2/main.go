package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

func main() {

	proxyUrl, err := url.Parse("http://iproxy.msk.mts.ru:8088/")

	reqUrl := "https://tabs.mts.ru/fusion/v1/datasheets/dstgybq4VMYjM2X63v/attachments"
	method := "POST"

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	file, errFile1 := os.Open("/Users/snkosya1/Desktop/same_application_requests(exec).har")
	defer file.Close()
	part1,
		errFile1 := writer.CreateFormFile("test", filepath.Base("/Users/snkosya1/Desktop/same_application_requests(exec).har"))
	_, errFile1 = io.Copy(part1, file)
	if errFile1 != nil {
		fmt.Println(errFile1)
		return
	}
	err := writer.Close()
	if err != nil {
		fmt.Println(err)
		return
	}

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		fmt.Println(err)
		return
	}
	req.Header.Add("Authorization", "Bearer uskH9Z9ZlyU8OB1LGHA2DyR")
	req.Header.Add("Cookie", "_first_source=isso-dev.mts.ru/referral; mts_id=368f69ba-bfc4-41a0-a73b-9bbf88bb3c2a; mts_id_last_sync=1666965535; _ga=GA1.2.1304766472.1666965720; adrcid=AzRBTFk-WFZIJxckrXtG22Q")

	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(body))
}
