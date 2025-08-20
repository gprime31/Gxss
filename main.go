package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"math/rand"
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

var (
	custhead customh
	client   *http.Client
	payloadRe *regexp.Regexp
	outMu    sync.Mutex
)

func banner() {
	fmt.Println(`                  
 _____ __ __ _____ _____ 
|   __|  |  |   __|   __|
|  |  |-   -|__   |__   |
|_____|__|__|_____|_____|
                         
	4.0 - @KathanP19 (modded)
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

	// compile regex once
	payloadRe = regexp.MustCompile(payload)

	// setup http client once
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	if proxy != "" {
		if proxyUrl, err := url.Parse(proxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyUrl)
		}
	}
	client = &http.Client{
		Transport:     transport,
		CheckRedirect: redirectPolicyFunc,
		Timeout:       15 * time.Second,
	}

	// read stdin into slice
	urls := readInput(os.Stdin)

	// shuffle urls randomly
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(urls), func(i, j int) {
		urls[i], urls[j] = urls[j], urls[i]
	})

	// prepare output file if needed
	var f *os.File
	if outputFile != "" {
		var err error
		f, err = os.Create(outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
	}

	// channel for workers
	urlChan := make(chan string, len(urls))
	for _, u := range urls {
		urlChan <- u
	}
	close(urlChan)

	// launch workers
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for link := range urlChan {
				checkreflection(link, f)
			}
		}()
	}
	wg.Wait()

	if verbose {
		fmt.Println("\nFinished Checking, Thank you for using Gxss.")
	}
}

func readInput(r io.Reader) []string {
	var urls []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			urls = append(urls, line)
		}
	}
	return urls
}

func checkreflection(link string, f *os.File) {
	decoded, _ := url.QueryUnescape(link)
	u, err := url.Parse(decoded)
	if err != nil {
		decoded := url.QueryEscape(link)
		if v, err2 := url.Parse(decoded); err2 == nil {
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
		orig := value[0]

		if appendPayload {
			q.Set(key, orig+payload)
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

		if payloadRe.MatchString(body) {
			if verbose {
				fmt.Printf("Url : %q\n", u)
				fmt.Printf("Reflected Param : %q\n", key)
			} else {
				fmt.Println(u.String())
			}
			if f != nil {
				outMu.Lock()
				f.WriteString(u.String() + "\n")
				outMu.Unlock()
			}
		}
		q.Set(key, orig) // restore original
	}
}

func requestfunc(u string, requestData string, method string) (resp *http.Response, body string, errs []error) {
	req, err := http.NewRequest(method, u, bytes.NewBufferString(requestData))
	if err != nil {
		return nil, "", nil
	}
	req.Header.Add("User-Agent", useragent)

	if method == "POST" {
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, v := range custhead {
		s := strings.SplitN(v, ":", 2)
		if len(s) == 2 {
			req.Header.Add(strings.TrimSpace(s[0]), strings.TrimSpace(s[1]))
		}
	}

	if verbose {
		if requestDump, err := httputil.DumpRequest(req, true); err == nil {
			fmt.Println(string(requestDump))
		}
	}

	resp, err = client.Do(req)
	if err != nil {
		return nil, "", nil
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, "", nil
	}
	return resp, string(bodyBytes), nil
}

func redirectPolicyFunc(req *http.Request, via []*http.Request) error {
	return http.ErrUseLastResponse
}
