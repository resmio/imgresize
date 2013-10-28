package main

import (
    "testing"
    "log"
)

func TestParseRequest(t *testing.T) {
    w, h, url, ext, err := parseRequest("/100x0/http://some/url.jpg")
    if (w != 100 ||
        h != 0 ||
        url != "http://some/url.jpg" ||
        ext != ".jpg" ||
        err != nil) {
        log.Println("/100x0/http://some/url.jpg failed", w, h, url, ext, err)
        t.FailNow()
    }
    w, h, url, ext, err = parseRequest("/0x12/https://some/url.JPEG")
    if (w != 0 ||
        h != 12 ||
        url != "https://some/url.JPEG" ||
        ext != ".JPEG" ||
        err != nil) {
        log.Println("/0x12/https://some/url.JPEG failed", w, h, url, ext, err)
        t.FailNow()
    }
}