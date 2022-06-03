package handles

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"mime/multipart"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// limitaiton for 3 hours.
// redis에 upload session를 생성합니다.
// uid : redis session를 구분하는데 사용함.
// uploadtype : f -> post type
// expired time : 0분 < max < 6시간
func (up *UploadHandler) redisUploadSession(expTime int, uploadType string, uid string) error {

	// prefix 추가
	upload_key := uid
	log.Println("[INFO][STEP(2/13)] ["+uid+"] CreateUploadSession : ", upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO][STEP(2/13)] ["+uid+"] parameter : ", expTime, uploadType, upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))

	if expTime <= 0 || expTime > 21600 {
		log.Println("[DEBUG] ["+uid+"] <<Expired time , CreateUploadSession ", upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		return errors.New("expired time")
	}

	if "" == uploadType || ("n" != uploadType && "f" != uploadType) {
		log.Println("[DEBUG] ["+uid+"] <<Bad parameters ,CreateUploadSession ", upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		return errors.New("invalid parmesters")
		//c.String(http.StatusBadRequest, FMT_ERROR0, "true", "bad parameters", uploadType, "null", "0")
	}

	var dat map[string]interface{}
	byt := []byte(`{"d":"` + up.serverHost + `","u":"` + uploadType + `","s":"0"}`)
	if err := json.Unmarshal(byt, &dat); err != nil {
		log.Println("[ERROR] ["+upload_key+"] <<CreateUploadSession , json marshal err.. ", upload_key, err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		return errors.New("redis json marshal error.")
	}
	if err := up.redisClient.HMSet(upload_key, dat).Err(); err != nil {
		log.Println("[ERROR] ["+upload_key+"] <<CreateUploadSession , HMSET  ", upload_key, err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		return errors.New("redis HMSET error.")
	}
	if err := up.redisClient.Expire(upload_key, time.Duration(expTime)*time.Second).Err(); err != nil {
		log.Println("[ERROR] ["+upload_key+"] <<CreateUploadSession , EXPIRE  ", upload_key, err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		return errors.New("redis EXPIRE error.")
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
			return errors.New("create crazy redis HMSET error.")
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
	log.Println("[INFO][STEP(2/13)] ["+uid+"] CreateUploadSession : ", upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
	return nil

}

/*
FileCopy Arguments
1.fileSize 2.Multipart 3.ctxResponseWebHook pointer 4.gin.Context
5.contentPath 6.profileKey 7.encrypKey 8.categorykey
9.destinationDir 10.isCopyPassthrough
*/
func (up *UploadHandler) UploadMultiPartsFileCopy(
	multiPartContentLength int64, part *multipart.Part,
	uploadContext *CtxResponseWebHook, c *gin.Context,
	tmpPath string, uo *UploadOptions,
	isCopyToPassThrough bool) error {
	// 파일이 정상처리후 완료 api 호출 루틴으로 독립쓰레드 추가

	chanCtx := make(chan *CtxResponseWebHook)
	isComplete := make(chan error)

	isCreated := make(chan error)
	chanTempPath := make(chan string)

	go ConfirmedToFiles(chanCtx, isComplete)
	go CreateCompleteFile(chanTempPath, isCreated, uo.uploadFileKey)

	uploadedFileName := part.FileName()

	fUtils := &FileUtils{}

	uploadedFileName = fUtils.ConvertNFCToNFD(uploadedFileName)

	// 확장자 뒤에오는 이상한 문자 제거
	ext := filepath.Ext(uploadedFileName)

	ext = strings.Replace(ext, "%22", "", -1) // " 제거
	ext = strings.Replace(ext, "\"", "", -1)  // " 제거
	ext = strings.Replace(ext, "%27", "", -1) // ' 제거
	ext = strings.Replace(ext, "'", "", -1)   // ' 제거
	ext = strings.Replace(ext, "%20", "", -1) // 공백 제거
	ext = strings.Replace(ext, " ", "", -1)   // 공백 제거
	ext = strings.TrimSpace(ext)              // 좌우 공백 제거

	// 파일 명 재 조립
	uploadedFileName = uploadedFileName[0:len(uploadedFileName)-len(path.Ext(uploadedFileName))] + ext

	uploadContext.file_size = multiPartContentLength
	uploadContext.file_name = uploadedFileName

	/// issue #20
	randFileDirectory := "KUS" + randStr(32)
	randFileName := randStr(32) + "_" + uploadedFileName

	/// Getting the suitable part
	log.Println("[INFO][STEP(5/13)] ["+uploadContext.upload_key+"] FILENAME : "+uploadedFileName+" ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO][STEP(5/13)] ["+uploadContext.upload_key+"] FILESIZE : ", multiPartContentLength, " ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO][STEP(5/13)] ["+uploadContext.upload_key+"] FORMNAME : "+part.FormName()+" ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))

	///
	/// Pre hooking
	/// Retrieves desired path from the RESTAPI
	//passthrough시에는 ftp upload 디렉토리로 넣기 때문에 따로 api 처리하지 않음
	if !isCopyToPassThrough {
		if uo.profileKey != "" {
			uploadContext.profile_key = uo.profileKey
		}
		log.Println("[DEBUG][STEP(6/13)] ["+uploadContext.upload_key+"] start preWebHook  ----- ", uploadContext.upload_key, ext, uo.encryptionKey, time.Now().Format(" [2006/01/02-15:04:05]"))
		if err := up.preWebHook(c, uploadContext); err != nil {
			log.Println("[ERROR] ["+uploadContext.upload_key+"] pre-hook ERROR"+err.Error(), uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
			uploadContext.preHookError = c.Error(err)
			if err := up.redisClient.Expire(uploadContext.upload_key, 0); err != nil {
				log.Println("[ERROR] ["+uploadContext.upload_key+"] Deleteting upload_key was failed in the end of UploadMultiParts ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
			}
			return err
		}

		// audio upload case
		if uo.encryptionKey == "audio_enc" {
			uploadContext.desiredPath += "/_audio_encrypt"
		} else if uo.encryptionKey == "audio_non_enc" {
			uploadContext.desiredPath += "/_audio"
		}

		log.Println("[DEBUG][STEP(6/13)] ["+uploadContext.upload_key+"] end preWebHook -----", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		log.Println("[DEBUG][STEP(7/13)] ["+uploadContext.upload_key+"] trace "+uploadContext.desiredPath, time.Now().Format(" [2006/01/02-15:04:05]"))
	} else {
		categoryName := ""
		// category path 받아오기
		if uo.categoryKey == "none" || uo.categoryKey == "" {
			categoryName = "_None"
		} else {
			OAToken, err := up.OAuthToken(uploadContext)
			if OAToken != "" && err == nil {
				CategoryPath, err := up.GetCategoryPath(c, uploadContext.upload_key, uo.categoryKey, OAToken)
				if CategoryPath != "" && err == nil {
					categoryName = CategoryPath
				} else {
					categoryName = "_None"
				}
			} else {
				log.Println("[ERROR][STEP(5/13)]["+uploadContext.upload_key+"] GET OAuthToken error = ", err, time.Now().Format(" [2006/01/02-15:04:05]"))
			}
		}
		log.Println("[DEBUG][STEP(5/13)]["+uploadContext.upload_key+"] categoryName = ", categoryName, "category_key = ", uo.categoryKey, time.Now().Format(" [2006/01/02-15:04:05]"))

		if isCopyToPassThrough && uo.encryptionKey == "enc" {
			if strings.Contains(uo.profileKey, "mp3") == true {
				uo.desiredPath += "/_audio_passthrough_encrypt/" + categoryName
			} else {
				uo.desiredPath += "/_passthrough_encrypt/" + categoryName
			}
		} else if isCopyToPassThrough && uo.encryptionKey == "non_enc" {
			if strings.Contains(uo.profileKey, "mp3") == true {
				uo.desiredPath += "/_audio_passthrough/" + categoryName
			} else {
				uo.desiredPath += "/_passthrough/" + categoryName
			}
		}
		log.Println("[DEBUG][STEP(7/13)] ["+uploadContext.upload_key+"] trace "+uo.desiredPath, time.Now().Format(" [2006/01/02-15:04:05]"))
	}

	fileDirectory := uploadContext.upload_key
	if "" != uploadContext.desiredPath {
		fileDirectory = uploadContext.desiredPath
	}
	//Create uploading file
	fileName := uploadContext.file_name
	if "" != uploadContext.desiredFileName {
		fileName = uploadContext.desiredFileName
	}

	///1. Create temp directory
	/// issue #20
	if err := os.MkdirAll(up.tempPathContents+"/"+randFileDirectory, 0777); nil != err {
		log.Println("[ERROR] ["+uploadContext.upload_key+"] MkdirAll,permission denied,at the create temprorary directory "+err.Error(), uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		uploadContext.preHookError = c.Error(err)
		return err
	}
	log.Println("[INFO][STEP(9/13)] ["+uploadContext.upload_key+"] Created temporary directory  "+up.tempPathContents+"/"+randFileDirectory+" ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))

	///
	/// File existing checking and add profile key
	///
	/// TODO: 여기서 Profile체크하지 말고 위에서 create url만들때 profile키가 없으면 에러떨구는 구조로 변경.
	//  TODO: profile key가 정확하면 여기서 굳이 체크할 필요없을듯.
	if isCopyToPassThrough {
		path := filepath.Ext(uploadedFileName)

		fileName = strings.Replace(uploadedFileName, path, "", -1) + "_" + uo.profileKey + path
		randFileName = randStr(32) + "_" + fileName

		log.Println("[DEGUG][STEP(8/13)] ["+uploadContext.upload_key+"] FileName ==> ", fileName, time.Now().Format(" [2006/01/02-15:04:05]"))
		log.Println("[DEGUG][STEP(8/13)] ["+uploadContext.upload_key+"] randFileName ==> ", randFileName, time.Now().Format(" [2006/01/02-15:04:05]"))

		if _, err := os.Stat(tmpPath + "/" + uo.desiredPath + "/" + fileName); os.IsExist(err) {
			log.Println("[ERROR] ["+uploadContext.upload_key+"] Existing file with a duplicated file key "+fileName+" "+err.Error()+" ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
			uploadContext.preHookError = c.Error(err)
			return err
		}
		//2. Create directory
		if err := os.MkdirAll(tmpPath+"/"+uo.desiredPath, 0777); nil != err {
			log.Println("[ERROR] ["+uploadContext.upload_key+"] MkdirAll,permission denied,at the UploadMultiParts "+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
			uploadContext.preHookError = c.Error(err)
			return err
		}
		log.Println("[INFO][STEP(9/13)] ["+uploadContext.upload_key+"] Created upload directory  "+tmpPath+"/"+uo.desiredPath+" ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))

		//MAC으로 테스트할경우 해당부분으로 변경해야됨
		//cmd := exec.Command("chown", "-R", "kollus", tmpPath)

		// 라이브환경
		cmd := exec.Command("chown", "-R", "kollus:kollus", tmpPath)

		if err := cmd.Run(); err != nil {
			log.Println("[ERROR] ["+uploadContext.upload_key+"] MkdirAll,permission denied,at the UploadMultiParts "+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
			uploadContext.preHookError = c.Error(err)
			return err
		}
	} else {
		log.Println("[DEGUG][STEP(8/13)] ["+uploadContext.upload_key+"] FileName ==> ", fileName, time.Now().Format(" [2006/01/02-15:04:05]"))
		if uo.profileKey != "" {
			//subFileName := strings.Split(fileName,".")
			//fileName = subFileName[0] + "_" + profile_key + "." + subFileName[len(subFileName)-1]
			//fileDirectory += "/_fileToLive"
			uploadContext.profile_key = uo.profileKey
		}

		if _, err := os.Stat(tmpPath + "/" + fileDirectory + "/" + fileName); os.IsExist(err) {
			log.Println("[ERROR] ["+uploadContext.upload_key+"] Existing file with a duplicated file key "+fileName+" "+err.Error()+" ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
			uploadContext.preHookError = c.Error(err)
			return err
		}
		//2. Create directory
		if err := os.MkdirAll(tmpPath+"/"+fileDirectory, 0777); nil != err {
			log.Println("[ERROR] ["+uploadContext.upload_key+"] MkdirAll,permission denied,at the UploadMultiParts "+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
			uploadContext.preHookError = c.Error(err)
			return err
		}
		log.Println("[INFO][STEP(9/13)] ["+uploadContext.upload_key+"] Created upload directory  "+tmpPath+"/"+fileDirectory+" ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))

		if err := os.Chown(tmpPath+"/"+fileDirectory, up.processUID, up.processGID); err != nil {
			log.Println("[ERROR] ["+uploadContext.upload_key+"] Changing the ownership,at the UploadMultiParts "+fileDirectory+" uid:"+string(up.processUID)+"@gid:"+string(up.processGID)+" "+err.Error()+" ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
			uploadContext.preHookError = c.Error(err)
			return err
		}

	}

	/// Path for Temprorary contents file
	/// issue #20tempDst, err := os.Create(up.tempPathContents + "/" + randFileDirectory + "/" + randFileName)
	tempDst, err := os.Create(up.tempPathContents + "/" + randFileDirectory + "/" + randFileName)
	defer tempDst.Close()
	log.Println("[INFO][STEP(9/13)] ["+uploadContext.upload_key+"] Created temporary full path   "+up.tempPathContents+"/"+randFileDirectory+"/"+randFileName+" ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
	// issue #47
	if err != nil {
		log.Println("[ERROR] ["+uploadContext.upload_key+"] permission denied,at the UploadMultiParts "+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		uploadContext.preHookError = c.Error(err)
		//c.JSON(http.StatusInternalServerError,gin.H{ "upload_key": "nil","desc":"Denied permission ","status": http.StatusInternalServerError})
		return err
	}
	if err := os.Chown(up.tempPathContents+"/"+randFileDirectory, up.processUID, up.processGID); err != nil {
		log.Println("[ERROR] ["+uploadContext.upload_key+"] Changing the ownership,at the UploadMultiParts "+fileDirectory+" uid:"+string(up.processUID)+"@gid:"+string(up.processGID)+" "+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		uploadContext.preHookError = c.Error(err)
		return err
	}
	/// 실패시 관련 디렉토리를 삭제 합니다.
	uploadContext.temprorary_directory = up.tempPathContents + "/" + randFileDirectory

	var read int64
	var written int64
	var p int32
	//var next int32 = 10
	var next int32 = 5
	updateQuery := []string{uploadContext.upload_key, "s", ""}

	/// Data loop
	uploadContext.multipartuploadProcess = PORCESS_SIMPLE_MULTIPART_BEGINES
	length := multiPartContentLength
	errCnt := 0

	buffer := make([]byte, 1024*1024) // 1MByte
	for {
		cBytes, readErr := part.Read(buffer)

		// read Byte Length가 0 이하(0포함)일 경우
		// err count 증가
		// **** 업로드시 업로드하다가 갑자기 끊어졌을때 ****
		if cBytes <= 0 || buffer == nil {
			errCnt++
			log.Println("[ERROR] ["+uploadContext.upload_key+"] cbyte => ", cBytes, time.Now().Format(" [2006/01/02-15:04:05]"))
			log.Println("[ERROR] ["+uploadContext.upload_key+"] errcount => ", errCnt, time.Now().Format(" [2006/01/02-15:04:05]"))
			if buffer == nil {
				log.Println("buffer => null")
			}

		} else {
			errCnt = 0
		}

		if readErr == io.EOF {
			//#issue 53
			if cBytes > 0 {
				log.Println("[DEBUG][STEP(10/13)] ["+uploadContext.upload_key+"] Remained bytes with flag of EOF: ", cBytes, time.Now().Format(" [2006/01/02-15:04:05]"))
				read = read + int64(cBytes)
				writtenBytes, err := tempDst.Write(buffer[0:cBytes])
				if err != nil {
					log.Println("[ERROR] ["+uploadContext.upload_key+"] temp file writing error: ", written, err.Error(), uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
					uploadContext.preHookError = c.Error(err)
					return err
				}
				written = written + int64(writtenBytes)
				uploadContext.file_size = written
			}

			break
		}

		// err count가 연속 10회 이상일 경우 최종 에러 처리
		if readErr != nil || errCnt > 10 {
			log.Println("[ERROR] ["+uploadContext.upload_key+"] Reading unpredictable error ", errCnt, time.Now().Format(" [2006/01/02-15:04:05]"))
			return readErr
		}

		// read Byte Length가 0 이하이면 continue
		if cBytes <= 0 {
			time.Sleep(time.Microsecond * 1)
			continue
		}

		read = read + int64(cBytes)
		writtenBytes, err := tempDst.Write(buffer[0:cBytes])
		if err != nil {
			log.Println("[ERROR] ["+uploadContext.upload_key+"] in writing progress: ", written, err.Error(), uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
			uploadContext.preHookError = c.Error(err)
			return err
		}
		written = written + int64(writtenBytes)
		uploadContext.file_size = written

		p = int32(float32(read) / float32(length) * 100)

		if 0 != p && 0 == p%next {
			updateQuery[2] = strconv.Itoa(int(p))
			if exist, err := up.redisClient.Exists(uploadContext.upload_key).Result(); exist != 1 && err != nil {
				/// "invalid syntax" , HMSET 방어 코드 추가. issue #28
				// "invalid syntax" , 나중에 처리할꺼면 여기 처리하기
				log.Println("[ERROR] ["+uploadContext.upload_key+"] HMSET UPLOADING(temprorary passing) : "+err.Error(), uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
				log.Println("[ERROR] ["+uploadContext.upload_key+"] HMSET UPLOADING(temprorary passing) : ", updateQuery, uploadContext.upload_key, updateQuery[2], "%", time.Now().Format(" [2006/01/02-15:04:05]"))
				time.Sleep(time.Microsecond * 1)
				continue
			}
			var dat map[string]interface{}
			byt := []byte(`{"s":"` + updateQuery[2] + `"}`)
			if err := json.Unmarshal(byt, &dat); err != nil {
				log.Println("[ERROR] ["+uploadContext.upload_key+"] HMSET UPLOADING(json marshal err) ", updateQuery, err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
				time.Sleep(time.Microsecond * 1)
				continue
			}
			if err := up.redisClient.HMSet(uploadContext.upload_key, dat).Err(); err != nil {
				log.Println("[ERROR] ["+uploadContext.upload_key+"] HMSET UPLOADING(HMSET error) ", updateQuery, err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
				time.Sleep(time.Microsecond * 1)
				continue
			}
			log.Println("[INFO][STEP(10/13)] ["+uploadContext.upload_key+"] HMSET PROGRESS QUERY IS ", updateQuery[2], "%", time.Now().Format(" [2006/01/02-15:04:05]"))
			next += 5
		}
	}
	updateQuery[2] = "100"
	log.Printf("[INFO][STEP(10/13)] ["+uploadContext.upload_key+"] UPLOAD SUCCESS : 100 %", time.Now().Format(" [2006/01/02-15:04:05]"))
	var dat map[string]interface{}
	byt := []byte(`{"s":"100"}`)
	if err := json.Unmarshal(byt, &dat); err != nil {
		log.Printf("[INFO] ["+uploadContext.upload_key+"] UPLOAD SUCCESS Error : "+err.Error()+" ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
	}
	if err := up.redisClient.HMSet(uploadContext.upload_key, dat).Err(); err != nil {
		if exist, err := up.redisClient.Exists(uploadContext.upload_key).Result(); exist != 1 && err != nil {
			log.Printf("[INFO][STEP(10/13)] ["+uploadContext.upload_key+"] HMSET DONE : "+err.Error()+" ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
			log.Printf("[INFO][STEP(10/13)] ["+uploadContext.upload_key+"] HMSET COMMAND : HMSET ", updateQuery, uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))

			// issue #51
			for i := 0; i < 3; i++ {
				if exist, err := up.redisClient.Exists(uploadContext.upload_key).Result(); exist == 1 && err == nil {
					uploadContext.preHookError = nil
					break
				}
				time.Sleep(1 * time.Second)

				if err := json.Unmarshal(byt, &dat); err != nil {
					log.Printf("[INFO] ["+uploadContext.upload_key+"] UPLOAD SUCCESS Error : "+err.Error()+" ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
					continue
				}
				if err := up.redisClient.HMSet(uploadContext.upload_key, dat).Err(); err != nil {
					log.Printf("[INFO] ["+uploadContext.upload_key+"] UPLOAD SUCCESS Error : "+err.Error()+" ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
					continue
				}

				uploadContext.preHookError = c.Error(err)
			}
			// issue #51
			if uploadContext.preHookError != nil {
				return err
			}
		}
	}

	log.Println("[DEBUG][STEP(10/13)] ["+uploadContext.upload_key+"] HMSET DONE : ", updateQuery, uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))

	//This loop has no error
	uploadContext.preHookError = nil
	uploadContext.multipartuploadProcess = PORCESS_SIMPLE_MULTIPART_DONE

	chanCtx <- uploadContext

	completeError := <-isComplete

	close(isComplete)

	if completeError != nil {
		log.Println("[ERROR][STEP(10/13)] Uploaded and saved files do not match", uploadContext.temprorary_directory+"/"+uploadContext.file_name, "사이즈", uploadContext.file_size)
		uploadContext.preHookError = completeError
		return completeError
	}

	/// issue #20
	/// 완료된 파일을 사용자 디렉토리로 복사 합니다.

	// passthrough 일때 /tmp_passthrough 로 옮김
	var mvPath string

	if isCopyToPassThrough {
		log.Println("[INFO][STEP(11/13)] ["+uploadContext.upload_key+"] moving file..   ", up.tempPathContents+"/"+randFileDirectory+"/"+randFileName, " ===> "+tmpPath+"/"+uo.desiredPath+"/"+fileName, " upload key "+uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))

		// 추가 transcoding시 파일명 변경
		if uo.trProfileKeys != "" {
			trProfiles := strings.Split(uo.trProfileKeys, ",")

			lastDotIdx := strings.LastIndex(fileName, ".")
			orgFileName := fileName[:lastDotIdx]
			orgFileType := fileName[lastDotIdx:]

			for _, profile := range trProfiles {
				orgFileName += "+" + profile
			}

			fileName = orgFileName + orgFileType

			log.Println("[INFO][STEP(11/13)] ["+fileName+"] Add Transcoding profile_key ", time.Now().Format(" [2006/01/02-15:04:05]"))
		}

		mvPath = tmpPath + "/" + uo.desiredPath + "/" + fileName
		if err := up.mv(tmpPath+"/"+uo.desiredPath+"/"+fileName,
			up.tempPathContents+"/"+randFileDirectory,
			randFileName,
			up.processUID,
			up.processGID); err != nil {
			log.Println("[ERROR] ["+uploadContext.upload_key+"] Moving file ", err.Error()+" ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		}

		if err := up.redisClient.Expire(uploadContext.upload_key, 0); err != nil {
			log.Println("[ERROR][STEP(10/13)] ["+uploadContext.upload_key+"] Deleteting upload_key ", time.Now().Format(" [2006/01/02-15:04:05]"))
		}
	} else {
		log.Println("[INFO][STEP(11/13)] ["+uploadContext.upload_key+"] moving file..   ", up.tempPathContents+"/"+randFileDirectory+"/"+randFileName, " ===> "+tmpPath+"/"+fileDirectory+"/"+fileName, " upload key "+uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		mvPath = tmpPath + "/" + fileDirectory + "/" + fileName
		if err := up.mv(tmpPath+"/"+fileDirectory+"/"+fileName,
			up.tempPathContents+"/"+randFileDirectory,
			randFileName,
			up.processUID,
			up.processGID); err != nil {
			log.Println("[ERROR] ["+uploadContext.upload_key+"] Moving file ", err.Error()+" ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))
		}
	}

	chanTempPath <- mvPath

	moveErr := <-isCreated

	close(isCreated)

	if moveErr != nil {
		log.Println("[ERROR][STEP(11/13)] Fail to Moved File", mvPath)
		uploadContext.preHookError = moveErr
		return moveErr
	}

	log.Println("[INFO][STEP(12/13)] ["+uploadContext.upload_key+"] trace end, process ", uploadContext.upload_key, time.Now().Format(" [2006/01/02-15:04:05]"))

	return nil
}

/*
File Move Arguments
1.Final Destination File Path 2.Temporary Directory Path
3.Temporary File Name 4.Process UID 5.Process GID
*/
func (up *UploadHandler) mv(dst string, src_dir string, src_filename string, uid int, gid int) error {
	src := src_dir + "/" + src_filename

	sFile, err := os.Open(src)
	if err != nil {
		log.Println("[ERROR] file open error", err.Error()+" ", dst)
		return err
	}
	defer sFile.Close()
	dst_file, err := os.Create(dst)
	if err != nil {
		log.Println("[ERROR] file create error", err.Error()+" ", dst)
		return err
	}
	defer dst_file.Close()

	if err := os.Chown(dst, uid, gid); err != nil {
		log.Println("[ERROR] Changing the ownership,at the UploadMultiParts "+"uid:"+string(uid)+"@gid:"+string(gid)+" "+err.Error()+" ", dst)
		return err
	}

	if _, err := io.Copy(dst_file, sFile); err != nil {
		log.Println("[ERROR] file copy error", err.Error()+" ", dst)
		return err
	}

	err = os.RemoveAll(src_dir + "/")
	if err != nil {
		log.Println("[ERROR] mv command file removing error", err.Error()+" ", dst)
		return err
	}
	return nil
}
