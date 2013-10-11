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
    return
}

// returns whether the given file or directory exists
func fileExists(path string) (bool) {
    _, err := os.Stat(path)
    if err == nil { return true }
    if os.IsNotExist(err) { return false }
    return false
}

// returns a path of the type <cachedir>/<hash of url><ext>
func hashedFilePath(url string, ext string) string {
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

// get a file using Get and save it in path
func getAndSaveFile(url, path string) error {
        resp, err := http.Get(url)
        if err != nil {
            log.Println("Error getting the file", url)
            fmt.Println(err)
            return err
        }
        defer resp.Body.Close()
        if code := resp.StatusCode; code != 200 {
            log.Printf("Error getting the file %s: got status code %s", 
                       url, code)
            return err
        }

        fi, err := os.Create(path)
        if err != nil {
            panic(err)
        }
        defer fi.Close() 
        if _, err := io.Copy(fi, resp.Body); err != nil {
            panic(err)
        }
        return nil
}

type Handler struct {}
 
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    log.Println("# new request:", r.URL.Path)

    // 0. parse request
    width, height, url, ext, err := parse_request(r.URL.Path)
    if err != nil {
        log.Println("Bad request", r.URL.Path)
        log.Println(err)
        return
    }

    // 1. if original file is not present
    //    dowload and save original file in cache

    // path where the image having the original size should be
    imagePathOriginal := hashedFilePath(url, ext);
    if !fileExists(imagePathOriginal) {
        log.Println("getting image", url)
        err = getAndSaveFile(url, imagePathOriginal)
        if err != nil {
            return
        }
    }

    // 2. if file is not present in resized version
    //    resize and save resized version in cache
    // 
    // corollary: in this case it also wasn't present in original size
    //   as it might happen that only 2 get executed, but in practice it will
    //   never happen that only 1 get excecuted
    imagePathResized := hashedFilePath(r.URL.Path, ext)
    if !fileExists(imagePathResized) {
        log.Println("resizing image", width, height)
        img, err := loadImage(imagePathOriginal, ext)
        if err != nil {
            log.Println("Error loading cached file")
            log.Println(err)
            return
        }
        // resize image
        img = resize.Resize(uint(width), uint(height), img, resize.NearestNeighbor)
        // save resized version in cache
        saveImage(img, imagePathResized, ext)
    }

    // 3. serve resized file which now certainly is in the cache
    http.ServeFile(w, r, imagePathResized)
    return
}

func main() {
    var port int
    flag.StringVar(&cacheDir, "cachedir", "./cachedir", "path to cache directory")
    flag.IntVar(&port, "port", 8080, "port")
    flag.Parse()

    if !fileExists(cacheDir) {
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