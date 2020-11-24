package download

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const defaultBufferSize = 2 << 20

type downloadClient struct {
	httpClient             *http.Client
	overwrite              bool
	fileUrl, fileLocalPath string
	header                 map[string]string
	progressCallBackHandle func(finish, total int64) bool //想要停止，在进度回调中返回false
	completeCallBackHandle func(err error)                //完成后回调
	completeChan           chan bool
	chunkSize              int64
}

//被暂停不会回调
func (client *downloadClient) completeCallBack(err error) {
	if client.completeCallBackHandle != nil {
		client.completeCallBackHandle(err)
	}
}

func (client *downloadClient) progressCallBack(finish, total int64) bool {
	if client.progressCallBackHandle != nil {
		return client.progressCallBackHandle(finish, total)
	} else {
		return true
	}
}
func (client *downloadClient) SetProgressCallBackHandle(callBackHandle func(finish, total int64) bool) *downloadClient {
	client.progressCallBackHandle = callBackHandle
	return client
}
func (client *downloadClient) SetCompleteCallBackHandle(callBackHandle func(err error)) *downloadClient {
	client.completeCallBackHandle = callBackHandle
	return client
}

func (client *downloadClient) Overwrite(filePath string) *downloadClient {
	client.fileLocalPath = filePath
	client.overwrite = true
	return client
}
func (client *downloadClient) Append(filePath string) *downloadClient {
	client.fileLocalPath = filePath
	client.overwrite = false
	return client
}
func (client *downloadClient) SetHeader(key, value string) *downloadClient {
	client.header[key] = value
	return client
}

func (client *downloadClient) Get(url string) *downloadClient {
	client.fileUrl = url
	return client
}

func getFileSize(filePath string) (int64, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		} else {
			return 0, err
		}
	} else {
		if stat.IsDir() {
			return 0, errors.New(fmt.Sprintf("%s is dir", filePath))
		} else {
			return stat.Size(), nil
		}
	}

}

func (client *downloadClient) Start() {
	defer close(client.completeChan)
	var fileSize int64 = 0
	if !client.overwrite {
		size, err := getFileSize(client.fileLocalPath)
		if err != nil {
			client.completeCallBack(err)
			client.completeChan <- false
			return
		}
		fileSize = size
	}

	resp, err := http.NewRequest("GET", client.fileUrl, nil)
	if err != nil {
		client.completeCallBack(err)
		client.completeChan <- false
		return
	}
	for key, value := range client.header {
		resp.Header.Add(key, value)
	}
	resp.Header.Add("Range", fmt.Sprintf("bytes=%d-", fileSize))

	response, err := client.httpClient.Do(resp)
	if err != nil {
		client.completeCallBack(err)
		client.completeChan <- false
		return
	}
	defer response.Body.Close()
	var fileSizeFromServer int64 = 0
	if fileSize > 0 {
		if response.StatusCode == 416 {
			client.completeCallBack(nil)
			client.completeChan <- true
			return
		}
		fileSizeFromServer = getContentLengthFromContentRange(response.Header.Get("Content-Range"))
		if fileSizeFromServer == 0 {
			client.completeCallBack(errors.New("this request not supported for partial content"))
			client.completeChan <- true
			return
		}
	} else {
		if response.StatusCode < 300 {
			fileSizeFromServer, _ = strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
		} else {
			client.completeCallBack(errors.New(response.Status))
			client.completeChan <- true
			return
		}
	}
	flag := os.O_RDWR | os.O_CREATE
	if client.overwrite {
		flag = os.O_RDWR | os.O_CREATE | os.O_TRUNC
	}
	fileHandle, err := os.OpenFile(client.fileLocalPath, flag, 0664)
	if err != nil {
		client.completeCallBack(err)
		client.completeChan <- false
		return
	}
	defer fileHandle.Close()
	_, err = fileHandle.Seek(0, os.SEEK_END)
	if err != nil {
		client.completeCallBack(err)
		client.completeChan <- false
		return
	}

	totalSize := fileSizeFromServer + fileSize
	finishSize := fileSize
	buffer := make([]byte, client.chunkSize)
	for {
		n, err := response.Body.Read(buffer)
		if n > 0 {
			fileHandle.Write(buffer[0:n])
			finishSize = finishSize + int64(n)
			if client.progressCallBack(finishSize, totalSize) == false {
				fmt.Println("downLoad stop")
				break
			}
		}
		if err == io.EOF { //结束
			if finishSize >= totalSize {
				client.completeCallBack(nil)
			} else {
				client.completeCallBack(errors.New("remote end hung up unexpectedly"))
			}
			break
		} else {
			if err != nil {
				client.completeCallBack(err)
				client.completeChan <- false
				return
			}
		}
	}

	client.completeChan <- true

}
func (client *downloadClient) Wait() bool {
	success := <-client.completeChan
	return success
}
func Default() *downloadClient {
	return New(defaultBufferSize)
}

func New(chunkSize int64) *downloadClient {
	client := &downloadClient{}
	client.overwrite = false
	client.chunkSize = chunkSize
	client.completeChan = make(chan bool, 1)
	client.httpClient = &http.Client{}
	return client
}

//正常ContentRange: bytes 277768-277787/277788
//不正常: bytes */277788
//不正常:  空
//请求的range是合法时返回正常的ContentRange。
func getContentLengthFromContentRange(contentRange string) int64 {
	if contentRange == "" {
		return 0
	}
	splits := strings.Split(contentRange, " ")
	if len(splits) != 2 {
		return 0
	}
	splits = strings.Split(splits[1], "/")
	if len(splits) != 2 {
		return 0
	}
	splits = strings.Split(splits[0], "-")
	if len(splits) != 2 {
		return 0
	} else {
		start, err := strconv.ParseInt(splits[0], 10, 64)
		if err != nil {
			return 0
		}
		end, err := strconv.ParseInt(splits[1], 10, 64)
		if err != nil {
			return 0
		}
		return end - start + 1
	}
}
