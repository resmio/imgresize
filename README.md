# Resize image and cache resized images #
Webservice that allows resized images to be retrived through an url like
```
http://service.com/600x400/http://example.com/myimage.jpg
```

You can also pass the format you want the image converted in the url like this.
(currently it only works with jpg)
```
http://service.com/600x400/jpg/http://example.com/myimage.jpg
```

And if you want to control compression ration, you can pass a value between
1 and 100 also this way
```
http://service.com/600x400/70/jpg/http://example.com/myimage.jpg
```


Both the original sized and the resized images will be cached.

## Usage ##
```
$ go build
$ ./imgresize [-port <port>][-cachedir <cachedir>]
```
