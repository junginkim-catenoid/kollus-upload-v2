package handles

import (
	"encoding/json"
	"errors"
	"log"
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
