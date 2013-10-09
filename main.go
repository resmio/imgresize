package main

import (
    "fmt"
    "regexp"
    "net/http"
    "image"
    "image/jpeg" 
    "log"
    "strconv"
    "hash/crc32"
    "errors"
    "os"
    "path/filepath"
    "flag"
)

import "github.com/nfnt/resize"

var re, err = regexp.Compile(`^/([0-9]+)x([0-9]+)/(http[s]?://[\w/\.\-_]+)((?i)\.jpeg|\.jpg)$`)
var cacheDir string

func hash(s string) uint32 {
    return crc32.ChecksumIEEE([]byte(s))
}

// get the parameters from the url path
// returns
// - width of the image
// - height of the image
// - url where the original image is located (including the extension)
// - extension (including the dot)
func parse_request(path string)(width, height uint64, url, ext string, err error) {
    res := re.FindStringSubmatch(path)

    if res == nil {
        err = errors.New("No match for the regexp.")
        return
    }

    var parseErr error
    height, parseErr = strconv.ParseUint(res[1], 10, 64)
    if parseErr != nil {
        err = errors.New("Could not parse width.")
        return
    }
    width, parseErr = strconv.ParseUint(res[2], 10, 64)
    if parseErr != nil {
        err = errors.New("Could not parse height.")
        return
    }
    url = res[3] + res[4]
    ext = res[4]
    fmt.Println(hash(url))  
    return
}

// returns whether the given file or directory exists or not
func pathExists(path string) (bool) {
    _, err := os.Stat(path)
    if err == nil { return true }
    if os.IsNotExist(err) { return false }
    return false
}

func urlToPath(url string, ext string) string {
    checksum := hash(url)
    log.Println("Checksum", checksum)
    filename := strconv.FormatUint(uint64(checksum), 36) + ext
    log.Println("Filename", filename)
    path := filepath.Join(cacheDir, filename)
    return path
}

func saveImage(img image.Image, path string) error {
    fi, err := os.Create(path)
    if err != nil {
        log.Println("Error creating", path)
        return err
    }
    defer fi.Close()

    // save image
    jpeg.Encode(fi, img, nil)

    // close fi on exit and check for its returned error
    return nil
}

func loadImage(path string) (image.Image, error) {
    fi, err := os.Open(path)
    if err != nil {
        log.Println("Error reading", path)
        return nil, err
    }
    defer fi.Close()

    var img image.Image
    img, err = jpeg.Decode(fi)
    if err != nil {
        return nil, err
    }

    return img, nil
}

type Handler struct {}
 
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    path := r.URL.Path
    height, width, url, ext, err := parse_request(path)
    if err != nil {
        log.Println("Bad request", path)
        log.Println(err)
        return
    }

    var img image.Image

    if path := urlToPath(r.URL.Path, ext); pathExists(path) {
        // file is cached in it's requested size
        log.Println("file is cached in it's requested size")
        img, err = loadImage(path)
    } else if path := urlToPath(url, ext); pathExists(path) {
        // file is cached only in the original size
        log.Println("file is cached only in the original size")
        img, err = loadImage(path)
        // resize image
        img = resize.Resize(uint(height), uint(width), img, resize.NearestNeighbor)
        // save resized version in cache
        saveImage(img, urlToPath(r.URL.Path, ext))
    } else {
        // file is not cached at all
        log.Println("file is not cached at all")
        // download the file
        resp, err := http.Get(url)
        if err != nil {
            log.Println("Error getting the image")
            fmt.Println(err)
            return
        }
        if code := resp.StatusCode; code != 200 {
            log.Println("Error getting the image: got status code", code)
            return
        }
        defer resp.Body.Close()

        // decode jpeg into image.Image
        img, err = jpeg.Decode(resp.Body)
        if err != nil {
            log.Println("Error decoding the image")
            log.Println(err)
            return
        }
        // save original image
        saveImage(img, urlToPath(url, ext))
        // resize image
        img = resize.Resize(uint(height), uint(width), img, resize.NearestNeighbor)
        // save resized version in cache
        saveImage(img, urlToPath(r.URL.Path, ext))
    }

    // write new image to request writer
    err = jpeg.Encode(w, img, nil)
    if err != nil {
        log.Println("Error encoding file")
        log.Println(err)
    }
    return
}

func main() {
    var port int
    flag.StringVar(&cacheDir, "cachedir", "./cachedir", "path to cache directory")
    flag.IntVar(&port, "port", 8080, "port")
    flag.Parse()

    if !pathExists(cacheDir) {
        log.Println("Create new cache directory", cacheDir)
        err = os.Mkdir(cacheDir, 0700)
        if err != nil {
            log.Println("Error creating directory", cacheDir)
            log.Println(err)
            return
        }
    }
    srv := &http.Server{
            Addr:    fmt.Sprintf(":%d", port),
            Handler: &Handler{},
    }
    log.Fatal(srv.ListenAndServe())
}