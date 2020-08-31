package main

import (
	"log"
	"net/url"
	"os"

	"github.com/lixiangzhong/payssion"
)

func main() {
	c := payssion.NewClient("", "")
	c.SetLive(true)
	c.Debug(os.Stdout)
	data := url.Values{}
	data.Set("pm_id", "alipay_cn") //alipay_cn  , tenpay_cn
	data.Set("amount", "1")
	data.Set("currency", "CNY")
	data.Set("description", "")
	data.Set("order_id", "test00000002")
	data.Set("return_url", "https://www.baidu.com")
	rsp, err := c.Create(data)
	if err != nil {
		log.Println(err)
	}
	log.Println(rsp)
}
