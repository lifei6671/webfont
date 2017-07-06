package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var CachePath = "./"
var Expire = 3600 * 24 * 30
var Domain = ""

func main() {
	addr := *flag.String("addr", "", "IP Address")
	port := *flag.Int("port", 8090, "Server Port")
	tls := *flag.Bool("ssl", false, "Is enable https.")
	cert := *flag.String("cert", "", "HTTPS certificate file")
	key := *flag.String("key", "", "HTTPS key file.")
	flag.StringVar(&CachePath, "cache", "./", "Font and css cache path.")
	flag.IntVar(&Expire, "expire", 3600*24*30, "Cache expire.")
	flag.StringVar(&Domain, "domain", "", "Font domain name.")

	flag.Parse()

	if len(os.Args) >= 2 && os.Args[1] == "help" {
		flag.Usage();
		os.Exit(0)
	}


	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()

	if Domain == "" {
		if tls == true {
			Domain = fmt.Sprintf("https://localhost:%d", port)
		} else {
			Domain = fmt.Sprintf("http://localhost:%d", port)
		}
	}

	addr = fmt.Sprintf("%s:%d", addr, port)

	http.HandleFunc("/css", CssHandler)
	http.HandleFunc("/s/", FontHandler)

	if tls == true {
		if !FileExits(cert) {
			log.Fatalln("CertFile not found.")
		}
		if !FileExits(key) {
			log.Fatalln("KeyFile not found.")
		}
		log.Printf("http server Running on https://%s\n", addr)
		err := http.ListenAndServeTLS(addr, cert, key, nil)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		log.Printf("http server Running on https://%s\n", addr)

		err := http.ListenAndServe(addr, nil)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func FileExits(name string) bool {
	if _, err := os.Stat(name); os.IsNotExist(err) {
		return false
	}
	return true
}

//处理CSS请求.
func CssHandler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		log.Printf("%s %s\r", r.Method, r.RequestURI)
	}()

	err := r.ParseForm()

	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Invalid parameter:%s", err)
		return
	}
	family := r.FormValue("family")
	if family == "" {
		w.WriteHeader(500)
		fmt.Fprintf(w, ErrNotFamily)
		return
	}
	v := Md5(family)

	cssPath := filepath.Join(CachePath, "css", v+".css")
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Server", "Google Font Proxy")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache")
	f, err := os.Stat(cssPath)
	if err == nil {
		if f.ModTime().Add(time.Second * time.Duration(Expire)).After(time.Now()){
			b, err := ioutil.ReadFile(cssPath)
			if err != nil {
				w.WriteHeader(500)
				fmt.Fprintf(w, "Read file error:%s", err)
				return
			}

			fmt.Fprint(w, string(b))
			return
		} else {
			os.Remove(cssPath)
		}
	}

	urlStr := "https://fonts.googleapis.com/css?family=" + family

	body, status, err := Request(urlStr)

	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Request source error: %s", err)
		return
	}
	if status != 200 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		fmt.Fprint(w, string(body))
		return
	}

	body = bytes.Replace(body, []byte("https://fonts.gstatic.com"), []byte(Domain), -1)

	os.MkdirAll(filepath.Dir(cssPath), 0766)
	ioutil.WriteFile(cssPath, body, 0766)

	fmt.Fprint(w, string(body))
}

func FontHandler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		log.Printf("%s %s\r", r.Method, r.RequestURI)
	}()

	err := r.ParseForm()

	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Invalid parameter:%s", err)
		return
	}

	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "font/"+string(filepath.Ext(r.RequestURI)[1:]))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Server", "Google Font Proxy")

	fontPath := filepath.Join(CachePath, r.RequestURI)

	f, err := os.Stat(fontPath)

	if err == nil {
		if f.ModTime().Add(time.Second * time.Duration(Expire)).After(time.Now()) {
			b, err := ioutil.ReadFile(fontPath)
			if err != nil {
				w.WriteHeader(500)
				fmt.Fprintf(w, "Read file error:%s", err)
				return
			}

			fmt.Fprint(w, string(b))
			return
		}
	} else {
		os.Remove(fontPath)
	}

	urlStr := "https://fonts.gstatic.com" + r.RequestURI

	body, status, err := Request(urlStr)

	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Request source error: %s", err)
		return
	}
	if status != 200 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		fmt.Fprint(w, string(body))
		return
	}
	os.MkdirAll(filepath.Dir(fontPath), 0766)

	ioutil.WriteFile(fontPath, body, 0766)

	fmt.Fprint(w, string(body))
}

//计算md5值.
func Md5(v string) string {
	md5Ctx := md5.New()
	md5Ctx.Write([]byte(v))
	cipherStr := md5Ctx.Sum(nil)

	return hex.EncodeToString(cipherStr)
}

//请求.
func Request(urlStr string) ([]byte, int, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", urlStr, strings.NewReader(""))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/55.0.2883.87 Safari/537.36")
	resp, err := client.Do(req)

	if err != nil {
		return nil, 0, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

const ErrNotFamily = `
<!DOCTYPE html><html lang=en><head><meta charset=utf-8><title>Error 400 (Font family not found)!!1</title><link href="//fonts.googleapis.com/css?family=Open+Sans:300,400" rel="stylesheet" type="text/css" /><style>
      * {
        margin: 0;
        padding: 0;
      }
      html,code {
        font: 15px/22px arial,sans-serif;
      }
      html {
        background: #fff;
        color: #222;
        padding: 15px;
      }
      body {
        background: 100% 5px no-repeat;
        margin-top: 0;
        max-width: none:
        min-height: 180px;
        padding: 30px 0 15px;
      }
      * > body {
        padding-right: 205px;
      }
      p {
        margin: 22px 0 0;
        overflow: hidden;
      }
      ins {
        text-decoration: none;
      }
      ins {
        color: #777;
      }
      /* Google Fonts logo styling*/
      .projectLogo a {
        font-family: "Open Sans", arial, sans-serif;
        font-size: 32px;
        font-weight: 300;
        color: #63666a;
        line-height: 1.375;
        text-decoration: none;
      }
      .projectLogo img {
        margin: -1px 0 -4px;
        vertical-align: middle;
      }
    </style></head><body><h1 id="g" class="projectLogo"><a href="//www.google.com/fonts"><img src="//www.google.com/images/logos/google_logo_41.png" alt="Google"/> Fonts</a></h1><p><b>Error (400):</b>&nbsp;<ins>Missing font family</ins></p><p><p>The requested font families are not available.<p>Requested: </p><ins><p>For reference, see the <a href=https://developers.google.com/fonts/docs/getting_started>Google Fonts API documentation</a>.</p></ins></body></html>`
