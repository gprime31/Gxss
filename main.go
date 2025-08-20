package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	concurrency   int
	verbose       bool
	outputFile    string
	payload       string
	useragent     string
	proxy         string
	requestData   string
	method        string
	appendPayload bool
)

type customh []string

func (m *customh) String() string {
	return "This is custom flag for getting custom headers."
}

func (m *customh) Set(value string) error {
	*m = append(*m, value)
	return nil
}

var custhead customh

func banner() {
	fmt.Println(`                  
 _____ __ __ _____ _____ 
|   __|  |  |   __|   __|
|  |  |-   -|__   |__   |
|_____|__|__|_____|_____|
                         
	4.0 - @KathanP19
	`)
}

func main() {
	flag.IntVar(&concurrency, "c", 50, "Set the Concurrency")
	flag.BoolVar(&verbose, "v", false, "Verbose mode")
	flag.StringVar(&payload, "p", "Gxss", "Payload you want to Send to Check Reflection")
	flag.StringVar(&outputFile, "o", "", "Save Result to OutputFile")
	flag.StringVar(&requestData, "d", "", "Request data for POST based reflection testing")
	flag.StringVar(&proxy, "x", "", "Proxy URL. Example: http://127.0.0.1:8080")
	flag.StringVar(&useragent, "u", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.111 Safari/537.36", "Set Custom User agent. Default is Mozilla")
	flag.Var(&custhead, "h", "Set Custom Header.")
	flag.BoolVar(&appendPayload, "a", false, "Append payload to the parameter value instead of replacing")

	flag.Parse()

	if verbose {
		banner()
	}

	if payload != "" {

		if outputFile != "" {
			emptyFile, err := os.Create(outputFile)
			if err == nil {
				emptyFile.Close()
			}

			var wg sync.WaitGroup
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					testref(payload, verbose, outputFile, requestData)
					wg.Done()
				}()
				wg.Wait()
			}

		} else {

			var wg sync.WaitGroup
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					testref(payload, verbose, outputFile, requestData)
					wg.Done()
				}()
				wg.Wait()
			}
		}
	} else {
		flag.PrintDefaults()
	}
	if verbose {
		fmt.Println("\nFinished Checking, Thank you for using Gxss.")
	}
}

func testref(payload string, verbose bool, outputFile string, requestData string) {
	time.Sleep(500 * time.Microsecond)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		link := scanner.Text()
		checkreflection(link)
	}
}

func checkreflection(link string) {
	decoded, _ := url.QueryUnescape(link)
	u, err := url.Parse(decoded)
	if err != nil {
		decoded := url.QueryEscape(link)
		v, err2 := url.Parse(decoded)
		if err2 == nil {
			u = v
		} else {
			return
		}
	}

	if verbose {
		fmt.Println("[+] Testing URL : " + link)
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return
	}

	if requestData != "" {
		method = "POST"
		q, err = url.ParseQuery(requestData)
		if err != nil {
			return
		}
	} else {
		method = "GET"
	}

	for key, value := range q {
		var tm string = value[0]

		// NEW: append or replace payload depending on -a
		if appendPayload {
			q.Set(key, tm+payload)
		} else {
			q.Set(key, payload)
		}

		if method == "GET" {
			u.RawQuery = q.Encode()
		}
		if method == "POST" {
			requestData = q.Encode()
		}
		_, body, _ := requestfunc(u.String(), requestData, method)

		re := regexp.MustCompile(payload)
		match := re.FindStringSubmatch(body)

		if match != nil {
			if verbose {
				fmt.Printf("Url : %q\n", u)
				fmt.Printf("Reflected Param : %q\n", key)
			} else {
				fmt.Println(u.String() + "\n")
			}
			if outputFile != "" {
				f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_WRONLY, 0644)
				if err == nil {
					_, _ = f.WriteString(u.String() + "\n")
					f.Close()
				}
			}
		}
		q.Set(key, tm) // restore original
	}
}

// removed gorequest for more granular access to setting headers.

func requestfunc(u string, requestData string, method string) (resp *http.Response, body string, errs []error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	if proxy != "" {
		proxyUrl, err := url.Parse(proxy)
		if err == nil {
			http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
		}
	}

	client := &http.Client{
		CheckRedirect: redirectPolicyFunc,
	}

	req, err := http.NewRequest(method, u, bytes.NewBufferString(requestData))
	if err != nil {
		return nil, "", nil
	}
	req.Header.Add("User-Agent", useragent)

	if method == "POST" {
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	}
	// splitting headers and values by using : as separator
	for _, v := range custhead {
		s := strings.SplitN(v, ":", 2)
		if len(s) == 2 {
			req.Header.Add(s[0], s[1])
		}
	}

	// Converting request dump to string for verbose mode
	if verbose {
		requestDump, err := httputil.DumpRequest(req, true)
		if err == nil {
			fmt.Println(string(requestDump))
		}
	}

	resp, err = client.Do(req)
	if err != nil {
		return nil, "", nil
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return resp, "", nil
	}
	bodyString := string(bodyBytes)
	return resp, bodyString, errs
}

func redirectPolicyFunc(req *http.Request, via []*http.Request) error {
	return http.ErrUseLastResponse
}
