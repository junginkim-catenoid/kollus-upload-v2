package config

import (
	"github.com/kelseyhightower/envconfig"
	"os"
	"strconv"
)

type Configuration struct {
	UploadHost         string `envconfig:"KUS_UPLOAD_HOST"`
	UploadPort         string `envconfig:"KUS_UPLOAD_PORT"`
	RedisSentinelHost  string `envconfig:"KUS_REDIS_SENTINEL_HOST"`
	RedisSentinelPort  string `envconfig:"KUS_REDIS_SENTINEL_PORT"`
	RedisMasters       string `envconfig:"KUS_REDIS_MASTERS"`
	ServiceDomain      string `envconfig:"KUS_SERVICE_DOMAIN"`
	UploadPublicIP     string `envconfig:"KUS_UPLOAD_PUBLIC_IP"`
	TrustedProxies     string `envconfig:"KUS_TRUSTED_PROXIES"`
	TrustedProxiesLast string `envconfig:"KUS_TRUSTED_PROXIES_LAST"`
	/// basic local contents path.
	ContentsPath            string `envconfig:"KUS_CONTENTS_PATH"`
	ContentsPassthroughPath string `envconfig:"KUS_CONTENTS_PASSTHROUGH_PATH"`
	ProductionMode          string `envconfig:"KUS_PRODUCTION_MODE"`
	ProcessUID              string `envconfig:"KUS_PROCESS_UID"`
	ProcessGID              string `envconfig:"KUS_PROCESS_GID"`
	RedisPoolSize           int    `envconfig:"KUS_REDIS_POOL_SIZE"`
	KUS_TEMPDIR_SCAN_MIN    int    `envconfig:"KUS_TEMPDIR_SCAN_MIN"`
	KUS_TEMPDIR_REMOVE_HOUR int    `envconfig:"KUS_TEMPDIR_REMOVE_HOUR"`

	LogPath string `envconfig:"KUS_LOG_PATH"`
	// crete-hook
	CreateHookAPI     string `envconfig:"KUS_CREATE_H_API"`
	GetAccessTokenAPI string `envconfig:"KUS_GET_ACCESS_TOKEN_API"`
	// OAuth2
	OAuthTokenAPI     string `envconfig:"KUS_OAUTH_TOKEN_API"`
	OAuthClientID     string `envconfig:"KUS_OAUTH_CLIENT_ID"`
	OAuthClientSecret string `envconfig:"KUS_OAUTH_CLIENT_SECRET"`
	OAuthScope        string `envconfig:"KUS_OAUTH_SCOPE"`
	OAuthCategoryAPI  string `envconfig:"KUS_OAUTH_CATEGORY_API"`
	// duplicate_block_key
	DupKeyAPI string `envconfig:"KUS_DUP_API"`
	// profile_key
	ProfileKeyAPI string `envconfig:"KUS_PROFILE_API"`
	// Pre-hook
	PreHookEnable      bool   `envconfig:"KUS_PRE_H_ENABLE"`
	PreHookContentType string `envconfig:"KUS_PRE_H_CONTENT_TYPE"`
	PreHookMethod      string `envconfig:"KUS_PRE_H_METHOD"`
	PreHookAPI         string `envconfig:"KUS_PRE_H_API"`
	// End-hook
	EndHookEnable      bool   `envconfig:"KUS_END_H_ENABLE"`
	EndHookContentType string `envconfig:"KUS_END_H_CONTENT_TYPE"`
	EndHookMethod      string `envconfig:"KUS_END_H_METHOD"`
	EndHookAPI         string `envconfig:"KUS_END_H_API"`
}

// 환경 변수 설정
//- UploadHost=127.0.0.1
//- UploadPort=4242
//- RedisSentinelHost=192.168.56.109
//- RedisSentinelPort=16379
//- RedisMasters=kollus
//- ProductionMode=debug
//- ProcessUID=1000
//- ProcessGID=1000
//- KUS_REDIS_POOL_SIZE=100
//- KUS_TEMPDIR_SCAN_MIN=30
//- KUS_TEMPDIR_REMOVE_HOUR=12
//- ServiceDomain=upload.kr.kollus.com

func init() {
	println("configuration init")

	if os.Getenv("KUS_UPLOAD_HOST") == "" {
		os.Setenv("KUS_UPLOAD_HOST", "127.0.0.1")
	}
	if os.Getenv("KUS_UPLOAD_PORT") == "" {
		os.Setenv("KUS_UPLOAD_PORT", "3001")
	}

	if os.Getenv("KUS_LOG_PATH") == "" {
		os.Setenv("KUS_LOG_PATH", "/home/kollus/kollus-upload-v2/log")
	}

	if os.Getenv("KUS_PRODUCTION_MODE") == "" {
		os.Setenv("KUS_PRODUCTION_MODE", "debug")
	}

	if os.Getenv("KUS_TEMPDIR_SCAN_MIN") == "" {
		os.Setenv("KUS_TEMPDIR_SCAN_MIN", "30")
	}
	if os.Getenv("KUS_TEMPDIR_REMOVE_HOUR") == "" {
		os.Setenv("KUS_TEMPDIR_REMOVE_HOUR", "1")
	}

	if os.Getenv("KUS_PRODUCTION_MODE") == "debug" {
		println("KUS_PRODUCTION_MODE : DEBUG")
		os.Setenv("KUS_PROCESS_UID", strconv.Itoa(os.Getuid()))
		os.Setenv("KUS_PROCESS_GID", strconv.Itoa(os.Getgid()))
		//os.Setenv("KUS_PRE_H_ENABLE", "true")
		os.Setenv("KUS_LOG_PATH", "/home/kollus/kollus-upload-v2/log")
		os.Setenv("KUS_CONTENTS_PATH", "/home/kollus/kollus-upload-v2/http_upload")
		os.Setenv("KUS_CONTENTS_PASSTHROUGH_PATH", "/home/kollus/kollus-upload-v2/http_upload_passthrough")

		os.Setenv("KUS_REDIS_MASTERS", "mymaster")
		os.Setenv("KUS_REDIS_SENTINEL_PORT", "5000")
		os.Setenv("KUS_REDIS_SENTINEL_HOST", "127.0.0.1")

		os.Setenv("KUS_DUP_API", "http://api.kr.dev.kollus.com/0/media/content_provider?access_token=")
		os.Setenv("KUS_PROFILE_API", "http://api.kr.dev.kollus.com/0/media/media_profile?access_token=")
		os.Setenv("KUS_PRE_H_API", "http://api.kr.dev.kollus.com/0/media_auth/upload/begin_upload.json?access_token=dqgpfk4vioq7ztfm")
		os.Setenv("KUS_END_H_API", "http://api.kr.dev.kollus.com/0/media_auth/upload/complete_upload.json?access_token=dqgpfk4vioq7ztfm")
		os.Setenv("KUS_CREATE_H_API", "http://api.kr.dev.kollus.com/0/media_auth/upload/create_url_from_kus")
		os.Setenv("KUS_GET_ACCESS_TOKEN_API", "http://api.kr.dev.kollus.com/0/media/content_provider?access_token=")
		os.Setenv("KUS_SERVICE_DOMAIN", "api.kr.dev.kollus.com")
		os.Setenv("KUS_OAUTH_CATEGORY_API", "http://api.kr.dev.kollus.com/api/v1/vod/upload/categories")
		os.Setenv("KUS_OAUTH_TOKEN_API", "http://api.kr.dev.kollus.com/oauth/token")
	}

	//if os.Getenv("KUS_UPLOAD_HOST") == "" {
	//	os.Setenv("KUS_UPLOAD_HOST", "0.0.0.0")
	//}
	//if os.Getenv("KUS_UPLOAD_PORT") == "" {
	//	os.Setenv("KUS_UPLOAD_PORT", "4242")
	//}
	//
	//if os.Getenv("KUS_REDIS_SENTINEL_HOST") == "" {
	//	os.Setenv("KUS_REDIS_SENTINEL_HOST", "182.252.181.78")
	//	// Redis가 없는 상황에서도 서버를 구동시킵니다.
	//	//
	//	//os.Setenv("KUS_REDIS_SENTINEL_HOST", "0.0.0.0")
	//}
	//if os.Getenv("KUS_REDIS_SENTINEL_PORT") == "" {
	//	os.Setenv("KUS_REDIS_SENTINEL_PORT", "26379")
	//}
	//if os.Getenv("KUS_REDIS_MASTERS") == "" {
	//	os.Setenv("KUS_REDIS_MASTERS", "kollus")
	//}
	//if os.Getenv("KUS_PRODUCTION_MODE") == "" {
	//	os.Setenv("KUS_PRODUCTION_MODE", "debug")
	//}
	//if os.Getenv("KUS_PROCESS_UID") == "" {
	//	os.Setenv("KUS_PROCESS_UID", "2001")
	//}
	//if os.Getenv("KUS_PROCESS_GID") == "" {
	//	os.Setenv("KUS_PROCESS_GID", "2001")
	//}
	//if os.Getenv("KUS_REDIS_POOL_SIZE") == "" {
	//	os.Setenv("KUS_REDIS_POOL_SIZE", "100")
	//}
	//if os.Getenv("KUS_SERVICE_DOMAIN") == "" {
	//	//upload-stage-kr.kollus.com
	//	//127.0.0.1
	//	os.Setenv("KUS_SERVICE_DOMAIN", "upload-stage-kr.kollus.com")
	//}
	//if os.Getenv("KUS_UPLOAD_PUBLIC_IP") == "" {
	//	//upload-stage-kr.kollus.com
	//	//127.0.0.1
	//	os.Setenv("KUS_UPLOAD_PUBLIC_IP", "127.0.0.1")
	//}
	//
	// trusted_proxies
	if os.Getenv("KUS_TRUSTED_PROXIES") == "" {
		//upload-stage-kr.kollus.com
		//127.0.0.1
		os.Setenv("KUS_TRUSTED_PROXIES", "127.0.0.1-127.0.0.1")
		//os.Setenv("KUS_TRUSTED_PROXIES", "10.42.0.0-10.42.255.255")
	}
	//
	////if os.Getenv("KUS_CONTENTS_PATH") == "" {
	//os.Setenv("KUS_CONTENTS_PATH", "/tmp")
	////}
	//os.Setenv("KUS_CONTENTS_PASSTHROUGH_PATH", "/tmp_passthrough")
	//
	////create hook
	//if os.Getenv("KUS_CREATE_H_API") == "" {
	//	os.Setenv("KUS_CREATE_H_API", "http://api-stage-kr.kollus.com/0/media_auth/upload/create_url_from_kus")
	//}
	////get access token
	//if os.Getenv("KUS_GET_ACCESS_TOKEN_API") == "" {
	//	os.Setenv("KUS_GET_ACCESS_TOKEN_API", "https://api-stage-kr.kollus.com/0/media/content_provider?access_token=")
	//}
	//
	////oauth hook
	//if os.Getenv("KUS_OAUTH_TOKEN_API") == "" {
	//	os.Setenv("KUS_OAUTH_TOKEN_API", "https://vod-stage-kr.kollus.com/oauth/token")
	//}
	//if os.Getenv("KUS_OAUTH_CLIENT_ID") == "" {
	//	os.Setenv("KUS_OAUTH_CLIENT_ID", "69")
	//}
	//if os.Getenv("KUS_OAUTH_CLIENT_SECRET") == "" {
	//	os.Setenv("KUS_OAUTH_CLIENT_SECRET", "PSTul9hfIWnM77LYOjBxtV01B3xcGoVEqPSRJ0nh")
	//}
	//if os.Getenv("KUS_OAUTH_SCOPE") == "" {
	//	os.Setenv("KUS_OAUTH_SCOPE", "vod:uploader")
	//}
	//if os.Getenv("KUS_OAUTH_CATEGORY_API") == "" {
	//	os.Setenv("KUS_OAUTH_CATEGORY_API", "https://vod-stage-kr.kollus.com/api/v1/vod/upload/categories")
	//}
	//
	////pre hook
	//if os.Getenv("KUS_PRE_H_CONTENT_TYPE") == "" {
	//	os.Setenv("KUS_PRE_H_CONTENT_TYPE", "x-www-form-urlencode")
	//}
	//if os.Getenv("KUS_PRE_H_METHOD") == "" {
	//	os.Setenv("KUS_PRE_H_METHOD", "POST")
	//}
	//
	////end hook
	//if os.Getenv("KUS_END_H_CONTENT_TYPE") == "" {
	//	os.Setenv("KUS_END_H_CONTENT_TYPE", "x-www-form-urlencode")
	//}
	//if os.Getenv("KUS_END_H_METHOD") == "" {
	//	os.Setenv("KUS_END_H_METHOD", "POST")
	//}
	//
	//// dup key
	//if os.Getenv("KUS_DUP_API") == "" {
	//	os.Setenv("KUS_DUP_API", "http://api-stage-kr.kollus.com/0/media/content_provider?access_token=")
	//}
	//// get profile
	//if os.Getenv("KUS_PROFILE_API") == "" {
	//	os.Setenv("KUS_PROFILE_API", "http://api-stage-kr.kollus.com/0/media/media_profile?access_token=")
	//}
	////
	//// Local 환경 설정
	////
	//// Pre hook API
	//if os.Getenv("KUS_PRE_H_API") == "" {
	//	os.Setenv("KUS_PRE_H_API", "http://api-stage-kr.kollus.com/0/media_auth/upload/begin_upload.json?access_token=4sll7e0unr2n54yn")
	//}
	//// End hook API
	//if os.Getenv("KUS_END_H_API") == "" {
	//	os.Setenv("KUS_END_H_API", "http://api-stage-kr.kollus.com/0/media_auth/upload/complete_upload.json?access_token=4sll7e0unr2n54yn")
	//}
	//

	//
}

func LoadEnv() (*Configuration, error) {
	var config Configuration
	err := envconfig.Process("kus", &config)

	if err != nil {
		return nil, err
	}
	return &config, nil
}
