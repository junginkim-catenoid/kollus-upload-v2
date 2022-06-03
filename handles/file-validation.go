package handles

import (
	"errors"
	"fmt"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

type UploadInterface interface {
	FindUploadFile(*CtxResponseWebHook) error
	AddFile(string, string) error
}

type Inspects struct {
	ui  UploadInterface
	ufi *FileUtils
}

type UploadInspect struct {
}

type FileUtils struct {
}

//type result struct {
//	res interface{}
//}

// 업로드 및 파일 이동시 확인할 인터페이스 추
var i = &Inspects{&UploadInspect{}, &FileUtils{}}

func ConfirmedToFiles(chanCtx chan *CtxResponseWebHook, isComplete chan error) {
	ctx := <-chanCtx
	err := i.ui.FindUploadFile(ctx)

	if err != nil {
		isComplete <- err
		close(chanCtx)
	} else {
		isComplete <- nil
		close(chanCtx)
	}
}

func CreateCompleteFile(chanTempPath chan string, isCreated chan error, uploadFileKey string) {
	tempPath := <-chanTempPath
	err := i.ui.AddFile(tempPath, uploadFileKey)

	if err != nil {
		isCreated <- err
		close(chanTempPath)
	} else {
		isCreated <- nil
		close(chanTempPath)
	}
}

func (utils *FileUtils) ReadDirInFile(path string) (os.FileInfo, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Println("Empty Directory")
		return nil, err
	}
	var fileName string
	for _, f := range files {
		fileName = f.Name()
	}
	info, err := os.Stat(path + "/" + fileName)
	if err != nil {
		log.Println("Empty FileNames", fileName)
		return nil, err
	}

	return info, nil
}

func (utils *FileUtils) ReadFileStatus(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		log.Println("Empty FileNames", path)
		return nil, err
	}

	return info, nil
}

func (ui *UploadInspect) FindUploadFile(ctx *CtxResponseWebHook) error {
	info, err := i.ufi.ReadDirInFile(ctx.temprorary_directory)
	if err != nil {
		return err
	}
	if (ctx.file_size) != info.Size() {
		return errors.New("Inspect to File defferced")
	}
	log.Println("UploadFileName : ", info.Name(), info.Size(), " : OriginalFile")

	return nil
}

func (ui *UploadInspect) AddFile(tempPath string, uploadFileKey string) error {
	info, err := i.ufi.ReadFileStatus(tempPath)
	if err != nil {
		return err
	}

	size := strconv.Itoa(int(info.Size()))
	//sTime := time.Now().Format("2006-01-02 15:04:05")
	//cTime, _ := time.Parse("2006-01-02_15:04:05",sTime)
	//nTime := strconv.Itoa(int(cTime.UnixNano()/1000000))

	cTime := info.ModTime()
	nTime := strconv.Itoa(int(cTime.UnixNano() / 1000000))

	paths := strings.Split(tempPath, "/")
	pathSize := len(paths)
	fileName := paths[pathSize-1]
	fileName = strings.TrimSpace(fileName)
	dirPath := strings.Replace(tempPath, fileName, fileName+"_complete", -1)

	var fByte = []byte("k:" + uploadFileKey + "\n" + "f:" + tempPath + "\n" + "t:" + nTime + "\n" + "l:" + size)
	//var fByte = []byte("{k:"+uploadFileKey+"}\n"+"{f:"+tempPath+"}\n"+"{t:"+nTime+"}\n"+"{l:"+size+"}")

	fw, err := os.Create(dirPath)

	if err != nil {
		return err
	}

	_, err = fw.Write(fByte)

	if err != nil {
		return err
	}

	log.Println(fileName, dirPath)
	return nil
}

func (utils *FileUtils) ConvertNFCToNFD(name string) string {
	if norm.NFD.IsNormalString(name) {
		fmt.Print("Before to converted FileName : ", name, "\n")
		t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
		s, _, _ := transform.String(t, name)
		name = s
		fmt.Print("Complete to converted FileName :", name, "\n")
	}
	return name
}
