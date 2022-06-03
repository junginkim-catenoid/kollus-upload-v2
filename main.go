package main

import (
	"flag"
	"fmt"
	"gopkg.in/tylerb/graceful.v1"
	"io/ioutil"
	"kollus-upload-v2/cors"
	"kollus-upload-v2/handles"
	"kollus-upload-v2/pkg/config"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"gopkg.in/natefinch/lumberjack.v2"
)

const APP_VERSION = "1.3.6"
const KUS_STATICFILES_PATH = "/opt/go_work/bin/staticfiles"

var versionFlag *bool = flag.Bool("v", false, "Print the version number.")

func main() {
	fmt.Println("[INFO][STARTS] Started up KUS", time.Now().Format(" [2006/01/02-15:04:05]"))

	flag.Parse() // Scan the arguments list
	if *versionFlag {
		log.Println("[INFO][VERSION] Version:", APP_VERSION, time.Now().Format(" [2006/01/02-15:04:05]"))
		os.Exit(0)
	}

	var gConfiguraton *config.Configuration
	c, err := config.LoadEnv()
	if err != nil {
		log.Println("[ERROR] configuration loading error :"+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		os.Exit(1)
	}

	gConfiguraton = c

	log.Println("LogPath : ", gConfiguraton.LogPath)
	log.Println("ProcessUID : ", gConfiguraton.ProcessUID)

	if _, err := os.Stat(gConfiguraton.LogPath); os.IsNotExist(err) {
		_, err = os.Create(gConfiguraton.LogPath)
	}

	logwriterINFO := &lumberjack.Logger{Filename: gConfiguraton.LogPath, MaxSize: 100, MaxBackups: 3, MaxAge: 28}
	defer logwriterINFO.Close()

	log.SetOutput(logwriterINFO)
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	//[DEV mode]
	//debugging on the Docker
	if gConfiguraton.ProductionMode == "debug" {
		log.SetOutput(os.Stdout)
		log.Println("[INFO] Developer mode, check logging parameters before providing a production", time.Now().Format(" [2006/01/02-15:04:05]"))
	} else {
		//desable tracing on the consoble
		gin.DefaultWriter = logwriterINFO
	}

	// processUID != 0 , 즉 사용자가 선언되었을때,
	if gConfiguraton.ProcessUID != "0" {
		uid, _ := strconv.Atoi(gConfiguraton.ProcessUID)
		gid, _ := strconv.Atoi(gConfiguraton.ProcessGID)

		// logPath 에 대한 사용자권한, 그룹권한 부여
		if err := os.Chown(gConfiguraton.LogPath, uid, gid); err != nil {
			log.Println("[ERROR] Chown  : "+gConfiguraton.LogPath+" "+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
			os.Exit(1)
		}

		// logPath 에 대한 mode 부여 (읽기, 쓰기 권한 등)
		if err := os.Chmod(gConfiguraton.LogPath, 0766); err != nil {
			log.Println("[ERROR] Chmod for a logging file  : "+gConfiguraton.LogPath+" "+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
			os.Exit(1)
		}
	}

	// debug 모드일때는 logwriterInfo 를 사용하지 않아 실행되지 않음
	//log.SetOutput(logwriterINFO)

	// debug 모드시 실행하는 설정 로그들..
	log.Println("=======================================================")
	log.Println("[INFO] KUS-conf loaded , OK", time.Now().Format(" [2006/01/02-15:04:05]"))

	log.Println("[INFO] KUS-conf production mode         		 : "+gConfiguraton.ProductionMode, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-conf loaded UID              		 : "+string(gConfiguraton.ProcessUID), time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-conf loaded GID              		 : "+string(gConfiguraton.ProcessGID), time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-conf logging location at(docker)     	 : "+gConfiguraton.LogPath, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-conf contents path(docker)   		 : "+gConfiguraton.ContentsPath, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-conf host                    		 : "+gConfiguraton.UploadHost, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-conf port                    		 : "+gConfiguraton.UploadPort, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-conf RedisSentinal host      		 : "+gConfiguraton.RedisSentinelHost, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-conf RedisSentinal port      		 : "+gConfiguraton.RedisSentinelPort, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-conf temporary scaning       		 : ", gConfiguraton.KUS_TEMPDIR_SCAN_MIN, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-conf temporary removing      		 : ", gConfiguraton.KUS_TEMPDIR_REMOVE_HOUR, time.Now().Format(" [2006/01/02-15:04:05]"))

	log.Println("[INFO] KUS-web-prehook enable      		 	 : ", gConfiguraton.PreHookEnable, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-web-prehook contentType      	 	 : ", gConfiguraton.PreHookContentType, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-web-prehook method      		 	 : ", gConfiguraton.PreHookMethod, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-web-prehook API                           : ", gConfiguraton.PreHookAPI, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-web-endhook enable      		 	 : ", gConfiguraton.EndHookEnable, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-web-endhook contentType      	 	 : ", gConfiguraton.EndHookContentType, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-web-endhook method      		 	 : ", gConfiguraton.EndHookMethod, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-web-endhook API                           : ", gConfiguraton.EndHookAPI, time.Now().Format(" [2006/01/02-15:04:05]"))

	log.Println("[INFO] KUS-Service Domain                           : ", gConfiguraton.ServiceDomain, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] KUS-web-hook for creation					: ", gConfiguraton.CreateHookAPI, time.Now().Format(" [2006/01/02-15:04:05]"))

	if err := startServer(gConfiguraton); err != nil {
		log.Println("[ERROR] "+err.Error(), time.Now().Format(" [2006/01/02-15:04:05]"))
		os.Exit(1)
	}

	defer func() {
		log.Println("[INFO] BYE KOLLUS_UPLOAD", time.Now().Format(" [2006/01/02-15:04:05]"))
	}()
}

func startServer(conf *config.Configuration) error {
	log.Println("[INFO] Application version : "+APP_VERSION, time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] Starts upload service ", APP_VERSION, time.Now().Format(" [2006/01/02-15:04:05]"))

	if conf.ProductionMode == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	log.Println("[INFO] location of staticfiles : "+KUS_STATICFILES_PATH, time.Now().Format(" [2006/01/02-15:04:05]"))

	// redis setting
	client := redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:    conf.RedisMasters,
		SentinelAddrs: []string{conf.RedisSentinelHost + ":" + conf.RedisSentinelPort},
	})
	defer client.Close()

	handler := handles.NewSegmentUploadHandler(client, conf)
	log.Println(handler)

	runtime.GOMAXPROCS(runtime.NumCPU())

	gMux := gin.Default()

	v1 := gMux.Group("api/v1")
	{
		v1.Use(cors.CorsheaderMiddleware(cors.Options{}))
		v1.POST("/create_url", handler.CreateKollusOneTimeURL)
		v1.POST("/CreateUploadSession/:expTime/:uploadType", handler.CreateUploadSession)
		v1.POST("/UploadMultiParts/:upload_key/:user1", handler.UploadMultiParts)
	}

	gMux.StaticFile("/crossdomain.xml", KUS_STATICFILES_PATH+"/crossdomain.xml")
	gMux.StaticFS("/example", http.Dir(os.Getenv("GOPATH")+"/src/github.com/catenoid-company/kollus-upload/example"))
	log.Println("[INFO] GOPATH       : "+os.Getenv("GOPATH"), time.Now().Format(" [2006/01/02-15:04:05]"))
	log.Println("[INFO] Static page  : "+http.Dir(os.Getenv("GOPATH")+"/src/github.com/catenoid-company/kollus-upload/example"), time.Now().Format(" [2006/01/02-15:04:05]"))

	//clear session
	quit := make(chan bool)

	//go checkSession(quit,handler)
	go removeTemproraryDirectories(quit, conf.KUS_TEMPDIR_SCAN_MIN, conf.KUS_TEMPDIR_REMOVE_HOUR)

	log.Println("[INFO] KUS-conf Upload port  : "+conf.UploadPort, time.Now().Format(" [2006/01/02-15:04:05]"))

	defer func() {
		quit <- true
		close(quit)
		log.Println("[INFO] BYE KOLLUS_UPLOAD", time.Now().Format(" [2006/01/02-15:04:05]"))
	}()

	srv := &graceful.Server{
		Timeout: 0,
		ConnState: func(conn net.Conn, state http.ConnState) {
		},
		Server: &http.Server{
			Addr:    conf.UploadHost + ":" + conf.UploadPort,
			Handler: gMux,
		},
	}

	srv.ListenAndServe()

	return nil
}

/// linux '/tmp' 에 있는 경과된 파일을 삭제 합니다.
/// 삭제할 directory는 /tmp/KUS*  로 시작 됩니다.
/// 30분에 한번씩 24시간 이상 경과한 파일에 대하여 삭제
func removeTemproraryDirectories(quit chan bool, KUS_TEMPDIR_SCAN_MIN int, KUS_TEMPDIR_REMOVE_HOUR int) {

	scantime := KUS_TEMPDIR_SCAN_MIN
	if scantime < 1 {
		scantime = 1
		log.Println("[INFO] scantime ", scantime, time.Now().Format(" [2006/01/02-15:04:05]"))
	}

	tick := time.Tick(time.Duration(scantime) * time.Minute)
	log.Println("[INFO] Begins checking the temprorary directory", time.Now().Format(" [2006/01/02-15:04:05]"))
	for {
		select {
		case <-tick:
			dirname := "/tmp/.working"
			log.Println("[INFO] Check", dirname, time.Now().Format(" [2006/01/02-15:04:05]"))
			list, err := ioutil.ReadDir(dirname)
			if err != nil {
				log.Println("[ERROR] ReadDir %s: error expected, none found", dirname, time.Now().Format(" [2006/01/02-15:04:05]"))
			}
			for _, dir := range list {

				log.Println("IsDir : ", dir.IsDir())
				log.Println("dir.Name : ", len(dir.Name()))
				log.Println("dir.Name : ", dir.Name())
				log.Println("dir.Name[0:3] : ", string([]rune(dir.Name())[0:3]))

				switch {
				case dir.IsDir() && len(dir.Name()) > 3 && string([]rune(dir.Name())[0:3]) == "KUS":
					//log.Println(string([]rune(dir.Name())[0:3]));
					end := time.Now()
					latency := end.Sub(dir.ModTime())

					//env := os.Getenv("KUS_TEMPDIR_REMOVE_HOUR");
					frequencyHour := KUS_TEMPDIR_REMOVE_HOUR
					if KUS_TEMPDIR_REMOVE_HOUR < 1 {
						frequencyHour = 24
						log.Println("[INFO] frequencyHour ", frequencyHour, time.Now().Format(" [2006/01/02-15:04:05]"))
					}
					//log.Println(frequencyHour)
					if int(latency.Hours()) > frequencyHour {
						//if int(latency.Minutes()) > frequencyHour {
						if err = os.RemoveAll(path.Join(dirname, dir.Name())); err != nil {
							log.Println("[ERROR] file removing error", err.Error(), dir.Name(), time.Now().Format(" [2006/01/02-15:04:05]"))
						}
						log.Println("[INFO] Removed temprorary directory in ", path.Join(dirname, dir.Name()), frequencyHour, time.Now().Format(" [2006/01/02-15:04:05]"))
					}
					break
				}
			}
		case <-quit:
			log.Println("[INFO] Quit removeTemproraryFiles", time.Now().Format(" [2006/01/02-15:04:05]"))
			return
		}
	}
}
