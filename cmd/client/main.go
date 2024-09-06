package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	config "github.com/pircuser61/go_school/config"
	"github.com/pircuser61/go_school/internal/models"
)

type ReqMaterial struct {
	Type    string
	Status  string
	Title   string
	Content string
}
type Response struct {
	Error  bool
	ErrMsg string
}

var sUrl string

func main() {
	var line string

	port := config.GetHttpPort()
	sUrl = "http://127.0.0.1" + port + "/materials"
	in := bufio.NewScanner(os.Stdin)

	for {
		fmt.Println("list | new | set | del | help | quit")

		if !in.Scan() {
			fmt.Println("Scan error")
			continue
		}
		line = in.Text()
		args := strings.Split(line, " ")
		cmd := args[0]
		args = args[1:]
		switch cmd {
		case "й":
			fallthrough
		case "quit":
			fallthrough
		case "q":
			return

		case "list":
			materials(args)
		case "new":
			materialNew(args)
		case "set":
			materialSet(args)
		case "get":
			materialGet(args)
		case "del":
			materialDel(args)
		case "help":
			fmt.Println("new <Тип> <Статус> <Название> <Содержание>")
		default:
			fmt.Printf("Unknown command <%s>\n", cmd)
		}
	}
}

func materials(args []string) {
	base, err := url.Parse(sUrl)
	if err != nil {
		fmt.Println(err)
		return
	}
	reqParams := url.Values{}
	for i := 3; i <= len(args); i += 2 {
		reqParams.Add(args[i-2], args[i-1])
	}
	base.RawQuery = reqParams.Encode()
	fmt.Println(base.String())
	resp, err := http.DefaultClient.Get(base.String())
	if err != nil {
		fmt.Println(err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Println("status code", resp.StatusCode)
	}

	defer resp.Body.Close()
	type ListResponse struct {
		Response
		Body []*models.Material
	}
	var data ListResponse
	if !parse(resp, &data.Response) {
		return
	}

	fmt.Println("materials:")
	for _, val := range data.Body {
		fmt.Println(*val)
	}

}

func materialNew(args []string) {

	var req ReqMaterial
	if len(args) != 4 {
		fmt.Printf("Должно быть ровно 4 параметра")
		fmt.Println(args)
		return
	}

	req.Type = args[0]
	req.Status = args[1]
	req.Title = args[2]
	req.Content = args[3]

	jsonBody, err := json.Marshal(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(jsonBody))
	resp, err := http.DefaultClient.Post(sUrl, "text/json", bytes.NewReader(jsonBody))
	if err != nil {
		fmt.Println(err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Println("status code", resp.StatusCode)
	}

	defer resp.Body.Close()
	type AddRespnose struct {
		Response
		Body any
	}
	var data AddRespnose
	if !parse(resp, &data.Response) {
		return
	}
}

func materialSet(args []string) {
	var req ReqMaterial
	if len(args) != 5 {
		fmt.Printf("Должно быть ровно 5 параметра")
		fmt.Println(args)
		return
	}

	req.Type = args[1]
	req.Status = args[1]
	req.Title = args[3]
	req.Content = args[4]

	jsonBody, err := json.Marshal(req)
	if err != nil {
		fmt.Println(err)
		return
	}

	httpReq, err := http.NewRequest(http.MethodPut, sUrl+"/"+args[1], bytes.NewReader(jsonBody))
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		fmt.Println("Request error", err.Error())
		return
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Println("status code", resp.StatusCode)
	}

	defer resp.Body.Close()
	var data Response
	if !parse(resp, &data) {
		return
	}
}

func materialGet(args []string) {

	if len(args) != 1 {
		fmt.Printf("Должен быть ровно 1 параметр")
		return
	}

	resp, err := http.DefaultClient.Get(sUrl + "/" + args[0])
	if err != nil {
		fmt.Println(err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Println("status code", resp.StatusCode)
	}

	defer resp.Body.Close()
	type GetRespnose struct {
		Response
		Body models.Material
	}
	var data GetRespnose
	if !parse(resp, &data.Response) {
		return
	}
}

func materialDel(args []string) {
	if len(args) != 1 {
		fmt.Printf("Должен быть ровно 1 параметр")
		return
	}

	req, err := http.NewRequest(http.MethodDelete, sUrl+"/"+args[0], nil)
	if err != nil {
		fmt.Println(err.Error())
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Println("status code", resp.StatusCode)
	}

	defer resp.Body.Close()
	var data Response
	if !parse(resp, &data) {
		return
	}
}

func parse(resp *http.Response, data *Response) bool {

	err := json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		fmt.Println("Json parse error:", err)
		return false
	}
	if data.Error {
		fmt.Println("ERROR", data.ErrMsg)
		return false
	}
	return true
}
