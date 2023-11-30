package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/piliming/convert_time/clip"

	"github.com/araddon/dateparse"
	"github.com/gen2brain/beeep"
)

var loc = time.Now().Location()

func main() {
	watch(handle)
}

func watch(fn func(string)) {
	ch := clip.AdaptWatchDoubleText(context.Background())
	for s := range ch {
		//fmt.Println(s)

		fn(s)
	}

}

func handle(s string) {
	if len(s) > 40 {
		return
	}

	n, _ := strconv.ParseInt(s, 10, 64)
	if n > 0 {
		handleNum(n)
	} else {
		handleText(s)
	}
}

func handleNum(num int64) {
	if num > 10000000 && num < 10013221020 {
		handleS(num)
	}
	if num > 10013221020 && num < 2101322102000 {
		handleS(num / 1000)
	}
}

// 1701322102000

func handleS(num int64) {
	tStr := time.Unix(num, 0).In(loc).Format("2006-01-02 15:04:05")
	//fmt.Println(tStr)
	notify(tStr)
}

func handleText(s string) {
	t, err := dateparse.ParseLocal(s)
	if err != nil {
		log.Println(err)
		return
	}
	ts := t.Unix()
	//fmt.Println(t.Location().String())
	content := fmt.Sprintf("%d - 已复制到剪切板", ts)
	clip.Write(clip.FmtText, []byte(strconv.FormatInt(ts, 10)))
	notify(content)
}

func notify(s string) {
	err := beeep.Notify("时间转换", s, "assets/information.png")
	if err != nil {
		log.Println(err)
	}
}
