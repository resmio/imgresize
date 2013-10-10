package main

import (
    "fmt"
    "regexp"
    "net/http"
    "image"
    "image/jpeg" 
    "image/png" 
    "log"
    "strconv"
    "hash/crc32"
    "errors"
    "io"
    "os"
    "path/filepath"
    "flag"
)

import "github.com/nfnt/resize"

var re, err = regexp.Compile(`^/([0-9]+)x([0-9]+)/(http[s]?://[\w/\.\-_ ]+)(\.\w+)$`)
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
    width, parseErr = strconv.ParseUint(res[1], 10, 64)
    if parseErr != nil {
        err = errors.New("Could not parse width.")
        return
    }
    height, parseErr = strconv.ParseUint(res[2], 10, 64)
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

func saveImage(img image.Image, path string, extension string) error {
    fi, err := os.Create(path)
    if err != nil {
        log.Println("Error creating", path)
        return err
    }
    defer fi.Close()

    // save image
    encodeImage(fi, img, extension)

    // close fi on exit and check for its returned error
    return nil
}

func loadImage(path string, extension string) (image.Image, error) {
    fi, err := os.Open(path)
    if err != nil {
        log.Println("Error loading image from cache", path)
        return nil, err
    }
    defer fi.Close()

    var img image.Image
    img, err = decodeImage(fi, extension)
    if err != nil {
        log.Println("Error decoding file", path)
        return nil, err
    }

    return img, nil
}


func decodeImage(r io.Reader, extension string) (img image.Image, err error) {
    if res, _ := regexp.MatchString(`^.(?i)(jpg|jpeg)$`, extension); res {
        img, err = jpeg.Decode(r)
    } else if res, _ := regexp.MatchString(`^.(?i)(png)$`, extension); res {
        img, err = png.Decode(r)
    } else {
        err = fmt.Errorf("Invalid file extension %s", extension)
    }
    return
}

func encodeImage(w io.Writer, img image.Image, extension string) (err error) {
    if res, _ := regexp.MatchString(`^.(?i)(jpg|jpeg)$`, extension); res {
        err = jpeg.Encode(w, img, nil)
    } else if res, _ := regexp.MatchString(`^.(?i)(png)$`, extension); res {
        err = png.Encode(w, img)
    } else {
        err = fmt.Errorf("Invalid file extension %s", extension)
    }
    return
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
        img, err = loadImage(path, ext)
    } else if path := urlToPath(url, ext); pathExists(path) {
        // file is cached only in the original size
        log.Println("file is cached only in the original size")
        img, err = loadImage(path, ext)
        if err != nil {
            log.Println("Error loading cached file")
            log.Println(err)
            return
        }
        // resize image
        img = resize.Resize(uint(height), uint(width), img, resize.NearestNeighbor)
        // save resized version in cache
        saveImage(img, urlToPath(r.URL.Path, ext), ext)
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

        // decode into image.Image
        img, err = decodeImage(resp.Body, ext)
        if err != nil {
            log.Println("Error decoding the image")
            log.Println(err)
            return
        }
        // save original image
        saveImage(img, urlToPath(url, ext), ext)
        // resize image
        img = resize.Resize(uint(height), uint(width), img, resize.NearestNeighbor)
        // save resized version in cache
        saveImage(img, urlToPath(r.URL.Path, ext), ext)
    }

    // write new image to request writer
    err = encodeImage(w, img, ext)
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