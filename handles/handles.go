package handles

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"kollus-upload-v2/assembler"
	"kollus-upload-v2/hash"
	"kollus-upload-v2/pkg/config"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

	var UploadURI string
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

	HeaderProto := c.Request.Header["X-Forwarded-Proto"]
	RRemoteAddr := c.Request.RemoteAddr
	trustAddr := strings.Split(RRemoteAddr, ":")
	trial := net.ParseIP(trustAddr[0])
	ParsingIP := strings.Split(up.webHook.TrustedProxies, "-")
	ip1 := net.ParseIP(ParsingIP[0])
	ip2 := net.ParseIP(ParsingIP[1])

	resultMessage := &ResultMessage{Error: 0}
	if HeaderProto != nil && bytes.Compare(trial, ip1) >= 0 && bytes.Compare(trial, ip2) <= 0 {
		resultMessage.Result.UploadUrl = HeaderProto[0] + "://" + UploadURI
		resultMessage.Result.ProgressUrl = HeaderProto[0] + "://" + ProgressURI
	} else {
		resultMessage.Result.UploadUrl = "http://" + UploadURI
		resultMessage.Result.ProgressUrl = "http://" + ProgressURI
	}

	resultMessage.Result.UploadFileKey = kollusUploadFileKey
	tt := time.Now().Add(time.Duration(expireTimeNum) * time.Second).UTC()
	resultMessage.Result.WillBeExpiredAt = tt.String()

	log.Println("[INFO][STEP(3/13)] ["+kusSessionID+"] CreateKollusOneTimeURL : ", kusSessionID, UploadURI, time.Now().Format(" [2006/01/02-15:04:05]"))
	c.JSON(http.StatusOK, resultMessage)
}
