package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var version string

type PreData struct {
	Cgi              []string   `json:"cgi"`
	Info             []KeyValue `json:"info"`
	VisitedPageCount int        `json:"visitedPageCount"`
	Script           string     `json:"script"`
}

func NewPreData() PreData {
	infoList := make([]KeyValue, 0)
	appid := KeyValue{
		Key:   "appId",
		Value: "wx671bc478c73587ff",
	}
	infoList = append(infoList, appid)
	icon := KeyValue{
		Key:   "icon",
		Value: "http://wx.qlogo.cn/mmhead/Q3auHgzwzM5l22hvUV0N49nnc8HlgXQ7DR5aQF58ujWlLprZmqxXkA/96",
	}
	infoList = append(infoList, icon)
	nickname := KeyValue{
		Key:   "nickname",
		Value: "LELABO",
	}
	infoList = append(infoList, nickname)
	envVersion := KeyValue{
		Key:   "envVersion",
		Value: "trial",
	}
	infoList = append(infoList, envVersion)
	version := KeyValue{
		Key:   "version",
		Value: "",
	}
	infoList = append(infoList, version)
	return PreData{
		Info:             infoList,
		VisitedPageCount: 25,
		Script:           "",
	}
}

type Har struct {
	Log HLog
}

type HLog struct {
	Version string
	Entries []HEntry
}

type HEntry struct {
	//StartedDateTime string
	//Time int
	Request  HRequest
	Response HResponse
}

type HRequest struct {
	Url         string
	Method      string
	HttpVersion string
	Headers     []NameValue
	QueryString []NameValue
	PostData    PostData
}

type HResponse struct {
	Content HContent
}

type HContent struct {
	Size     int
	MimeType string
	Text     string
	Encoding string
}

type NameValue struct {
	Name  string
	Value string
}
type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
type PostData struct {
	Text string
}

// dump files to dir, keep hostname and path info as subfolders.
func (e *HEntry) dump(dir string) {
	u, err := url.Parse(e.Request.Url)
	if err != nil {
		log.Fatal(err)
	}
	scheme, host, path := u.Scheme, u.Host, u.Path // host,path = www.google.com , /search.do
	if scheme == "chrome-extension" {
		return // ignore all chrome-extension requests.
	}
	if i := strings.LastIndex(host, ":"); i != -1 {
		host = host[0:i] // remove port
	}
	path = dir + "/" + host + path
	if j := strings.LastIndex(path, "/"); j != -1 {
		os.MkdirAll(path[0:j], os.ModePerm)
		e.Response.Content.writeTo(path)
	}
}

// dump all files to the very dir, no subfolder created.
func (e *HEntry) dumpDirectly(dir string) {
	u, err := url.Parse(e.Request.Url)
	if err != nil {
		log.Fatal(err)
	}
	path := u.Path
	if j := strings.LastIndex(path, "/"); j != -1 {
		e.Response.Content.writeTo(dir + path[j:])
	}
}

func decode(str string, fileName string) {
	data, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		log.Fatal(err, " fileName:", fileName)
	} else {
		ioutil.WriteFile(fileName, data, os.ModePerm)
	}
}

var textContentPattern = regexp.MustCompile("text|json|javascript|ecmascript|xml")

func (c *HContent) writeTo(desiredFileName string) {
	f := getNoDuplicatePath(desiredFileName)

	switch {
	case strings.EqualFold(c.Encoding, "base64"):
		decode(c.Text, f)
	case textContentPattern.MatchString(c.MimeType):
		ioutil.WriteFile(f, []byte(c.Text), os.ModePerm)
	default:
		decode(c.Text, f)
	}
}

func fileExists(path string) bool {
	abs_path, _ := filepath.Abs(path)
	_, err := os.Stat(abs_path)
	return err == nil
}

func getNoDuplicatePath(desiredFileName string) (path string) {
	var i int
	for i, path = 2, desiredFileName; fileExists(path); i++ {

		ext := filepath.Ext(desiredFileName)
		pathBeforeExtension := strings.TrimSuffix(desiredFileName, ext)

		path = fmt.Sprintf("%s_(%d)%s", pathBeforeExtension, i, ext)
	}
	return
}

func (c *HContent) writeToFile(f *os.File) {
	f.WriteString(c.Text)
	f.Close()
}

func handle(r *bufio.Reader) {
	dec := json.NewDecoder(r)
	var har Har
	err := dec.Decode(&har)
	if err != nil {
		log.Fatal(err)
		os.Exit(-2)
	} else {
		if har.Log.Version == "1.2" {
			version12 = true
		}
		cgiList := make([]string, 0)
		for index, entry := range har.Log.Entries {
			b := listEntries(index, entry)
			cgiList = append(cgiList, b)
		}
		pd := NewPreData()
		pd.Cgi = cgiList
		xx, _ := json.Marshal(&pd)
		fmt.Println(string(xx))
	}
}

func output(index int, entry HEntry) {
	if list {
		if urlPattern != nil {
			if len(urlPattern.FindString(entry.Request.Url)) > 0 {
				listEntries(index, entry)
			}
		} else if mimetypePattern != nil {
			if len(mimetypePattern.FindString(entry.Response.Content.MimeType)) > 0 {
				listEntries(index, entry)
			}
		} else {
			listEntries(index, entry)
		}
	} else if extract {
		if index == extractIndex {
			extractOne(entry)
		}
	} else if extractPattern {
		if urlPattern != nil && len(urlPattern.FindString(entry.Request.Url)) > 0 {
			entry.dump(dir)
		} else if mimetypePattern != nil && len(mimetypePattern.FindString(entry.Response.Content.MimeType)) > 0 {
			if dumpDirectly {
				entry.dumpDirectly(dir)
			} else {
				entry.dump(dir)
			}
		}
	} else if extractAll {
		entry.dump(dir)
	}
}

func listEntries(index int, entry HEntry) string {
	var b bytes.Buffer
	b.Write([]byte(entry.Request.Method))
	b.Write([]byte(" "))
	b.Write([]byte(entry.Request.Url))
	b.Write([]byte(" "))
	b.Write([]byte(entry.Request.HttpVersion))
	b.Write([]byte("\r\n"))
	for _, v := range entry.Request.Headers {
		//if i == 0 && len(entry.Request.Headers) == 1 {
		//	b.Write([]byte(fmt.Sprintf("%s: %s", v.Name, v.Value)))
		//} else if i == len(entry.Request.Headers)-1 {
		//	b.Write([]byte(fmt.Sprintf("%s: %s", v.Name, v.Value)))
		//} else {
		b.Write([]byte(fmt.Sprintf("%s: %s\r\n", v.Name, v.Value)))
		//}
	}
	//b.Write([]byte("\r\n"))
	b.Write([]byte("\r\n"))
	if len(entry.Request.PostData.Text) != 0 {
		b.Write([]byte(entry.Request.PostData.Text))
	}
	//fmt.Println(b.String())
	encodeString := base64.StdEncoding.EncodeToString(b.Bytes())
	//fmt.Println(encodeString)
	return encodeString
	//fmt.Printf("[%3d][%6s][%25s][Size:%8d][URL:%s]\n", index, entry.Request.Method, entry.Response.Content.MimeType, entry.Response.Content.Size, entry.Request.Url)
}

func extractOne(entry HEntry) {
	fmt.Print(entry.Response.Content.Text)
}

var list bool = false

var extract bool = false
var extractIndex int = -1
var dumpDirectly bool = false

var extractPattern bool = false
var urlPattern *regexp.Regexp = nil
var mimetypePattern *regexp.Regexp = nil

var extractAll bool = false
var dir string = ""

var version12 bool = false // is of version 1.2 format ?

func help() {
	fmt.Println(`
usage: harx [options] har-file
    -l                         List files , lead by [index]
    -lu  urlPattern            List files filtered by UrlPattern
    -lm  mimetypePattern       List files filtered by response Mimetype
    -x   dir                   eXtract all content to [dir]
    -xi  index                 eXtract the [index] content to stdout , need run with -l first to get [index]
    -xu  urlPattern      dir   eXtract to [dir] with UrlPattern filter
    -xm  mimetypePattern dir   eXtract to [dir] with MimetypePattern filter
    -xmd mimetypePattern dir   eXtract to [dir] Directly without hostname and path info, with MimetypePattern filter
    -v                         Print version information
	
    `)
}
func main() {
	if len(os.Args) == 1 {
		help()
		return
	}

	var fileName string
	switch os.Args[1] {
	case "-l":
		list = true
		fileName = os.Args[2]
	case "-lu":
		list = true
		urlPattern = regexp.MustCompile(os.Args[2])
		fileName = os.Args[3]
	case "-lm":
		list = true
		mimetypePattern = regexp.MustCompile(os.Args[2])
		fileName = os.Args[3]
	case "-xi":
		extract = true
		extractIndex, _ = strconv.Atoi(os.Args[2])
		fileName = os.Args[3]
	case "-xu":
		extractPattern = true
		urlPattern = regexp.MustCompile(os.Args[2])
		dir = os.Args[3]
		fileName = os.Args[4]
	case "-xm":
		extractPattern = true
		mimetypePattern = regexp.MustCompile(os.Args[2])
		dir = os.Args[3]
		fileName = os.Args[4]
	case "-xmd":
		extractPattern = true
		dumpDirectly = true
		mimetypePattern = regexp.MustCompile(os.Args[2])
		dir = os.Args[3]
		fileName = os.Args[4]
	case "-x":
		extractAll = true
		dir = os.Args[2]
		fileName = os.Args[3]
	case "-v":
		fmt.Printf("harx version %s\n", version)
		return
	default:
		help()
		return
	}

	file, err := os.Open(fileName)

	if err == nil {
		if dir != "" {
			os.MkdirAll(dir, os.ModePerm)
		}
		handle(bufio.NewReader(file))
	} else {
		fmt.Printf("Cannot open file : %s\n", fileName)
		log.Fatal(err)
		os.Exit(-1)
	}

}
