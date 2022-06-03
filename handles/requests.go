package handles

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

type kollusMessage struct {
	Error   int    `json:"error" binding:"required"`
	Message string `json:"message"`
	Result  struct {
		ErrorCode   int    `json:"error_code,omitempty"`
		ErrorDetail string `json:"error_detail,omitempty"`
	} `json:"result,omitempty"`
}

func (up *UploadHandler) GetAudioProfile(c *gin.Context, kusSessionID string) int {

	resp, err := http.Get(up.webHook.ProfileKeyAPI + up.accessToken)
	if err != nil {
		log.Println("[ERROR] ["+kusSessionID+"] GetAudioProfile get err = ,"+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		return 0
	} else if resp.StatusCode != http.StatusOK {
		log.Println("[ERROR] ["+kusSessionID+"] GetAudioProfile Not found api", time.Now().Format(" [2006/01/02-15:04:05]"))
		return 0
	}
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("[ERROR] ["+kusSessionID+"] GetAudioProfile get err = ,"+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		return 0
	}
	//fmt.Println(string(contents))
	type (
		kollusMessage struct {
			Result struct {
				Count int
				Items []struct {
					Media_Profile_Group_Name string
				}
			}
		}
	)

	obj := &kollusMessage{}
	if err := json.Unmarshal([]byte(contents), obj); err != nil {
		log.Println("[ERROR] ["+kusSessionID+"] GetAudioProfile get err = ,"+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		//c.Error(errors.New("GetAudioProfile get err = " + err.Error()))
		return 0
	}

	for i := 0; i < obj.Result.Count; i++ {
		if "Audio" == obj.Result.Items[i].Media_Profile_Group_Name || "audio" == obj.Result.Items[i].Media_Profile_Group_Name {
			return 1
		}
	}
	return 0
}

func (up *UploadHandler) GetProfile(c *gin.Context, kusSessionID string, profileKey string) int {
	resp, err := http.Get(up.webHook.ProfileKeyAPI + up.accessToken)
	if err != nil {
		log.Println("[ERROR] ["+kusSessionID+"] GetDupKey get err = ,"+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		return 0
	} else if resp.StatusCode != http.StatusOK {
		log.Println("[ERROR] ["+kusSessionID+"] GetProfile Not found api", time.Now().Format(" [2006/01/02-15:04:05]"))
		return 0
	}
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("[ERROR] ["+kusSessionID+"] GetDupKey get err = ,"+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		return 0
	}
	type (
		kollusMessage struct {
			Result struct {
				Count int
				Items []struct {
					Key string
				}
			}
		}
	)

	obj := &kollusMessage{}
	if err := json.Unmarshal([]byte(contents), obj); err != nil {
		log.Println("[ERROR] ["+kusSessionID+"] GetDupKey get err = ,"+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		return 0
	}

	for i := 0; i < obj.Result.Count; i++ {
		if profileKey == obj.Result.Items[i].Key {
			return 1
		}
	}
	return 0
}

func (up *UploadHandler) kollusAPIRequest(url string, data url.Values) error {
	//data url.Values{}
	if url == "" {
		return errors.New("invalid parameter")
	}
	resp, err := http.PostForm(url, data)
	defer resp.Body.Close()
	if err != nil {
		log.Println(err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		return err
	} else if resp.StatusCode != http.StatusOK {
		log.Println("[ERROR] kollusAPIRequest Not found api", time.Now().Format(" [2006/01/02-15:04:05]"))
		return errors.New(", Not found api")
	}
	contents, err := ioutil.ReadAll(resp.Body)

	fmt.Println(string(contents))

	obj := &kollusMessage{}
	dec := json.NewDecoder(bytes.NewReader(contents))
	if err := dec.Decode(&obj); err != nil {
		log.Println(err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		return err
	}

	if 0 != obj.Result.ErrorCode {
		return errors.New(obj.Message)
	}

	return nil
}
