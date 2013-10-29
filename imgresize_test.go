package main

import (
    "testing"
    "os"
    "log"
    "image"
    "image/jpeg"
    "image/png"
    "image/color"
)

func TestParseRequest(t *testing.T) {
    w, h, url, ext, err := parseRequest("/100x0/http://some/url.jpg")
    if (w != 100 || h != 0 || url != "http://some/url.jpg" ||
                ext != ".jpg" || err != nil) {
        log.Println("/100x0/http://some/url.jpg failed", w, h, url, ext, err)
        t.FailNow()
    }
    w, h, url, ext, err = parseRequest("/0x12/https://some/url.JPEG")
    if (w != 0 || h != 12 ||  url != "https://some/url.JPEG" ||
                ext != ".JPEG" || err != nil) {
        log.Println("/0x12/https://some/url.JPEG failed", w, h, url, ext, err)
        t.FailNow()
    }
}

func TestFileExists(t *testing.T) {
    if fileExists("./unexisting_file123") {
        log.Println("unexisting_file123 should not exist")
        t.FailNow()
    }
    if !fileExists("./main.go") {
        log.Println("main.go not found")
        t.FailNow()
    }
}

func TestImageResizeSizes(t *testing.T) {
    // original size, resize params, expected new size
    sizes := [][]int{
        {100, 200, 10, 20, 10, 20},
        {100, 200, 10, 0, 10, 20},
        {100, 200, 0, 20, 10, 20},
        {100, 200, 5, 20, 5, 20},
        {100, 200, 500, 0, 500, 1000},
        {200, 50, 100, 0, 100, 25},
        {200, 50, 100, 100, 100, 100},
    }

    os.Mkdir("test_tmp", 0700)
    defer os.RemoveAll("test_tmp")
    
    for _, options := range sizes {
        origW, origH := options[0], options[1]
        paramW, paramH := options[2], options[3]
        expectedW, expectedH := options[4], options[5]

        // create a new image
        origImg := image.NewGray(image.Rect(0, 0, origW, origH))

        // save it as jpeg
        src, _ := os.Create("test_tmp/src.jpg")
        jpeg.Encode(src, origImg, nil)
        src.Close()

        resizeImage("test_tmp/src.jpg", "test_tmp/dst.jpg", uint(paramW), uint(paramH))

        dst, _ := os.Open("test_tmp/dst.jpg")
        img, _ := jpeg.Decode(dst)
        dst.Close()
        if img.Bounds().Max.X != expectedW || img.Bounds().Max.Y != expectedH {
            log.Println(img.Bounds().Max.X, img.Bounds().Max.Y)
            t.FailNow()
        }
    }
}

func TestImageResizeCropping(t *testing.T) {
    os.Mkdir("test_tmp", 0700)
    defer os.RemoveAll("test_tmp")

    white := color.Gray{255}
    gray := color.Gray{100}
    // create a new image
    origImg := image.NewGray(image.Rect(0, 0, 100, 100))
    // add some white points that should be cropped away
    origImg.Set(24, 24, white)
    origImg.Set(100-25, 100-25, white)
    origImg.Set(100-25, 100-25, white)
    // ad some gray points that should not be cropped away
    origImg.Set(25, 25, gray)
    origImg.Set(100-26, 100-26, gray)

    // save it as jpeg
    src, _ := os.Create("test_tmp/src.png")
    png.Encode(src, origImg)
    src.Close()

    // resize
    resizeImage("test_tmp/src.png", "test_tmp/dst.png", 50, 100)

    // reconvert to image.Image
    dst, _ := os.Open("test_tmp/dst.png")
    img, _ := png.Decode(dst)
    dst.Close()

    // check cropping
    // check that white pixels have been cropped away
    b := img.Bounds()
    for y := b.Min.Y; y < b.Max.Y; y++ {
        for x := b.Min.X; x < b.Max.X; x++ {
            if colorsEqual(img.At(x, y), white) {
                log.Println("point was not cropped", x, y)
                t.FailNow()
            }
        }
    }

    // check that gray pixels are still in the image
    if !colorsEqual(img.At(0, 25), gray) {
        log.Println("point was wrongly cropped: 0 25", img.At(0, 25))
        t.FailNow()
    }

    if !colorsEqual(img.At(49, 100-26), gray) {
        log.Println("point was wrongly cropped 49 100-26", img.At(49, 100-26))
        t.FailNow()
    }
}

func colorsEqual(first, second color.Color) bool {
    r1, g1, b1, a1 := first.RGBA()
    r2, g2, b2, a2 := second.RGBA()
    return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}