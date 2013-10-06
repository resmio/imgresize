package main

import (
    "fmt"
    "regexp"
    "net/http"
    // "io"
    // "os"
    "image/jpeg" 
    "log"
    "strconv"
    // "reflect"
)

import "github.com/nfnt/resize"

type Handler struct {}
 
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

    re, err := regexp.Compile(`^([0-9]+)x([0-9]+)/(http[s]?://[\w/\.\-_]+)$`)

    if err != nil {
        fmt.Printf("There is a problem with you regexp.\n")
        return
    }

    res := re.FindAllStringSubmatch(r.URL.Path[1:], -1)

    if len(res) == 0 {
        fmt.Printf("No match for the regexp.\n")
        return
    }

    height, err := strconv.ParseUint(res[0][1], 10, 64)
    width, err := strconv.ParseUint(res[0][2], 10, 64)
    url := res[0][3]

    // out, err := os.Create("output.jpg")
    // if err != nil {
    //     fmt.Printf("Could not open file for saving.\n")
    //     return
    // }
    // defer out.Close()

    resp, err := http.Get(url)
    if err != nil {
        fmt.Printf("Could not get the image.\n")
        return
    }
    defer resp.Body.Close()

    // decode jpeg into image.Image
    img, err := jpeg.Decode(resp.Body)
    if err != nil {
        log.Fatal(err)
    }

    // resize to width 1000 using Lanczos resampling
    // and preserve aspect ratio
    m := resize.Resize(uint(height), uint(width), img, resize.Lanczos3)

    // write new image to file
    jpeg.Encode(w, m, nil)

    // n, err := io.Copy(out, resp.Body)
    // if err != nil {
    //     fmt.Printf("Could not save the image.\n")
    //     return
    // }
    // fmt.Println(n)

    // fmt.Println(height, width, url)
    // fmt.Println(res)
    // fmt.Fprintf(w, "%v", res)
    // uri := r.URL.Path
    // fmt.Fprint(w, uri)
    return
}

func main() {

    srv := &http.Server{
            Addr:    ":8080",
            Handler: &Handler{},
    }
    log.Fatal(srv.ListenAndServe())
}