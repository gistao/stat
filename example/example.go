package main

import (
	"fmt"
	"github.com/gistao/stat"
	"net/http"
	"time"
)

func main() {
	var h stat.Handler
	sig := stat.NewSIG(10)
	st := &stat.Stat{}
	st.Init(time.Millisecond*1000, stat.DefaultBufSize, &h, sig)

	st.RegisterKey(&stat.StatKey{Group: "auth", Name: "psf", ST: stat.STQPS | stat.STSum})
	st.RegisterKey(&stat.StatKey{Group: "auth", Name: "login", STName: map[stat.StatType]string{stat.STQPS: "log", stat.STSum: "LOGIN"}, ST: stat.STQPS | stat.STSum})
	st.RegisterKey(&stat.StatKey{Group: "auth", Name: "pso", STName: map[stat.StatType]string{stat.STQPS: "pso", stat.STSum: "PSO"}, ST: stat.STQPS})
	st.RegisterKey(&stat.StatKey{Group: "auth", Name: "psw", STName: map[stat.StatType]string{stat.STQPS: "psw", stat.STSum: "PSW"}, ST: stat.STQPS | stat.STSum})

	st.RegisterKey(&stat.StatKey{Group: "gateway", Name: "wsup", STName: map[stat.StatType]string{stat.STQPS: "wsup", stat.STSum: "WSUP"}, ST: stat.STQPS | stat.STSum})
	st.RegisterKey(&stat.StatKey{Group: "gateway", Name: "wsof", STName: map[stat.StatType]string{stat.STQPS: "wsof", stat.STSum: "WSOF"}, ST: stat.STQPS | stat.STSum})
	st.RegisterKey(&stat.StatKey{Group: "gateway", Name: "timeout", STName: map[stat.StatType]string{stat.STQPS: "to", stat.STSum: "TO"}, ST: stat.STQPS | stat.STSum})
	st.RegisterKey(&stat.StatKey{Group: "gateway", Name: "close", ST: stat.STQPS | stat.STQPSPeak | stat.STSum})
	st.RegisterKey(&stat.StatKey{Group: "gateway", Name: "conn", ST: stat.STVal})

	con := int64(1)
	go func() {
		for {
			// time.Sleep(time.Microsecond * 100)
			st.AddVal("auth", "login", 1)
			st.AddVal("auth", "pso", 1)
			st.AddVal("auth", "psw", 1)
			st.AddVal("gateway", "wsup", 2)
			st.AddVal("gateway", "wsof", 3)
			st.AddVal("gateway", "timeout", 1)
			st.AddVal("gateway", "conn", con)
			con++
		}
	}()
	go func() {
		for {
			time.Sleep(time.Millisecond * 10)
			st.AddVal("auth", "psf", 1)
			st.AddVal("gateway", "close", 1)
		}
	}()

	http.HandleFunc("/", h)
	go http.ListenAndServe(":8080", nil)

	for {
		select {
		// case <-time.After(time.Second):
		case <-sig.Wait():
			ret := st.GetStringsInfo()
			for _, v := range ret {
				fmt.Println(time.Now(), v)
			}
			fmt.Println()

			js, err := st.GetJsonInfo()
			if err != nil {
				fmt.Println(err)
				continue
			}
			fmt.Println(string(js))
		}
	}
}
