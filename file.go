package files

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func CreateFile(filename string) (*os.File, error) {
	var err error
	var filehandle *os.File
	if _, err := os.Stat(filename); err == nil { //如果文件存在
		return nil, errors.New("File already exists")
	} else {
		filehandle, err = os.Create(filename) //创建文件
		if err != nil {
			return nil, err
		}
	}
	return filehandle, err
}

func AppendFile(filename string) (*os.File, error) {
	var err error
	var filehandle *os.File
	if _, err := os.Stat(filename); err == nil { //如果文件存在
		filehandle, err = os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			return nil, err
		}
	} else {
		filehandle, err = os.Create(filename) //创建文件
		if err != nil {
			return nil, err
		}
	}
	return filehandle, err
}

func NewFile(filename string, encode, lazy, append bool) (*File, error) {
	var file = &File{
		Filename: filename,
		encode:   encode,
		append:   append,
		buf:      bytes.NewBuffer([]byte{}),
		DataCh:   make(chan string, 100),
		Handler: func(s string) string {
			return s
		},
		Encoder: Flate,
	}

	if !lazy {
		err := file.Init()
		if err != nil {
			return nil, err
		}
	}

	go func() {
		for s := range file.DataCh {
			switch s {
			case "sync":
				file.Sync()
			default:
				if !file.Initialized {
					err := file.Init()
					if err != nil {
						fmt.Println(file.Filename + err.Error())
						return
					}
				}
				file.Write(file.Handler(s))
			}
		}

		if file.FileHandler != nil {
			if file.ClosedAppend != "" {
				file.Write(file.ClosedAppend)
			}
			file.Sync()
			file.FileHandler.Close()
		}
	}()

	return file, nil
}

type File struct {
	Filename     string
	Initialized  bool
	FileHandler  *os.File
	DataCh       chan string
	Handler      func(string) string
	Encoder      func([]byte) []byte
	ClosedAppend string
	Closed       bool
	fileWriter   *bufio.Writer
	buf          *bytes.Buffer
	encode       bool
	append       bool
}

func (f *File) Init() error {
	if f.FileHandler == nil {
		var err error
		// 防止初始化失败之后重复初始化, flag提前设置为true
		f.Initialized = true

		if f.append {
			f.FileHandler, err = AppendFile(f.Filename)
		} else {
			f.FileHandler, err = CreateFile(f.Filename)
		}
		if err != nil {
			return err
		}
		f.fileWriter = bufio.NewWriter(f.FileHandler)
	}
	return nil
}

func (f *File) SafeWrite(s string) {
	if !f.Closed {
		f.DataCh <- s
	}
}

func (f *File) SafeSync() {
	if !f.Closed {
		f.DataCh <- "sync"
	}
}

func (f *File) Write(s string) {
	_, _ = f.buf.WriteString(s)
	if f.buf.Len() > 4096 {
		f.Sync()
	}
	return
}

func (f *File) SyncWrite(s string) {
	f.Write(s)
	f.Sync()
}

func (f *File) WriteBytes(bs []byte) {
	_, _ = f.buf.Write(bs)
	if f.buf.Len() > 4096 {
		f.Sync()
	}
}

func (f *File) Sync() {
	if f.FileHandler == nil || f.buf.Len() == 0 {
		return
	}

	if f.encode {
		_, _ = f.fileWriter.Write(f.Encoder(f.buf.Bytes()))
	} else {
		_, _ = f.fileWriter.Write(f.buf.Bytes())
	}
	//Log.Debugf("sync %d bytes to %s", f.buf.Len(), f.Filename)
	f.buf.Reset()
	_ = f.fileWriter.Flush()
	_ = f.FileHandler.Sync()
	return
}

func (f *File) Close() {
	close(f.DataCh)
	time.Sleep(200)
	_ = f.FileHandler.Close()
	f.Closed = true
}

func GetExcPath() string {
	file, _ := exec.LookPath(os.Args[0])
	// 获取包含可执行文件名称的路径
	path, _ := filepath.Abs(file)
	// 获取可执行文件所在目录
	index := strings.LastIndex(path, string(os.PathSeparator))
	ret := path[:index]
	return strings.Replace(ret, "\\", "/", -1) + "/"
}
