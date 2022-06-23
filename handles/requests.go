package handles

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
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

func (up *UploadHandler) GetAudioProfile(kusSessionID string) int {

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

func (up *UploadHandler) GetProfile(kusSessionID string, profileKey string) int {
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

func (up *UploadHandler) OAuthToken(uploadContext *CtxResponseWebHook) (string, error) {
	token := ""
	// oauth token
	ttl, err := up.redisClient.TTL("kollus-OAuth-" + up.webHook.OAuthScope).Result()
	if err != nil {
		return "", err
	}
	if ttl == -2 || ttl == 0 || ttl < 100 {
		token, err = up.OAuthTokenAPI(uploadContext.upload_key)
		if err != nil {
			return "", err
		}
	} else {
		token, err = up.redisClient.Get("kollus-OAuth-" + up.webHook.OAuthScope).Result()
		if err != nil {
			return "", err
		}
		if token == "" || token == "nil" {
			token, err = up.OAuthTokenAPI(uploadContext.upload_key)
			if err != nil {
				return "", err
			}
		}
	}
	return token, err
}

func (up *UploadHandler) OAuthTokenAPI(upload_key string) (string, error) {
	OAuthTokenUrl := up.webHook.OAuthTokenAPI
	OAuthTokenUrl = strings.TrimSpace(OAuthTokenUrl)
	data := url.Values{}

	// 추가 전달 되는 값
	data.Set("grant_type", "client_credentials")
	data.Add("client_id", up.webHook.OAuthClientID)
	data.Add("client_secret", up.webHook.OAuthClientSecret)
	data.Add("scope", up.webHook.OAuthScope)

	resp, err := http.PostForm(OAuthTokenUrl, data)
	if err != nil {
		log.Println("[ERROR] ["+upload_key+"] OAuth Access_token after postForm Error, StatusCode = ", resp.StatusCode, time.Now().Format(" [2006/01/02-15:04:05]"))
		return "", err
	} else if resp.StatusCode != http.StatusOK {
		log.Println("[ERROR] ["+upload_key+"] OAuthToken Not found api", time.Now().Format(" [2006/01/02-15:04:05]"))
		return "", errors.New(" Not found api")
	}
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("[ERROR] ["+upload_key+"] OAuth Access_token after ioutil.ReadAll Error", time.Now().Format(" [2006/01/02-15:04:05]"))
		return "", err
	}
	fmt.Println(string(contents))

	obj := oAuthMessage{}
	if err := json.Unmarshal(contents, &obj); err != nil {
		log.Println("[ERROR] ["+upload_key+"] OAuth Access_token, which was unable to parse JSON format Error, StatusCode = ", resp.StatusCode, time.Now().Format(" [2006/01/02-15:04:05]"))
		return "", err
	}
	// redis OAuth token zadd
	err = up.redisClient.Set("kollus-OAuth-"+up.webHook.OAuthScope, obj.AccessToken, time.Duration(obj.ExpiresIn)*time.Second).Err()
	if err != nil {
		log.Println("[ERROR] [" + upload_key + "] OAuth token redis set error")
		return "", err
	}
	return obj.AccessToken, nil
}

func (up *UploadHandler) GetCategoryPath(upload_key string, category_key string, OAuthToken string) (string, error) {
	OAuthTokenUrl := up.webHook.OAuthCategoryAPI
	client := &http.Client{}
	req, err := http.NewRequest("GET", OAuthTokenUrl+"/"+category_key, nil)
	if err != nil {
		log.Println("[ERROR] ["+upload_key+"] GetCategory Path after postForm,"+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+OAuthToken)
	resp, err := client.Do(req)
	if err != nil {
		log.Println("[ERROR] ["+upload_key+"] GetCategory Path after postForm Error, StatusCode = ", resp.StatusCode, time.Now().Format(" [2006/01/02-15:04:05]"))
		return "", err
	} else if resp.StatusCode != http.StatusOK {
		log.Println("[ERROR] ["+upload_key+"] GetCategory Path Not found api", time.Now().Format(" [2006/01/02-15:04:05]"))
		return "", errors.New(" Not found api")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("[ERROR] ["+upload_key+"] GetCategory Path after ioutil.ReadAll Error,"+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		return "", err
	}

	obj := categoryMessage{}
	if err := json.Unmarshal(body, &obj); err != nil {
		log.Println("[ERROR] ["+upload_key+"] GetCategory Path, which was unable to parse JSON format,"+err.Error()+" "+string(body), time.Now().Format(" [2006/01/02-15:04:05]"))
		return "", err
	}
	return obj.Data.Level_Path, nil
}

func (up *UploadHandler) GetAccessToken(upload_key string, access_token string) (string, error) {
	AccessTokenUrl := up.webHook.GetAccessTokenAPI + access_token
	resp, err := http.Get(AccessTokenUrl)
	if err != nil {
		log.Println("[ERROR] ["+upload_key+"] Get Access token, "+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		return "", err
	} else if resp.StatusCode != http.StatusOK {
		log.Println("[ERROR] ["+upload_key+"] Get Access token statusCode != 200, "+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		return "", err
	}
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("[ERROR] ["+upload_key+"] Get Access token body err,"+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		return "", err
	}

	obj := accessTokenMessage{}
	if err := json.Unmarshal(contents, &obj); err != nil {
		log.Println("[ERROR] ["+upload_key+"] Get Access token json parser err."+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		return "", err
	}

	return obj.Result.Key, nil
}
