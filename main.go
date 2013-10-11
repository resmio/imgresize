package main

import (
    "fmt"
    "regexp"
    "net/http" 
    "log"
    "strconv"
    "hash/crc32"
    "errors"
    "io"
    "os"
    "path/filepath"
    "flag"
)

import "github.com/gographics/imagick/imagick"

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
func parse_request(path string)(width, height uint, url, ext string, err error) {
    res := re.FindStringSubmatch(path)

    if res == nil {
        err = errors.New("No match for the regexp.")
        return
    }

    var parseErr error
    var width64, height64 uint64
    width64, parseErr = strconv.ParseUint(res[1], 10, 64)
    if parseErr != nil {
        err = errors.New("Could not parse width.")
        return
    }
    height64, parseErr = strconv.ParseUint(res[2], 10, 64)
    if parseErr != nil {
        err = errors.New("Could not parse height.")
        return
    }
    width, height = uint(width64), uint(height64)
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

func resizeImage(src, dst string, width, height uint) {
    if width == 0 && height == 0 {
        panic(errors.New("width and height of image cannot be both 0"))
    }
    imagick.Initialize()
    // Schedule cleanup
    defer imagick.Terminate()
    var err error

    mw := imagick.NewMagickWand()
    // Schedule cleanup
    defer mw.Destroy()

    err = mw.ReadImage(src)
    if err != nil {
        panic(err)
    }

    // Get original size
    origWidth := mw.GetImageWidth()
    origHeight := mw.GetImageHeight()
    // keep proportions if widht or height set to 0
    if width == 0 {
        scaling := float64(height) / float64(origHeight)
        width = uint(float64(origWidth) * scaling)
    }
    if height == 0 {
        scaling := float64(width) / float64(origWidth)
        height = uint(float64(origHeight) * scaling)
    }

    // Resize the image using the Lanczos filter
    // The blur factor is a float, where > 1 is blurry, < 1 is sharp
    err = mw.ResizeImage(width, height, imagick.FILTER_LANCZOS, 1)
    if err != nil {
        panic(err)
    }

    // Set the compression quality to 95 (high quality = low compression)
    err = mw.SetImageCompressionQuality(95)
    if err != nil {
        panic(err)
    }

    mw.WriteImage(dst)
    if err != nil {
        panic(err)
    }
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
        resizeImage(imagePathOriginal, imagePathResized, width, height)
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
        log.Println("Createing new cache directory", cacheDir)
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