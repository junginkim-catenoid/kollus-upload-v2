package handles

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"kollus-upload-v2/assembler"
	"kollus-upload-v2/hash"
	"kollus-upload-v2/pkg/config"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type UploadHandler struct {
	redisClient      *redis.Client
	webHook          *config.Configuration
	tempPathContents string
	serverHost       string //Domain 를 사용
	serverPort       string
	processUID       int
	processGID       int
	title            string
	categoryKey      string
	categoryName     string
	desiredPath      string
	accessToken      string
	fileAssembler    *assembler.FileAssembler
}

const (
	FORM_FILE_NAME                         = "upload-file"
	TEMP_CONTENTS_PATH                     = "/tmp/.working"
	KBTYE                                  = 1000
	MGBYTE                                 = 1000000
	MINUTESEC                              = 60
	HOURSEC                                = 3600
	PORCESS_SIMPLE_MULTIPART_BEGINES       = 15
	PORCESS_SIMPLE_MULTIPART_DONE          = 30
	PORCESS_SIMPLE_MULTIPART_FINISHED_REST = 50
	PORCESS_SIMPLE_MULTIPART_FILE_CP_ERROR = 51
)

const (
	FMT_OK0    = "{\"error\": %s,\"message\": \"OK\",\"result\": {\"type\":\"%s\",\"upload_key\": \"%s\",\"progress\": %s}}"
	FMT_ERROR0 = "{\"error\": %s,\"message\": \"%s\",\"result\": {\"type\":\"%s\",\"upload_key\": \"%s\",\"progress\": %s}}"
)

func NewSegmentUploadHandler(redisClient *redis.Client, conf *config.Configuration) (up *UploadHandler) {
	/// "PUT" type upload method
	uid, _ := strconv.Atoi(conf.ProcessUID)
	gid, _ := strconv.Atoi(conf.ProcessGID)

	title := ""
	categoryKey := ""
	categoryName := "_None"
	desiredPath := "kollus"
	//tempPathContents
	accessToken := ""

	log.Println("[INFO] default directory ", TEMP_CONTENTS_PATH)

	return &UploadHandler{redisClient,
		conf,
		TEMP_CONTENTS_PATH,
		conf.UploadHost,
		conf.UploadPort,
		uid,
		gid,
		title,
		categoryKey,
		categoryName,
		desiredPath,
		accessToken,
		assembler.NewFileAssembler(conf.ContentsPath)}
}

func (up *UploadHandler) CreateKollusOneTimeURL(c *gin.Context) {
	//var kollusRequest KollusParam
	accessToken := c.Query("access_token")
	categoryKey := c.PostForm("category_key")
	expireTime := c.PostForm("expire_time")
	isAudioFile := c.PostForm("is_audio_upload")
	title := c.PostForm("title")
	isEncryptionUpload := c.PostForm("is_encryption_upload")

	isPassthrough := c.PostForm("is_passthrough")
	profileKey := c.PostForm("profile_key")

	t := time.Now().UTC()
	currentDate := t.Format("20060102")
	currentH := t.Hour()

	kollusUploadFileKey := currentDate + "-" + hash.ShortUid()
	kusSessionID := "KUS_" + kollusUploadFileKey

	log.Println("[INFO][STEP(1/13)] ["+kusSessionID+"] ", c.Request.PostForm, isAudioFile, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO][STEP(1/13)] ["+kusSessionID+"] CreateKollusOneTimeURL : ", "KUS_"+kollusUploadFileKey, time.Now().Format(" [2006/01/02-15:04:05]"))
	// title이 없으면 파일명으로 대체함.
	if accessToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "1", "message:": "failed to create upload session. invalid request"})
		return
	} else {
		up.accessToken = accessToken
	}
	if isAudioFile == "1" {
		if up.GetAudioProfile(c, kusSessionID) != 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "1", "message:": "The Audio profile key does not exist."})
			return
		}
	} else {
		isAudioFile = "0"
	}
	if expireTime == "" {
		expireTime = "600"
	}
	// category
	if categoryKey == "" {
		up.categoryKey = ""
		up.categoryName = "_None"
	} else {
		up.categoryKey = categoryKey
	}

	// passthrough
	if isPassthrough == "1" {
		if profileKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "1", "message:": "The profile key does not exist."})
			return
		}
		if up.GetProfile(c, kusSessionID, profileKey) != 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "1", "message:": "The profile key does not match."})
			return
		}

	} else {
		urlStr := up.webHook.CreateHookAPI + "?access_token=" + accessToken
		data := url.Values{}
		data.Add("category_key", categoryKey)
		data.Add("expire_time", expireTime)
		data.Add("is_audio_upload", isAudioFile)
		data.Add("is_encryption_upload", isEncryptionUpload)
		data.Add("title", title)
		data.Add("upload_file_key", kollusUploadFileKey)
		data.Add("http_endpoint_domain", up.webHook.ServiceDomain)
		data.Add("created_date", currentDate)
		data.Add("created_hour", strconv.Itoa(currentH))

		fmt.Println(urlStr)

		// creat-url api 를 호출함.
		err := up.kollusAPIRequest(urlStr, data)
		if err != nil {
			log.Println("[ALERT][ERROR] ["+kusSessionID+"] <<CreateKollusOneTimeURL , HMSET  ", kollusUploadFileKey, err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
			c.JSON(http.StatusBadRequest, gin.H{"error": "1", "message:": "failed to create upload session, " + err.Error()})
			return
		}
	}

	//
	// redis에 값설정
	//
	expireTimeNum, err := strconv.Atoi(expireTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "1", "message:": "failed to create upload session, " + err.Error()})
		return
	}

	err = up.redisUploadSession(expireTimeNum, "n", kusSessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "1", "message:": "failed to create upload session, " + err.Error()})
		return
	}

	type (
		ResultMessage struct {
			Error  int `json:"error" binding:"required"`
			Result struct {
				UploadUrl       string `json:"upload_url"`
				ProgressUrl     string `json:"progress_url"`
				UploadFileKey   string `json:"upload_file_key"`
				WillBeExpiredAt string `json:"will_be_expired_at"`
			} `json:"result"`
		}
	)

	fmt.Println("Complete ")

	var UploadURI string
	// passthrough 가 아니면서 Audio 파일일 때
	if isPassthrough != "1" && isAudioFile == "1" {
		UploadURI = up.webHook.ServiceDomain + "/api/v1/UploadMultiParts2/" + kusSessionID + "/" + kollusUploadFileKey
	} else {
		UploadURI = up.webHook.ServiceDomain + "/api/v1/UploadMultiParts/" + kusSessionID + "/" + kollusUploadFileKey
	}
	ProgressURI := up.webHook.ServiceDomain + "/api/v1/GetUploadingProgress/" + kusSessionID

	if isPassthrough == "1" {
		if isEncryptionUpload == "1" {
			UploadURI += "/" + profileKey + "/enc"
		} else {
			UploadURI += "/" + profileKey + "/non_enc"
		}

		if categoryKey != "" {
			UploadURI += "/" + categoryKey
		} else {
			UploadURI += "/none"
		}

		// #93 passthrough 업로드시 계정키에 '-' 가 존재하면 카테고리 에러현상 발생 (카테고리키 버그)
		UploadURI += "/" + accessToken
	} else {
		if isAudioFile == "1" {
			if isEncryptionUpload == "1" {
				UploadURI += "/audio_enc"
			} else {
				UploadURI += "/audio_non_enc"
			}
		}
	}

	//HeaderProto := c.Request.Header["X-Forwarded-Proto"]
	//RRemoteAddr := c.Request.RemoteAddr
	//trustAddr := strings.Split(RRemoteAddr, ":")
	//trial := net.ParseIP(trustAddr[0])
	//ParsingIP := strings.Split(up.webHook.TrustedProxies, "-")
	//ip1 := net.ParseIP(ParsingIP[0])
	//ip2 := net.ParseIP(ParsingIP[1])

	resultMessage := &ResultMessage{Error: 0}
	//if HeaderProto != nil && bytes.Compare(trial, ip1) >= 0 && bytes.Compare(trial, ip2) <= 0 {
	//	resultMessage.Result.UploadUrl = HeaderProto[0] + "://" + UploadURI
	//	resultMessage.Result.ProgressUrl = HeaderProto[0] + "://" + ProgressURI
	//} else {
	//	resultMessage.Result.UploadUrl = "http://" + UploadURI
	//	resultMessage.Result.ProgressUrl = "http://" + ProgressURI
	//}

	resultMessage.Result.UploadUrl = "http://" + UploadURI
	resultMessage.Result.ProgressUrl = "http://" + ProgressURI

	resultMessage.Result.UploadFileKey = kollusUploadFileKey
	tt := time.Now().Add(time.Duration(expireTimeNum) * time.Second).UTC()
	resultMessage.Result.WillBeExpiredAt = tt.String()

	log.Println("[INFO][STEP(3/13)] ["+kusSessionID+"] CreateKollusOneTimeURL : ", kusSessionID, UploadURI, time.Now().Format(" [2006/01/02-15:04:05]"))
	c.JSON(http.StatusOK, resultMessage)

	fmt.Println("UploadURI : ", UploadURI)
}

func (up *UploadHandler) CreateUploadSession(c *gin.Context) {
	var err error
	var expTime int

	expTime, err = strconv.Atoi(c.Param("expTime"))
	// limitaiton for 3 hours.

	uploadType := c.Param("uploadType")

	agent := c.Request.Header.Get("User-Agent")
	log.Println("[INFO][STEP(1/13)] START CREATE UPLOAD SESSION ", agent, time.Now().Format(" [2006/01/02-15:04:05]"))

	rw := c.Writer
	rw.Header().Set("Server", "Catenoied upload server")
	rw.Header().Set("Content-Type", "application/json")

	// prefix 추가
	upload_key := "KUS_" + randStr(32)
	log.Println("[INFO][STEP(1/13)] ["+upload_key+"] CreateUploadSession : ", upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO][STEP(1/13)] ["+upload_key+"] parameter : ", expTime, uploadType, upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))

	if err != nil || expTime <= 0 || expTime > 21600 {
		log.Println("[DEBUG] ["+upload_key+"] <<Expired time , CreateUploadSession ", upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		c.String(http.StatusBadRequest, FMT_ERROR0, "true", "expired time", uploadType, "null", "0")
		return
	}

	if "" == uploadType || ("n" != uploadType && "f" != uploadType) {
		log.Println("[DEBUG] ["+upload_key+"] <<Bad parameters ,CreateUploadSession ", upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		c.String(http.StatusBadRequest, FMT_ERROR0, "true", "bad parameters", uploadType, "null", "0")
		return
	}

	var dat map[string]interface{}
	byt := []byte(`{"d":"` + up.serverHost + `","u":"` + uploadType + `","s":"0"}`)
	if err := json.Unmarshal(byt, &dat); err != nil {
		log.Println("[ERROR] ["+upload_key+"] <<CreateUploadSession , json marshal err.. ", upload_key, err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		c.String(http.StatusInternalServerError, FMT_ERROR0, "true", "InternalServerError", uploadType, "null", "0")
		return
	}
	if err := up.redisClient.HMSet(upload_key, dat).Err(); err != nil {
		log.Println("[ERROR] ["+upload_key+"] <<CreateUploadSession , HMSET  ", upload_key, err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		c.String(http.StatusInternalServerError, FMT_ERROR0, "true", "InternalServerError", uploadType, "null", "0")
		return
	}
	if err := up.redisClient.Expire(upload_key, time.Duration(expTime)*time.Second).Err(); err != nil {
		log.Println("[ERROR] ["+upload_key+"] <<CreateUploadSession , EXPIRE  ", upload_key, err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		c.String(http.StatusInternalServerError, FMT_ERROR0, "true", "InternalServerError", uploadType, "null", "0")
		return
	}
	//Check Exists
	//
	//issue: #44 HMSET 안되는 케이스에 대한 방어 코드
	//
	for i := 0; i <= 6; {
		if exist, err := up.redisClient.Exists(upload_key).Result(); exist == 1 && err == nil {
			break
		}
		log.Println("[INFO][STEP(2/13)] ["+upload_key+"] Crazy REDIS re create session ", i, time.Now().Format(" [2006/01/02-15:04:05]"))

		if i == 6 {
			log.Println("[INFO] ["+upload_key+"] Crazy REDIS error.. ", time.Now().Format(" [2006/01/02-15:04:05]"))
			c.String(http.StatusBadRequest, FMT_ERROR0, "true", "create crazy redis HMSET error.", uploadType, "null", "0")
			return
		}
		time.Sleep(100 * time.Millisecond)
		if err := up.redisClient.HMSet(upload_key, dat).Err(); err != nil {
			log.Println("[ERROR] ["+upload_key+"] <<CreateUploadSession , HMSET  ", upload_key, err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		}
		if err := up.redisClient.Expire(upload_key, time.Duration(expTime)*time.Second).Err(); err != nil {
			log.Println("[ERROR] ["+upload_key+"] <<CreateUploadSession , EXPIRE  ", upload_key, err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		}
		i++
	}

	/// HTML5 upload 방식.
	// rkey : 외부에서 접근 용으로 사용 됩니다, 내부에 파일 패스를 분별할때 사용 됩니다.
	// Redis 의 검색은 upload_key 로 설정 되면 파일 directory 역시 동일 합니다.
	// In cunkfile uploading case,
	// Users make sure pass the upload_key(=rdir)which is used to decide indivisual path.
	// Expire time 은 upload_key 로 진행 합니다. (with REDIS)
	///

	if "f" == uploadType && up.fileAssembler != nil {

		if _, err := up.fileAssembler.CreateSession(upload_key); err != nil {
			log.Println("[ERROR] ["+upload_key+"] <<Creating FileAssembler : ", err.Error(), upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
			//c.JSON(http.StatusInternalServerError, gin.H{"type":uploadType,"upload_key": upload_key,"desc":"Creating session error","status":http.StatusInternalServerError})
			c.String(http.StatusInternalServerError, FMT_ERROR0, "true", "Creating session", uploadType, "null", "0")
			defer up.fileAssembler.CleanupSession(upload_key)
			return
		}
		c.String(http.StatusOK, FMT_OK0, "false", uploadType, upload_key, "0")
		return
	}
	log.Println("[INFO][STEP(3/13)] ["+upload_key+"] CreateUploadSession : ", upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
	c.String(http.StatusOK, FMT_OK0, "false", uploadType, upload_key, "0")

}

func randStr(strSize int) string {

	var dictionary string
	dictionary = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var bytes = make([]byte, strSize)
	rand.Read(bytes)

	for k, v := range bytes {
		bytes[k] = dictionary[v%byte(len(dictionary))]
	}
	return string(bytes)
}
