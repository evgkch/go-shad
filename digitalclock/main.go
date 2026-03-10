//go:build !solution

package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var symbols = map[byte]string{
	'0': Zero, '1': One, '2': Two, '3': Three, '4': Four,
	'5': Five, '6': Six, '7': Seven, '8': Eight, '9': Nine,
}

func drawSymbol(img *image.RGBA, sym string, xOff, k int) int {
	lines := strings.Split(sym, "\n")
	for y, line := range lines {
		for x, ch := range line {
			c := color.RGBA{255, 255, 255, 255}
			if ch == '1' {
				c = Cyan
			}
			for dy := 0; dy < k; dy++ {
				for dx := 0; dx < k; dx++ {
					img.Set((xOff+x)*k+dx, y*k+dy, c)
				}
			}
		}
	}
	return len(strings.Split(sym, "\n")[0])
}

func handler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	k := 1
	if s := q.Get("k"); s != "" {
		var err error
		k, err = strconv.Atoi(s)
		if err != nil || k < 1 || k > 30 {
			http.Error(w, "invalid k", http.StatusBadRequest)
			return
		}
	}

	t := q.Get("time")
	if t == "" {
		t = time.Now().Format("15:04:05")
	}

	if _, err := time.Parse("15:04:05", t); err != nil || len(t) != 8 {
		http.Error(w, "invalid time", http.StatusBadRequest)
		return
	}

	h := len(strings.Split(Zero, "\n"))
	wD := len(strings.Split(Zero, "\n")[0])
	wC := len(strings.Split(Colon, "\n")[0])

	img := image.NewRGBA(image.Rect(0, 0, (6*wD+2*wC)*k, h*k))

	x := 0
	for _, ch := range []byte(t) {
		if ch == ':' {
			x += drawSymbol(img, Colon, x, k)
		} else {
			x += drawSymbol(img, symbols[ch], x, k)
		}
	}

	w.Header().Set("Content-Type", "image/png")
	png.Encode(w, img)
}

func main() {
	port := flag.Int("port", 8080, "port")
	flag.Parse()
	http.HandleFunc("/", handler)
	http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
}
