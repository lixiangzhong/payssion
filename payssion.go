//payssion
//doc https://www.payssion.com/cn/docs/#api-reference-payment-request

package payssion

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	jsoniter "github.com/json-iterator/go"
)

const (
	liveURLHost    = "https://www.payssion.com"
	sandboxURLHost = "https://sandbox.payssion.com"
)

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

func NewClient(apikey, apisecret string) Client {
	return Client{
		debug:      ioutil.Discard,
		apiKey:     apikey,
		apiSecret:  apisecret,
		live:       false,
		httpclient: new(http.Client),
	}
}

type Client struct {
	debug      io.Writer
	apiKey     string
	apiSecret  string
	live       bool
	httpclient *http.Client
}

func (c *Client) SetLive(live bool) {
	c.live = live
}

func (c *Client) Debug(w io.Writer) {
	c.debug = w
}

func (c Client) do(r *http.Request) (jsoniter.Any, error) {
	buf := new(bytes.Buffer)
	buf.WriteString("--------------------\n")
	buf.WriteString(time.Now().Format("2006-01-02 15:04:05\n"))
	defer func() {
		io.Copy(c.debug, buf)
	}()
	b, err := httputil.DumpRequest(r, true)
	if err == nil {
		buf.Write(b)
	}
	var data jsoniter.Any
	rsp, err := c.httpclient.Do(r)
	if err != nil {
		return data, err
	}
	defer rsp.Body.Close()
	b, err = httputil.DumpResponse(rsp, true)
	if err == nil {
		buf.Write(b)
	}
	b, err = ioutil.ReadAll(rsp.Body)
	if err != nil {
		return data, err
	}
	err = json.Unmarshal(b, &data)
	return data, err
}

type CreateResponse struct {
	ResultCode  int                    `json:"result_code"`
	Transaction map[string]interface{} `json:"transaction"`
	RedirectURL string                 `json:"redirect_url"`
}

var (
	sigKeys = map[string][]string{
		"create": {"api_key", "pm_id", "amount", "currency", "order_id", "secret_key"},
		"notify": {"api_key", "pm_id", "amount", "currency", "order_id", "state", "secret_key"},
	}
)

//Create  pm_id,amount,currency,description,order_id
func (c Client) Create(data url.Values) (CreateResponse, error) {
	//pm_id alipay_cn tenpay_cn
	var rsp CreateResponse

	if data == nil {
		return rsp, errors.New("参数不能为空")
	}
	data.Set("api_key", c.apiKey)
	var sig []string
	for _, v := range sigKeys["create"] {
		if v == "secret_key" {
			sig = append(sig, c.apiSecret)
			continue
		}
		sig = append(sig, data.Get(v))
	}
	data.Set("api_sig", md5sum(strings.Join(sig, "|")))
	u := fmt.Sprintf("%v/api/v1/payment/create", c.apiHost())
	r, err := http.NewRequest(http.MethodPost, u, strings.NewReader(data.Encode()))
	if err != nil {
		return rsp, err
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	any, err := c.do(r)
	if err != nil {
		return rsp, err
	}
	any.ToVal(&rsp)
	return rsp, nil
}

func (c Client) apiHost() string {
	switch c.live {
	case true:
		return liveURLHost
	default:
		return sandboxURLHost
	}
}

func md5sum(s string) string {
	m := md5.New()
	m.Write([]byte(s))
	return hex.EncodeToString(m.Sum(nil))
}

type NotifyData struct {
	Appname       string `form:"app_name" json:"app_name"`
	PmID          string `form:"pm_id" json:"pm_id"`
	TransactionID string `json:"transaction_id" db:"transaction_id" form:"transaction_id"`
	OrderID       string `form:"order_id" json:"order_id"`
	Amount        string `form:"amount" json:"amount"`
	Paid          string `json:"paid" db:"paid" form:"paid"`
	Currency      string `json:"currency" db:"currency" form:"currency"`
	Description   string `json:"description" db:"description" form:"description"`
	State         string `json:"state" db:"state" form:"state"`
	NotifySig     string `form:"notify_sig" json:"notify_sig"`
}

func (n NotifyData) Verify(apikey, apiSecret string) bool {
	//		"notify": {"api_key", "pm_id", "amount", "currency", "order_id", "state", "secret_key"},
	sig := []string{apikey, n.PmID, n.Amount, n.Currency, n.OrderID, n.State, apiSecret}
	want := md5sum(strings.Join(sig, "|"))
	if want != n.NotifySig {
		return false
	}
	return true
}

func NewCallBack(apikey, apiSecret string, do func(NotifyData) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		//	app_name：应用名称
		//	pm_id：支付方式id: 例如 alipay_cn. 详细pm_id请参考
		//	transaction_id： Payssion平台交易号，非商户订单号。
		//	order_id：商家订单号
		//	amount：订单金额
		//	paid: 已支付金额
		//	net: 扣除手续费后的净额
		//	currency：交易币种
		//	description：订单描述
		//	state：支付状态
		//	notify_sig: 异步通知签名，具体规则参考签名规则。
		var data NotifyData
		if err := c.ShouldBind(&data); err != nil {
			c.JSON(http.StatusBadRequest, nil)
			return
		}
		ok := data.Verify(apikey, apiSecret)
		if !ok {
			c.JSON(http.StatusUnauthorized, nil)
			return
		}
		err := do(data)
		if err != nil {
			c.JSON(http.StatusInternalServerError, nil)
			return
		}
		c.JSON(http.StatusOK, nil)
	}
}
