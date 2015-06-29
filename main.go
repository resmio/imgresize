package main

import (
    "fmt"
    "math"
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
    "strings"
    "sync"
)

import "github.com/gographics/imagick/imagick"

var regex, err = regexp.Compile(`^/([0-9]+)x([0-9]+)(/jpg)?/(http[s]?://[\w/\.\-_ ]+?)((\.\w+)?)$`)
var cacheDir string

func hash(s string) uint32 {
    return crc32.ChecksumIEEE([]byte(s))
}

// get the parameters from the url path
// returns
// - width of the image
// - height of the image
// - If we want the image converted to jpg
// - url where the original image is located (including the extension)
// - extension (including the dot)
func parseRequest(path string)(width, height uint, outputFormat, url, ext string, err error) {
    res := regex.FindStringSubmatch(path)

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
    outputFormat = strings.Replace(res[3], "/", ".", 1)
    url = res[4] + res[5]
    ext = res[5]
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

var resizeImageMutex sync.Mutex
func resizeImage(src, dst, outputFormat string, width, height uint) {
    resizeImageMutex.Lock()
    defer resizeImageMutex.Unlock()

    log.Println("Resizing file", src)
    if width == 0 && height == 0 {
        log.Panicln(errors.New("width and height of image cannot be both 0"))
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
        log.Panicln(err)
    }

    // get the original size
    origWidth := mw.GetImageWidth()
    origHeight := mw.GetImageHeight()

    // keep proportions if widht or height set to 0
    if width == 0 {
        scaling := float64(height) / float64(origHeight)
        width = uint(math.Floor(scaling*float64(origWidth)+0.5))
    } else if height == 0 {
        scaling := float64(width) / float64(origWidth)
        height = uint(math.Floor(scaling*float64(origHeight)+0.5))
    } else if float64(origWidth)/float64(origHeight) >
                                    float64(width)/float64(height) {
        // crop off some width
        scaling := float64(height) / float64(origHeight)
        newWidth := uint(math.Floor(float64(width)/scaling+0.5))
        deltaWidth := origWidth - newWidth
        if deltaWidth >= 1 {
            mw.CropImage(newWidth, origHeight, int(deltaWidth)/2, 0)
        }
    } else {
        // crop off some height
        scaling := float64(width) / float64(origWidth)
        newHeight := uint(math.Floor(float64(height)/scaling+0.5))
        deltaHeight := origHeight - newHeight
        if deltaHeight >= 1 {
            mw.CropImage(origWidth, newHeight, 0, int(deltaHeight)/2)
        }
    }

    // resize the image
    err = mw.ResizeImage(width, height, imagick.FILTER_LANCZOS, 1)
    if err != nil {
        log.Panicln(err)
    }

    err = mw.SetImageFormat(outputFormat)
    log.Println("--------------------format")
    if err != nil {
        log.Panicln(err)
    }

    // Set the compression quality to 95 (high quality = low compression)
    err = mw.SetImageCompressionQuality(70)
    if err != nil {
        log.Panicln(err)
    }

    err = mw.WriteImage(dst)
    if err != nil {
        log.Panicln(err)
    }
    log.Println("Done resizing file", src)
}

// get a file using Get and save it in path
func getAndSaveFile(url, path string) error {
    log.Println("calling get on", url)
    resp, err := http.Get(url)
    log.Println("finished calling get")
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
        log.Panicln(err)
    }
    defer fi.Close()
    log.Println("Copying file")
    if _, err := io.Copy(fi, resp.Body); err != nil {
        log.Panicln(err)
    }
    log.Println("done copying file")
    return nil
}

type Handler struct {}

func (h *Handler) ServeHTTP(w http.ResponseWriter, request *http.Request) {
    log.Println("# new request:", request.URL.Path)

    // 0. parse request
    width, height, outputFormat, url, ext, err := parseRequest(request.URL.Path)
    if err != nil {
        log.Println("Bad request", request.URL.Path)
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

    // If convert to jpg is true, we switch the extension to jpg
    // before checking if the image is already cached, if we don't do this
    // we'd be checking for the original format
    if outputFormat != "" { ext = outputFormat }

    // 2. if file is not present in resized version
    //    resize and save resized version in cache
    //
    // corollary: in this case it also wasn't present in original size
    //   as it might happen that only 2 get executed, but in practice it will
    //   never happen that only 1 get excecuted
    imagePathResized := hashedFilePath(request.URL.Path, ext)
    if !fileExists(imagePathResized) {
        resizeImage(imagePathOriginal, imagePathResized, outputFormat, width, height)
    }

    // 3. serve resized file which now certainly is in the cache
    http.ServeFile(w, request, imagePathResized)
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
