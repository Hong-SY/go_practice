package main

import (
	"fmt"
	"golang.org/x/net/html"
	"log"
	"net/http"
	"runtime"
	"strings"
	"sync"
)

type result struct {
	url  string
	name string
}

var fetched = struct {
	m map[string]error
	sync.Mutex
}{m: make(map[string]error)}

func fetch(url string) (*html.Node, error) {
	res, err := http.Get(url)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	doc, err := html.Parse(res.Body)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return doc, nil
}

func main() {
	numCPUs := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPUs)

	urls := make(chan string)
	done := make(chan struct{})
	c := make(chan result)

	var wg sync.WaitGroup
	const numWorkers = 10
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			worker(done, urls, c)
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(c)
	}()

	urls <- "https://github.com/huonghope?tab=following"
	count := 0
	for r := range c {
		fmt.Println(r.name)

		count++
		if count > 100 {
			close(done)
			break
		}
	}
}

func worker(done <-chan struct{}, urls chan string, c chan<- result) {
	for url := range urls {
		select {
		case <-done:
			return
		default:
			crawl(url, urls, c)
		}
	}
}

func parseFollowing(doc *html.Node, urls chan string) <-chan string {
	name := make(chan string)

	go func() {
		var f func(*html.Node)
		f = func(n *html.Node) {
			if n.Type == html.ElementNode {
				for _, a := range n.Attr {
					if a.Key == "class" && a.Val == "p-name vcard-fullname d-block overflow-hidden" {
						name <- strings.TrimSpace(n.FirstChild.Data)
						break
					}
					if a.Key == "class" && a.Val == "Link--secondary pl-1" {
						user := strings.TrimSpace(n.FirstChild.Data)
						urls <- "https://github.com/" + user + "?tab=following"
						break
					}
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
		}
		f(doc)
	}()

	return name
}

func crawl(url string, urls chan string, c chan<- result) {
	fetched.Lock()
	if _, ok := fetched.m[url]; ok {
		fetched.Unlock()
		return
	}
	fetched.Unlock()

	doc, err := fetch(url)
	if err != nil {
		go func(u string) {
			urls <- u
		}(url)
		return
	}

	fetched.Lock()
	fetched.m[url] = err
	fetched.Unlock()

	name := <-parseFollowing(doc, urls)
	c <- result{url, name}
}
