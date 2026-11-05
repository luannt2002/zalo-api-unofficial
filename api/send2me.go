package api

// GetSend2meID trả về send2me_id (ID hội thoại Cloud của tôi).
func (a *api) GetSend2meID() string {
	return a.sc.Send2meID()
}

// IsSend2me kiểm tra xem một ID có phải là Cloud của tôi (send2me_id) hay không.
func (a *api) IsSend2me(id string) bool {
	return a.sc.IsSend2me(id)
}

