package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "log"
    "net/http"
    "net/url"
    "os"
    "sync"
)

const Rest = "/secure.flickr.com/services/rest/"

var (
    endpoint = flag.String("endpoint", "https://foauth.org", "The foauth endpoint to use")
    output   = flag.String("out", "", "the directory to put things in")
    email    = flag.String("email", "", "email to authenticate foauth with")
    password = flag.String("password", "", "password to authenticate foauth with")
    id       = flag.String("id", "", "id of the flickr set")
)

type Flickr struct {
    email, password, dir string
}

type Photo struct {
    Id     string `json:"id"`
    Secret string `json:"secret"`
    Server string `json:"server"`
    Farm   int    `json:"farm"`
}

type Photoset struct {
    Id     string  `json:"id"`
    Title  string  `json:"title"`
    Photos []Photo `json:"photo"`
}

type Size struct {
    Label  string `json:"label"`
    Source string `json:"source"`
    Url    string `json:"url"`
    Media  string `json:"media"`
}

type Sizes struct {
    Sizes []Size `json:"size"`
}

func (f Flickr) Run(id string) {
    if f.email == "" || f.password == "" || id == "" {
        log.Fatalf("missing arguments")
    }

    var rsp struct {
        Photoset Photoset `json:"photoset"`
        Stat     string   `json:"stat"`
    }
    query := url.Values{
        "method":         {"flickr.photosets.getPhotos"},
        "photoset_id":    {id},
        "nojsoncallback": {"1"},
        "format":         {"json"},
    }
    f.GetJSON(&rsp, fmt.Sprintf("%s%s?%s", *endpoint, Rest, query.Encode()))

    log.Printf("downloading %d photos", len(rsp.Photoset.Photos))

    ids := make(chan string)
    var wg sync.WaitGroup

    go f.downloader(ids, &wg)
    go f.downloader(ids, &wg)
    go f.downloader(ids, &wg)
    go f.downloader(ids, &wg)

    for _, p := range rsp.Photoset.Photos {
        ids <- p.Id
    }

    close(ids)
    wg.Wait()
}

func (f Flickr) GetJSON(i interface{}, uri string) {
    req, err := http.NewRequest("GET", uri, nil)
    if err != nil {
        log.Fatalf("failed building request: %s", err)
    }
    req.SetBasicAuth(f.email, f.password)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        log.Fatalf("request failed: %s", err)
    }
    defer resp.Body.Close()
    decoder := json.NewDecoder(resp.Body)
    err = decoder.Decode(i)
    if err != nil {
        log.Fatalf("failed decoding: %s", err)
    }
}

func (f Flickr) downloadPhoto(id string) {
    if id == "" {
        return
    }

    var rsp struct {
        Sizes Sizes  `json:"sizes"`
        Stat  string `json:"stat"`
    }
    query := url.Values{
        "method":         {"flickr.photos.getSizes"},
        "photo_id":       {id},
        "nojsoncallback": {"1"},
        "format":         {"json"},
    }
    f.GetJSON(&rsp, fmt.Sprintf("%s%s?%s", *endpoint, Rest, query.Encode()))

    for _, size := range rsp.Sizes.Sizes {
        if size.Label == "Original" {
            file, err := os.OpenFile(fmt.Sprintf("%s.jpg", id), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
            if err != nil {
                log.Fatalf("failed opening file: %s", err)
            }
            defer file.Close()

            log.Printf("downloading %s", size.Source)
            resp, err := http.Get(size.Source)
            if err != nil {
                return
            }
            defer resp.Body.Close()
            io.Copy(file, resp.Body)
            return
        }
    }
}

func (f Flickr) downloader(ids chan string, wg *sync.WaitGroup) {
    wg.Add(1)
    defer wg.Done()
    for id := range ids {
        f.downloadPhoto(id)
    }
}

func main() {
    flag.Parse()
    f := Flickr{*email, *password, *output}
    f.Run(*id)
}
