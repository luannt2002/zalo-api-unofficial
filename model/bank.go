package model

type BankInfo struct {
	Bin           int    `json:"bin"`
	Logo          string `json:"logo"`
	Name          string `json:"name"`
	NameEng       string `json:"name_eng"`
	ShortName     string `json:"short_name"`
	SearchKeyWord string `json:"search_key_word"`
}

type BankAccount struct {
	ID         string `json:"id"`
	Bin        int    `json:"bin"`
	Default    bool   `json:"default"`
	BankNumber string `json:"bank_number"`
	BankLogo   string `json:"bank_logo"`
	HolderName string `json:"holder_name"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
	AccountID  int64  `json:"account_id"`
	BankName   string `json:"bank_name"`
	IsDefault  bool   `json:"is_default"`
}
