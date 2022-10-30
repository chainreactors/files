package files

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"sync"
)

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
			case "!!sync":
				file.Sync()
				file.wg.Done()
			case "!!close":
				if file.ClosedAppend != "" {
					file.Write(file.ClosedAppend)
				}
				file.Sync()
				file.wg.Done()
			default:
				if !file.Initialized {
					err := file.Init()
					if err != nil {
						fmt.Println(file.Filename + err.Error())
						return
					}
				}
				file.Write(file.Handler(s))
				file.wg.Done()
			}
		}

		if file.fileHandler != nil {
			file.fileHandler.Close()
		}
		file.wg.Done()
	}()

	return file, nil
}

type File struct {
	Filename     string
	Initialized  bool
	InitSuccess  bool
	DataCh       chan string
	Handler      func(string) string
	Encoder      func([]byte) []byte
	ClosedAppend string
	Closed       bool

	fileHandler *os.File
	wg          sync.WaitGroup
	fileWriter  *bufio.Writer
	buf         *bytes.Buffer
	encode      bool
	append      bool
}

func (f *File) Init() error {
	if f.fileHandler == nil {
		var err error
		// 防止初始化失败之后重复初始化, flag提前设置为true
		f.Initialized = true

		if f.append {
			f.fileHandler, err = AppendFile(f.Filename)
		} else {
			f.fileHandler, err = CreateFile(f.Filename)
		}
		if err != nil {
			return err
		}
		f.InitSuccess = true
		f.fileWriter = bufio.NewWriter(f.fileHandler)
	}
	return nil
}

func (f *File) SafeWrite(s string) {
	if !f.Closed {
		f.wg.Add(1)
		f.DataCh <- s
	}
}

func (f *File) SafeSync() {
	if !f.Closed {
		f.wg.Add(1)
		f.DataCh <- "!!sync"
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
	if f.fileHandler == nil || f.buf.Len() == 0 {
		return
	}

	if f.encode {
		_, _ = f.fileWriter.Write(f.Encoder(f.buf.Bytes()))
	} else {
		_, _ = f.fileWriter.Write(f.buf.Bytes())
	}

	f.buf.Reset()
	_ = f.fileWriter.Flush()
	return
}

func (f *File) Close() {
	f.wg.Add(1)
	f.DataCh <- "!!close"
	f.wg.Wait()

	f.wg.Add(1)
	close(f.DataCh)
	f.wg.Wait()

	f.Closed = true
}
