# Resize image and cache resized images #
Webservice that allows resized images to be retrived through an url like
```
http://service.com/600x400/http://example.com/myimage.jpg
```

Both the original sized and the resized images will be cached.

## Usage ##
Cd into the project and run `go get ./...` to install the dependencies.
```
$ go build
$ ./imgresize [-port <port>][-cachedir <cachedir>]
```
