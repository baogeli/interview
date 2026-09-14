package main

import "fmt"

//todo 应有用场景 多种支付接入，统一接口实现

type MultiPay interface {
	Pay(amount int) (string, error)
	Refund(orderNumber string, amount int) (string, error)
	Query(orderNumber string) (string, error)
}

type AliPay struct {
	AppId      string
	AppSecret  string
	MerchantNo string
}

func (a AliPay) Pay(amount int) (string, error) {
	return "AliPaySuccess : amount === " + fmt.Sprintf("%d", amount), nil
}

func (a AliPay) Refund(orderNumber string, amount int) (string, error) {
	return "RefundSuccess", nil
}

func (a AliPay) Query(orderNumber string) (string, error) {
	return "QuerySuccess", nil
}

type WechatPay struct {
	AppId      string
	AppSecret  string
	MerchantNo string
}

func (w WechatPay) Pay(amount int) (string, error) {
	return "WechatPaySuccess", nil
}

func (w WechatPay) Refund(orderNumber string, amount int) (string, error) {
	return "RefundSuccess", nil
}

func (w WechatPay) Query(orderNumber string) (string, error) {
	return "QuerySuccess", nil
}

// ProcessPayment todo 统一支付
func ProcessPayment(pay MultiPay, amount int) (string, error) {
	fmt.Println("Processing payment...")
	result, err := pay.Pay(amount)
	if err != nil {
		return "", err
	}
	return result, nil
}

// ProcessRefund todo 统一退款
func ProcessRefund(pay MultiPay, orderNumber string, amount int) (string, error) {
	return pay.Refund(orderNumber, amount)
}

// ProcessQuery todo 统一查询
func ProcessQuery(pay MultiPay, orderNumber string) (string, error) {
	return pay.Query(orderNumber)
}

func main() {
	aliPay := AliPay{
		AppId:      "aliPayAppId",
		AppSecret:  "aliPayAppSecret",
		MerchantNo: "aliPayMerchantNo",
	}

	result, err := ProcessPayment(aliPay, 100)
	if err != nil {
		fmt.Println("Error:", err)
	}
	fmt.Println(result)

	fmt.Println("pay rule === ", aliPay.AppId)
}
