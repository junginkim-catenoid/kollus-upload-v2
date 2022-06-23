package handles

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"io"
	"io/ioutil"
	"kollus-upload-v2/assembler"
	"kollus-upload-v2/hash"
	"kollus-upload-v2/pkg/config"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
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

type CtxResponseWebHook struct {
	name                  string
	namePrehook           string
	preHookHttpStatusCode int
	preHookError          error
	desiredPath           string
	desiredFileName       string
	upload_result         string
	last_message          string
	///
	/// KUS, session key
	///
	upload_key string
	// encryption_key string
	/// start time
	preHookStartTime time.Time
	/// file inforamtion
	file_name string
	file_size int64
	fine_info string
	///
	formHiddenValues map[string]string
	/// 임시 컨텐츠 디렉토리 파일
	temprorary_directory string
	/// Process validation of simple multifileUploading such as longterm porcess.
	multipartuploadProcess int

	// Profile_key 일반업로드시 사용하기위해 추가
	profile_key string
}

type UploadOptions struct {
	profileKey    string `default:""`
	encryptionKey string `default:""`
	categoryKey   string `default:""`
	desiredPath   string `default:""`
	trProfileKeys string `default:""`
	uploadFileKey string `default:""`
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
		if up.GetAudioProfile(kusSessionID) != 1 {
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
		if up.GetProfile(kusSessionID, profileKey) != 1 {
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

	err = up.redisUploadSession(c, expireTimeNum, "n", kusSessionID)
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

// UploadMultiParts 파일을 업로드 합니다.
//UploadMultiParts -> UploadMultiPartsFileCopy
//defer endWebHook
func (up *UploadHandler) UploadMultiParts(c *gin.Context) {

	log.Println("[INFO][STEP(4-0/13)]", time.Now().Format(" [2006/01/02-15:04:05]"))
	req := c.Request

	///
	/// END hook process
	//var ctxResponseWebHook *CtxResponseWebHook;
	//마지막 훅이 호출 될때까지의 context 입니다.
	ctxResponseWebHook := CtxResponseWebHook{"", "", 200, nil, "", "", "1", "", "", time.Now(), "", 0, "", make(map[string]string), "", 0, ""}

	upload_key := c.Param("upload_key")
	encryption_key := c.Param("encryption_key")
	profileKey := c.Param("profile_key")

	log.Println("[INFO][STEP(4/13)] ["+upload_key+"] UploadMultiParts, file-key ", upload_key, c.Param("user1"), encryption_key, req, time.Now().Format(" [2006/01/02-15:04:05]"))
	if 0 == len(upload_key) {
		log.Println("[ERROR] ["+upload_key+"] Bad request,at the UploadMultiParts ", upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		ctxResponseWebHook.preHookError = c.Error(errors.New("invalied key name , 'upload_key' "))
		//c.JSON(http.StatusNotFound,gin.H{ "upload_key": "nil","desc":"invalied key name","status": http.StatusNotFound})
		return
	}

	defer up.endWebHook(c, &ctxResponseWebHook, "")

	ctxResponseWebHook.upload_key = upload_key
	if exist, errRedis := up.redisClient.Exists(upload_key).Result(); exist != 1 || errRedis != nil {
		redisRes, errr := up.redisClient.TTL(upload_key).Result()
		log.Println("[INFO] ["+upload_key+"] TTL ", redisRes, errr, time.Now().Format(" [2006/01/02-15:04:05]"))
		log.Println("[INFO] ["+upload_key+"] User tried connection with an invalid upload_key, check your redis server which has the key as : ", upload_key, exist, errRedis, time.Now().Format(" [2006/01/02-15:04:05]"))
		log.Println("[ERROR] ["+upload_key+"] The upload_key has already been uploaded or does not exist. ", upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		ctxResponseWebHook.preHookError = c.Error(errors.New("[WARN] OK, User tried connection with an invalid upload_key"))
		return
	}

	_, err := up.redisClient.HMGet(upload_key, "d", "u").Result()
	if err != nil {
		log.Println("[ERROR] ["+upload_key+"] invalid key-name with the HMGET commands : ", upload_key, err, time.Now().Format(" [2006/01/02-15:04:05]"))
		ctxResponseWebHook.preHookError = c.Error(errors.New("Invalid key-name with the HMGET commands."))
		//c.JSON(http.StatusNotFound,gin.H{ "upload_key": "nil","desc":"invalied key name","status": http.StatusNotFound})
		return
	}
	length := req.ContentLength
	if length <= 0 {
		log.Println("[ERROR] ["+upload_key+"] Bad request,at the UploadMultiParts", time.Now().Format(" [2006/01/02-15:04:05]"))
		ctxResponseWebHook.preHookError = c.Error(errors.New("File contentes lenght 0 or negative"))
		//c.String(http.StatusInternalServerError,FMT_ERROR0,"true","Bad request","n","null","0");
		return
	}

	mr, err := req.MultipartReader()
	if err != nil {
		log.Println("[ERROR] ["+upload_key+"] "+err.Error(), upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		ctxResponseWebHook.preHookError = c.Error(err)
		return
	}

	var CountOfMultipartsFiles uint32 = 0

	// uploadOptions을 struct에 정의
	uploadOptions := &UploadOptions{
		profileKey:    profileKey,
		encryptionKey: encryption_key,
		categoryKey:   "",
		desiredPath:   "",
		trProfileKeys: "",
		uploadFileKey: c.Param("user1"),
	}

	for {
		part, err := mr.NextPart()

		// 그외 에러 처리 루틴 추가(2018. 09. 13 kw.cho)
		if err == io.EOF || err != nil {
			//log.Println("[DEBUG] exit part");
			if nil != part {
				part.Close()
			}
			if err != io.EOF {
				log.Println("[ERROR] ["+upload_key+"] UploadMultiParts NextPart "+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
			}
			break
		}

		//issue #55
		if part != nil && part.FormName() == "accept" {
			buf := new(bytes.Buffer)
			buf.ReadFrom(part)
			log.Println("[INFO] ["+upload_key+"] accept is: ", buf.String(), time.Now().Format(" [2006/01/02-15:04:05]"))
			ctxResponseWebHook.formHiddenValues["accept"] = buf.String()
		}

		if part != nil && part.FormName() == "return_url" {
			buf := new(bytes.Buffer)
			buf.ReadFrom(part)
			log.Println("[INFO] ["+upload_key+"] return_url is: ", buf.String(), time.Now().Format(" [2006/01/02-15:04:05]"))
			ctxResponseWebHook.formHiddenValues["return_url"] = buf.String()
		}

		if part != nil && part.FormName() == "disable_alert" {
			buf := new(bytes.Buffer)
			buf.ReadFrom(part)
			log.Println("[INFO] ["+upload_key+"] disable_alert is: ", buf.String(), time.Now().Format(" [2006/01/02-15:04:05]"))
			ctxResponseWebHook.formHiddenValues["disable_alert"] = buf.String()
		}

		if part != nil && part.FormName() == "redirection_scope" {
			buf := new(bytes.Buffer)
			buf.ReadFrom(part)
			log.Println("[INFO] ["+upload_key+"] redirection_scope is: ", buf.String(), time.Now().Format(" [2006/01/02-15:04:05]"))
			ctxResponseWebHook.formHiddenValues["redirection_scope"] = buf.String()
		}
		/// Kollus 에서 file field 네임을 upload-file 로 fix 함.
		/// Form 영역에서 upload-file은 한번만 처리함. "CountOfMultipartsFiles have to be 0"
		// issue #51
		if part != nil && len(part.FileName()) > 0 && FORM_FILE_NAME == part.FormName() && 0 == CountOfMultipartsFiles {

			err := up.UploadMultiPartsFileCopy(
				req.ContentLength, part,
				&ctxResponseWebHook, c,
				up.webHook.ContentsPath, uploadOptions,
				false)

			if err != nil && err.Error() != "EOF" {
				ctxResponseWebHook.multipartuploadProcess = PORCESS_SIMPLE_MULTIPART_FILE_CP_ERROR
				ctxResponseWebHook.last_message = err.Error()
				//issue #55
				//return
			}
			CountOfMultipartsFiles++
			ctxResponseWebHook.multipartuploadProcess = PORCESS_SIMPLE_MULTIPART_FINISHED_REST
		}

	} //multi part block
}

func (up *UploadHandler) preWebHook(c *gin.Context, ctxResponseWebHook *CtxResponseWebHook) error {
	if ctxResponseWebHook == nil {
		return c.Error(errors.New("Ctx ResponseWebhook is nil "))
	}

	if true == up.webHook.PreHookEnable {
		upload_file_key := c.Param("user1")
		urlStr := up.webHook.PreHookAPI
		urlStr = strings.TrimSpace(urlStr)
		data := url.Values{}

		if "" == upload_file_key || "" == ctxResponseWebHook.file_name {
			return c.Error(errors.New("lack of parameters,c.Param(user1),file_name"))
		}
		c.Request.ParseForm()
		// 추가 전달 되는 값
		data.Set("upload_file_key", upload_file_key)
		data.Add("category_key", up.categoryKey)
		data.Add("uploaded_filename", ctxResponseWebHook.file_name)
		data.Add("return_url", c.Request.FormValue("return_url"))
		data.Add("disable_alert", c.Request.FormValue("disable_alert"))
		data.Add("redirection_scope", c.Request.FormValue("redirection_scope"))
		data.Add("profile_key", ctxResponseWebHook.profile_key)

		log.Println("[DEBUG][STEP(6/13)]", "Normarl Upload Uri  :   ", urlStr, "\nNormal Upload Parameters  :  ", data)

		resp, err := http.PostForm(urlStr, data)
		if err != nil {
			log.Println("[ERROR] ["+c.Param("upload_key")+"] pre Hooking after postForm,"+err.Error()+" ", c.Param("upload_key"), time.Now().Format(" [2006/01/02-15:04:05]"))
			c.Error(errors.New("pre Hooking after postForm," + err.Error()))
			return err
		}
		defer resp.Body.Close()
		contents, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return c.Error(errors.New("pre Hooking," + err.Error()))
		}

		obj := preHookMessage{}
		if err := json.Unmarshal(contents, &obj); err != nil {
			log.Println("[ERROR] ["+c.Param("upload_key")+"] pre Hooking, which was unable to parse JSON format,"+err.Error()+" "+string(contents)+" ", c.Param("upload_key"), time.Now().Format(" [2006/01/02-15:04:05]"))
			return c.Error(errors.New("pre Hooking, which was unable to parse JSON format," + err.Error() + " " + string(contents)))
		}

		if "" != obj.Result.Target && "" != obj.Result.Local.Path && "" != obj.Result.Local.Filename {
			ctxResponseWebHook.desiredPath = obj.Result.Local.Path
			up.desiredPath = obj.Result.Local.Path
			ctxResponseWebHook.desiredFileName = obj.Result.Local.Filename

		} else {
			log.Println("[ERROR] ["+c.Param("upload_key")+"] pre Hooking, which was unable to parse JSON format, target, path , filename"+" ", c.Param("upload_key"), time.Now().Format(" [2006/01/02-15:04:05]"))
			return c.Error(errors.New("pre Hooking, which was unable to parse JSON format, target, path , filename " + string(contents)))
		}

		///
		/// end preWebHookend preWebHook
		///
		log.Println("[DEBUG][STEP(6/13)] ["+c.Param("upload_key")+"] Prehook-response"+string(contents)+" ", c.Param("upload_key"), time.Now().Format(" [2006/01/02-15:04:05]"))
		ctxResponseWebHook.preHookHttpStatusCode = resp.StatusCode
		if resp.StatusCode != 200 {
			log.Println("[DEBUG] ["+c.Param("upload_key")+"] "+string(resp.StatusCode)+" ", c.Param("upload_key")+" ", c.Param("upload_key"), time.Now().Format(" [2006/01/02-15:04:05]"))
			return c.Error(errors.New("StatusCode was not 200"))
		}
	} else {
		c.Error(errors.New("[INFO] [" + c.Param("upload_key") + "] no prehook job"))
	}

	return nil
}

func (up *UploadHandler) endWebHook(c *gin.Context, responseHook *CtxResponseWebHook, desiredPath string) {
	log.Println("[DEBUG][STEP(13/13)] ["+c.Param("upload_key")+"] start EndHook", time.Now().Format(" [2006/01/02-15:04:05]"))
	if nil == responseHook {
		log.Println("[ERROR] ["+c.Param("upload_key")+"] pre hook nil pointer ", time.Now().Format(" [2006/01/02-15:04:05]"))
		c.Error(errors.New("pre hook nil pointer "))
		return
	}

	/// last logging
	defer func() {
		errorType := "INFO"
		clientIP := c.ClientIP()
		if 1 == strings.Count(c.ClientIP(), ":") {
			clientIP = strings.Split(c.ClientIP(), ":")[0]
		}

		clientType := c.ContentType()
		method := c.Request.Method
		uploadResult := "OK"
		comment := " "
		if responseHook.preHookError != nil {
			uploadResult = "FAILED"
			errorType = "ERROR"
			comment = c.Errors.ByType(gin.ErrorTypeAny).String()
			//ctxResponseWebHook.preHookError = c.Error(errors.New("invalied key name , 'upload_key' "))
			//c.Writer.WriteHeader(http.StatusBadRequest)
		}

		/// Checking abnormal disconnection, requests and so on.
		if responseHook.multipartuploadProcess < PORCESS_SIMPLE_MULTIPART_BEGINES {
			uploadResult = "INVALID_REQUESTS"
			errorType = "WARN"
			comment = comment + " : invalid requests " + strconv.Itoa(responseHook.multipartuploadProcess) + " "
			responseHook.preHookError = c.Error(errors.New("INVALID_REQUESTS"))
			/// result 에 대해 해더를 변경함.
			//c.Writer.WriteHeader(http.StatusRequestTimeout)

			/// 업로드 중 에러 발생시, 관련 파일 삭제
		} else if responseHook.multipartuploadProcess < PORCESS_SIMPLE_MULTIPART_DONE {
			uploadResult = "ABNORMAL_DISCONNECTION"
			errorType = "WARN"
			comment = comment + " : abnormal disconnected from the client " + strconv.Itoa(responseHook.multipartuploadProcess) + " "

			responseHook.preHookError = c.Error(errors.New("ABNORMAL_DISCONNECTION"))
			/// Removes temprorary directory on the abnormal disconnection.
			if responseHook.temprorary_directory != "" {
				err := os.RemoveAll(responseHook.temprorary_directory + "/")
				if err != nil {
					log.Println("[DEBUG] ["+c.Param("upload_key")+"] Remove a temprorary directory on the abnormal disconnection", err.Error()+" ", responseHook.temprorary_directory+"/", time.Now().Format(" [2006/01/02-15:04:05]"))
				}

			}
		}

		/// ERROR 발생시 삭제
		if errorType == "ERROR" && responseHook.temprorary_directory != "" {
			err := os.RemoveAll(responseHook.temprorary_directory + "/")
			if err != nil {
				log.Println("[DEBUG] ["+c.Param("upload_key")+"] Remove a temprorary directory", err.Error()+" ", responseHook.temprorary_directory+"/", time.Now().Format(" [2006/01/02-15:04:05]"))
			}
		}
		statusCode := c.Writer.Status()
		path := c.Request.URL.Path
		end := time.Now()
		latency := end.Sub(responseHook.preHookStartTime)

		log.Println("\n")
		log.Printf("[KUS][%s][STEP(13/13)] | %v | %s | %s | %d | %s | %d | %s | %s | %s | %s | %s | %d | %s | %s | %s | %s\n",
			errorType,
			end.Format("2006/01/02 - 15:04:05"),
			clientIP,
			clientType,
			//latency.Nanoseconds()/1e6,
			latency.Nanoseconds()/int64(time.Millisecond),
			method,
			statusCode,
			uploadResult,
			responseHook.desiredFileName,
			responseHook.desiredPath,
			desiredPath,
			responseHook.file_name,
			responseHook.file_size,
			path,
			comment,
			time.Now().Format(" [2006/01/02-15:04:05]"),
			c.Request.Header.Get("User-Agent"))
		log.Println("\n")
	}()

	if true == up.webHook.EndHookEnable {
		upload_file_key := c.Param("user1")
		urlStr := up.webHook.EndHookAPI
		data := url.Values{}

		accepted := c.Request.Header.Get("accept")
		if strings.Contains(accepted, "application/json") {
			accepted = "application/json"
		}

		log.Println("[DEBUG] ", accepted)

		data.Set("accept", accepted)
		data.Set("content-type", c.Request.Header.Get("content-type"))
		data.Set("upload_file_key", upload_file_key)
		if responseHook.preHookError == nil {
			/// OK ,
			data.Add("upload_result", "1")
		} else {
			/// ERROR , 오류 전달
			data.Add("upload_result", "0")
			c.Writer.WriteHeader(http.StatusNotFound)
		}

		/// bypass parameters
		/// body 'accept' 우선 함
		for key, value := range responseHook.formHiddenValues {
			if key == "accept" {
				data.Set(key, value)
			} else {
				data.Add(key, value)
			}

		}
		//data.Add("return_url",c.Request.FormValue("return_url"))
		//data.Add("disable_alert",c.Request.FormValue("disable_alert"))
		//data.Add("redirection_scope",c.Request.FormValue("redirection_scope"))
		log.Println("[DEBUG][STEP(13/13)] ["+c.Param("upload_key")+"] user request and it will request to the php ", data, time.Now().Format(" [2006/01/02-15:04:05]"))

		resp, err := http.PostForm(urlStr, data)
		if err != nil {
			log.Println("[ERROR] last Hooking "+err.Error(), c.Param("upload_key"), upload_file_key)
			responseHook.preHookError = c.Error(err)
			c.JSON(http.StatusNotFound, gin.H{"error": 1, "message": "End of WEB-hooking was failed "})
			return
		}

		defer resp.Body.Close()

		// last hooking 결과에 따른 output을 설정 한다.

		contents, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println("[ERROR] ["+c.Param("upload_key")+"] last Hooking "+err.Error(), c.Param("upload_key"), upload_file_key, time.Now().Format(" [2006/01/02-15:04:05]"))
			responseHook.preHookError = c.Error(err)
			c.JSON(http.StatusNotFound, gin.H{"error": 1, "message": "End of WEB-hooking was failed on the reading "})
			return
		}
		log.Println("[INFO][STEP(13/13)] ["+c.Param("upload_key")+"] Result of last hook: "+urlStr, c.Param("upload_key"), upload_file_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		log.Println("[INFO][STEP(13/13)] ["+c.Param("upload_key")+"] Result of last hook: "+data.Encode(), c.Param("upload_key"), upload_file_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		//{"error":0,"message":"Successfully uploaded.","result":{"content_type":"text\/html","body":"<script>alert('Successfully uploaded.');<\/script>"}}

		obj := endHookMessage{}

		if err := json.Unmarshal(contents, &obj); err != nil {
			log.Println("[ERROR] ["+c.Param("upload_key")+"] JSON Parsing Data  :  ", string(contents), time.Now().Format(" [2006/01/02-15:04:05]"))
			log.Println("[ERROR] ["+c.Param("upload_key")+"] last Hooking,which was unable to parse JSON format,"+err.Error(), c.Param("upload_key"), upload_file_key, time.Now().Format(" [2006/01/02-15:04:05]"))
			responseHook.preHookError = c.Error(err)
		}

		if 0 != obj.Error {
			log.Println("[ERROR] ["+c.Param("upload_key")+"] User's last Hook API", c.Param("upload_key"), upload_file_key, time.Now().Format(" [2006/01/02-15:04:05]"))
			return
		}

		log.Println("[DEBUG][STEP(13/13)] ["+c.Param("upload_key")+"] Result  ", obj.Result.Content_type, time.Now().Format(" [2006/01/02-15:04:05]"))
		//
		//Setting header
		//
		c.Writer.Header().Set("Server", "Catenoied upload service")

		//
		// 정상 업로드일 경우 cache 설정 : Browser 재진입 문제
		//
		if responseHook.multipartuploadProcess == PORCESS_SIMPLE_MULTIPART_FINISHED_REST {
			log.Println("[DEBUG][STEP(13/13)] ["+c.Param("upload_key")+"] this upload porcess is OK", time.Now().Format(" [2006/01/02-15:04:05]"))
			//cacheSince := time.Now().Format(http.TimeFormat)
			//func (t Time) AddDate(years int, months int, days int) Time
			cacheUntil := time.Now().AddDate(1, 0, 0).Format(http.TimeFormat)

			//c.Writer.Header().Set("Cache-Control", "max-age:290304000, public")
			//c.Writer.Header().Set("Last-Modified", cacheSince)
			c.Writer.Header().Set("Expires", cacheUntil)
		}
		// 사용자 content-type 채크, 아래 케이스는 무조건 통과 시킴.
		if obj.Result.Content_type == "text/html" {
			log.Println("[DEBUG][STEP(13/13)] ["+c.Param("upload_key")+"] content-type ", obj.Result.Content_type, time.Now().Format(" [2006/01/02-15:04:05]"))
			c.Writer.Header().Set("Server", "Catenoied upload service")
			//c.Writer.Header().Del("content-type")
			//c.Writer.Header().Set("content-type", resp.Header.Get("content-type"))
			c.Writer.Header().Set("content-type", obj.Result.Content_type)
			fmt.Fprint(c.Writer, obj.Result.Body)
			return
		}
		// 사용자가 content-type application 인 경우 처리
		// process status 변경
		if responseHook.multipartuploadProcess < PORCESS_SIMPLE_MULTIPART_FINISHED_REST {
			c.Writer.WriteHeader(http.StatusNotFound)
			//c.Writer.Header().Set("content-type", obj.Result.Content_type)
			c.Writer.Header().Set("content-type", c.Request.Header.Get("accept"))
			fmt.Fprint(c.Writer, "{\"error\": 1, \"message\": \"User request was expired.\"}")
			c.Writer.Flush()

			return
		}

		// issue #47
		if responseHook.multipartuploadProcess == PORCESS_SIMPLE_MULTIPART_FILE_CP_ERROR {
			c.Writer.WriteHeader(http.StatusBadRequest)
			c.Writer.Header().Set("content-type", c.Request.Header.Get("accept"))
			//fmt.Fprint(c.Writer, "{\"error\": 1, \"message\": \"Bad file requested \"}")
			fmt.Fprintf(c.Writer, "{\"error\": 1, \"message\": \"%s \"}", responseHook.last_message)
			c.Writer.Flush()

			return
		}

		c.Writer.Header().Set("content-type", obj.Result.Content_type)
		fmt.Fprint(c.Writer, obj.Result.Body)
		return
	}
	return
}

func (up *UploadHandler) GetUploadingProgress(c *gin.Context) {
	rw := c.Writer
	rw.Header().Set("Server", "Catenoied upload service")
	rw.Header().Set("Content-Type", "application/json")

	upload_key := c.Param("upload_key")
	if 0 == len(upload_key) {
		c.JSON(http.StatusBadRequest, gin.H{"error": 1, "message": "invalied upload_key"})
		return
	}
	//redisExist, err := up.redisClient.Exists(upload_key).Result()
	//if err != nil || redisExist == 0 {
	//	c.JSON(http.StatusNotFound, gin.H{"error": 1, "message": "Upload_key does not exist."})
	//	return
	//}
	redisRes, err := up.redisClient.HMGet(upload_key, "s").Result()
	if redisRes[0] == nil || err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": 1, "message": "Upload_key does not exist."})
		return
	}
	/// please use this tool  !!! http://json2struct.mervine.net/

	var msg struct {
		Error   int    `json:"error"`
		Message string `json:"message"`
		Result  struct {
			Progress int `json:"progress"`
		} `json:"result"`
	}

	RedisProgress, _ := strconv.Atoi(redisRes[0].(string))
	msg.Error = 0
	msg.Message = "OK"
	msg.Result.Progress = RedisProgress
	c.JSON(http.StatusOK, msg)
}

func (up *UploadHandler) UploadMultiPartsPassThrough(c *gin.Context) {

	req := c.Request

	///
	/// END hook process
	//var ctxResponseWebHook *CtxResponseWebHook;
	//마지막 훅이 호출 될때까지의 context 입니다.
	ctxResponseWebHook := CtxResponseWebHook{"", "", 200, nil, "", "", "1", "", "", time.Now(), "", 0, "", make(map[string]string), "", 0, ""}

	upload_key := c.Param("upload_key")
	profile_key := c.Param("profile_key")
	encryption_key := c.Param("encryption_key")
	category_key := c.Param("category_key")
	access_token := c.Param("access_token")

	// 추가 트랜스코딩 관련 추가
	trProfileKeys := c.Param("tr_profile_key")

	log.Println("[INFO][STEP(4/13)] ["+upload_key+"] UploadMultiPartsPassThrough, ", upload_key, " , ", profile_key, " , ", category_key, " , ", access_token, " , ", c.Param("user1"), encryption_key, req, time.Now().Format(" [2006/01/02-15:04:05]"))
	if 0 == len(upload_key) || 0 == len(profile_key) || 0 == len(access_token) {
		log.Println("[ERROR] ["+upload_key+"] Bad request,at the UploadMultiPartsPassThrough ", upload_key, " , ", profile_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		ctxResponseWebHook.preHookError = c.Error(errors.New("invalied key name , 'upload_key' or 'profile_key' "))
		//c.JSON(http.StatusNotFound,gin.H{ "upload_key": "nil","desc":"invalied key name","status": http.StatusNotFound})
		return
	}
	//s1 := strings.Split(profile_key, "-")
	AccessToken, err := up.GetAccessToken(upload_key, access_token)
	if err != nil {
		log.Println("[ERROR] ["+upload_key+"] get access token api error, ", upload_key, err, time.Now().Format(" [2006/01/02-15:04:05]"))
		ctxResponseWebHook.preHookError = c.Error(errors.New("Get access token api error."))
		//c.JSON(http.StatusNotFound,gin.H{ "upload_key": "nil","desc":"invalied key name","status": http.StatusNotFound})
		return
	}
	desiredPath := AccessToken
	defer up.endWebHook(c, &ctxResponseWebHook, desiredPath)

	ctxResponseWebHook.upload_key = upload_key
	if exist, errRedis := up.redisClient.Exists(upload_key).Result(); exist != 1 || errRedis != nil {
		redisRes, errr := up.redisClient.TTL(upload_key).Result()
		log.Println("[INFO] ["+upload_key+"] TTL ", redisRes, errr, time.Now().Format(" [2006/01/02-15:04:05]"))
		log.Println("[INFO] ["+upload_key+"] User tried connection with an invalid upload_key, check your redis server which has the key as : ", upload_key, exist, errRedis, time.Now().Format(" [2006/01/02-15:04:05]"))
		log.Println("[ERROR] ["+upload_key+"] The upload_key has already been uploaded or does not exist. ", upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		ctxResponseWebHook.preHookError = c.Error(errors.New("[WARN] OK, User tried connection with an invalid upload_key"))
		return
	}

	_, err = up.redisClient.HMGet(upload_key, "d", "u").Result()
	if err != nil {
		log.Println("[ERROR] ["+upload_key+"] invalid key-name with the HMGET commands : ", upload_key, err, time.Now().Format(" [2006/01/02-15:04:05]"))
		ctxResponseWebHook.preHookError = c.Error(errors.New("Invalid key-name with the HMGET commands."))
		//c.JSON(http.StatusNotFound,gin.H{ "upload_key": "nil","desc":"invalied key name","status": http.StatusNotFound})
		return
	}
	length := req.ContentLength
	if length <= 0 {
		log.Println("[ERROR] ["+upload_key+"] Bad request,at the UploadMultiParts", time.Now().Format(" [2006/01/02-15:04:05]"))
		ctxResponseWebHook.preHookError = c.Error(errors.New("File contentes lenght 0 or negative"))
		//c.String(http.StatusInternalServerError,FMT_ERROR0,"true","Bad request","n","null","0");
		return
	}

	mr, err := req.MultipartReader()
	if err != nil {
		log.Println("[ERROR] ["+upload_key+"] "+err.Error(), upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		ctxResponseWebHook.preHookError = c.Error(err)
		return
	}

	var CountOfMultipartsFiles uint32 = 0

	// uploadOptions을 struct에 정의
	uploadOptions := &UploadOptions{
		profileKey:    profile_key,
		encryptionKey: encryption_key,
		categoryKey:   category_key,
		desiredPath:   desiredPath,
		trProfileKeys: trProfileKeys,
		uploadFileKey: c.Param("user1"),
	}

	for {
		part, err := mr.NextPart()

		// 그외 에러 처리 루틴 추가(2018. 09. 13 kw.cho)
		if err == io.EOF || err != nil {
			//log.Println("[DEBUG] exit part");
			if nil != part {
				part.Close()
			}
			if err != io.EOF {
				log.Println("[ERROR] ["+upload_key+"] UploadMultiParts NextPart "+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
			}
			break
		}

		//issue #55
		if part != nil && part.FormName() == "accept" {
			buf := new(bytes.Buffer)
			buf.ReadFrom(part)
			log.Println("[INFO] ["+upload_key+"] accept is: ", buf.String(), time.Now().Format(" [2006/01/02-15:04:05]"))
			ctxResponseWebHook.formHiddenValues["accept"] = buf.String()
		}

		if part != nil && part.FormName() == "return_url" {
			buf := new(bytes.Buffer)
			buf.ReadFrom(part)
			log.Println("[INFO] ["+upload_key+"] return_url is: ", buf.String(), time.Now().Format(" [2006/01/02-15:04:05]"))
			ctxResponseWebHook.formHiddenValues["return_url"] = buf.String()
		}

		if part != nil && part.FormName() == "disable_alert" {
			buf := new(bytes.Buffer)
			buf.ReadFrom(part)
			log.Println("[INFO] ["+upload_key+"] disable_alert is: ", buf.String(), time.Now().Format(" [2006/01/02-15:04:05]"))
			ctxResponseWebHook.formHiddenValues["disable_alert"] = buf.String()
		}

		if part != nil && part.FormName() == "redirection_scope" {
			buf := new(bytes.Buffer)
			buf.ReadFrom(part)
			log.Println("[INFO] ["+upload_key+"] redirection_scope is: ", buf.String(), time.Now().Format(" [2006/01/02-15:04:05]"))
			ctxResponseWebHook.formHiddenValues["redirection_scope"] = buf.String()
		}
		/// Kollus 에서 file field 네임을 upload-file 로 fix 함.
		/// Form 영역에서 upload-file은 한번만 처리함. "CountOfMultipartsFiles have to be 0"
		// issue #51
		if part != nil && len(part.FileName()) > 0 && FORM_FILE_NAME == part.FormName() && 0 == CountOfMultipartsFiles {

			err := up.UploadMultiPartsFileCopy(
				req.ContentLength, part,
				&ctxResponseWebHook, c,
				up.webHook.ContentsPassthroughPath, uploadOptions,
				true)
			if err != nil && err.Error() != "EOF" {
				ctxResponseWebHook.multipartuploadProcess = PORCESS_SIMPLE_MULTIPART_FILE_CP_ERROR
				ctxResponseWebHook.last_message = err.Error()
				//issue #55
				//return
			}
			CountOfMultipartsFiles++
			ctxResponseWebHook.multipartuploadProcess = PORCESS_SIMPLE_MULTIPART_FINISHED_REST
		}

	} //multi part block
}

//func (up *UploadHandler)SgHandleFilePut(rw http.ResponseWriter, req *http.Request) {
func (up *UploadHandler) UploadFileChunk(c *gin.Context) {

	upload_key := c.Param("upload_key")
	offset := c.Param("offset")
	req := c.Request
	rw := c.Writer

	if origin := req.Header.Get("Origin"); origin != "" {
		rw.Header().Set("Access-Control-Allow-Origin", origin)
		//rw.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		rw.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT")
		//rw.Header().Set("Access-Control-Allow-Headers",
		//    "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	}
	// Stop here if its Preflighted OPTIONS request
	if req.Method == "OPTIONS" {
		return
	}

	defer req.Body.Close()

	if "" == upload_key || "" == offset {
		c.JSON(http.StatusNotFound, gin.H{"desc": "Invalid keys", "status": http.StatusNotFound})
		return
	}

	var (
		sess *assembler.Session
		err  error
	)

	sess = up.fileAssembler.GetSession(upload_key)
	if sess == nil {
		c.JSON(http.StatusNotFound, gin.H{"desc": "Invalid the file_key", "status": http.StatusNotFound})
		//sendHttpResponse(rw, HttpResp{code: http.StatusNotFound})
		return
	}

	///
	/// 현재 offset(io written) 과 일치 않을시 서버의 진행중인 offset 전송
	///
	if sess.GetOffsetStr() != offset {
		c.JSON(http.StatusBadRequest, gin.H{"desc": "offset not matched", "upload_key": upload_key, "status": http.StatusBadRequest})
		return
	}

	//log.Println("[DEBUG] requested PUT")
	if err = sess.Put(req.Body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"desc": "append error " + err.Error(), "status": http.StatusBadRequest})
		return
	}

	c.JSON(http.StatusOK, gin.H{"desc": "OK", "upload_key": upload_key, "status": http.StatusOK})
}

//func (up *UploadHandler)SgHandleCommit(rw http.ResponseWriter, req *http.Request) {
func (up *UploadHandler) UploadChunkCommit(c *gin.Context) {
	upload_key := c.Param("upload_key")
	sess := up.fileAssembler.GetSession(upload_key)
	fileName := c.Param("fileName")

	req := c.Request
	rw := c.Writer
	if origin := req.Header.Get("Origin"); origin != "" {
		rw.Header().Set("Access-Control-Allow-Origin", origin)
		//rw.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		rw.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT")
		//rw.Header().Set("Access-Control-Allow-Headers",
		//    "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	}
	// Stop here if its Preflighted OPTIONS request
	if req.Method == "OPTIONS" {
		return
	}

	if "" == upload_key || "" == fileName {
		c.JSON(http.StatusNotFound, gin.H{"desc": "Invalid keys", "status": http.StatusNotFound})
		return
	}

	if sess == nil {
		c.JSON(http.StatusNotFound, gin.H{"desc": "session not found,(expired)", "status": http.StatusNotFound})
		return
	}

	fpath := path.Join(up.fileAssembler.GetPath()+"/SG_"+upload_key, fileName)
	if err := os.MkdirAll(up.fileAssembler.GetPath()+"/SG_"+upload_key, 0755); nil != err {
		c.JSON(http.StatusInternalServerError, gin.H{"desc": "Denied permission ", "upload_key": upload_key, "file_name": fileName, "status": http.StatusInternalServerError})
		return
	}

	if err := sess.Commit(fpath); err != nil {
		log.Println("[ERROR] Commit:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"desc": "cmmit error", "upload_key": upload_key, "file_name": fileName, "status": http.StatusInternalServerError})
		return
	} else {
		up.fileAssembler.CleanupSession(sess.GetID())
		log.Printf("[INFO] Session %s closed\n", sess.GetID())
		c.JSON(http.StatusOK, gin.H{"desc": "OK", "upload_key": upload_key, "file_name": fileName, "status": http.StatusOK})
	}
}
