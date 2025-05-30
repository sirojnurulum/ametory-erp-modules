package models

type ProfitLoss struct {
	GeneralReport
	Profit       ProfitLossCategory `json:"profit"`
	Loss         ProfitLossCategory `json:"loss"`
	NetSurplus   ProfitLossCategory `json:"net_surplus"`
	Amount       float64            `json:"amount"`
	IsPeriodical bool
	CurrencyCode string `json:"currency_code" example:"currency_code"`
}

type ProfitLossCategory struct {
	Title             string           `json:"title"`
	ProfitLossAccount []BalanceAccount `json:"accounts"`
	SubTotal          float64          `json:"subtotal"`
	CurrencyCode      string           `json:"currency_code" example:"currency_code"`
}

type ProfitLossReport struct {
	GeneralReport
	Profit            []ProfitLossAccount `json:"profit"`
	Loss              []ProfitLossAccount `json:"loss"`
	NetSurplus        []ProfitLossAccount `json:"net_surplus"`
	Tax               []ProfitLossAccount `json:"tax"`
	GrossProfit       float64             `json:"gross_profit"`
	TotalNetSurplus   float64             `json:"total_net_surplus"`
	TotalExpense      float64             `json:"total_expense"`
	NetProfit         float64             `json:"net_profit"`
	IncomeTax         float64             `json:"income_tax"`
	NetProfitAfterTax float64             `json:"net_profit_after_tax"`
}

type ProfitLossAccount struct {
	ID     string  `json:"id"`
	Code   string  `json:"code"`
	Name   string  `json:"name"`
	Type   string  `json:"type"`
	Sum    float64 `json:"sum"`
	Link   string  `json:"link"`
	IsCogs bool    `json:"is_cogs"`
}
